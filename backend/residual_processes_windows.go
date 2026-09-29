//go:build windows
// +build windows

package backend

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

func killResidualRuntimeProcesses(appRoot string) error {
	appRoot = strings.TrimSpace(appRoot)
	if appRoot == "" {
		return nil
	}
	if _, err := os.Stat(appRoot); err != nil {
		return nil
	}

	exePath, _ := os.Executable()
	// 启动 PowerShell + WMI 查询需要约 2 秒，而绝大多数启动时并没有遗留进程。
	// 先用 Toolhelp 快照（毫秒级）确认 bin\ 下确有进程，再交给原有脚本处理
	// “浏览器仍在运行则不清理”等判断。快照失败时保守地回退到脚本。
	if found, err := hasProcessUnderDir(filepath.Join(appRoot, "bin"), exePath); err == nil && !found {
		return nil
	}
	return killResidualRuntimeProcessesPowerShell(appRoot, exePath)
}

// hasProcessUnderDir 判断是否存在可执行文件位于 dir 之下的进程（排除 excludePath）。
// 无权查询路径的进程（如其他用户或更高权限的进程）会被跳过，与 WMI 查询的可见范围一致。
func hasProcessUnderDir(dir, excludePath string) (bool, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return false, err
	}
	prefix := strings.TrimRight(absDir, `\/`) + `\`
	exclude := ""
	if strings.TrimSpace(excludePath) != "" {
		if abs, err := filepath.Abs(excludePath); err == nil {
			exclude = abs
		}
	}

	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return false, err
	}
	defer windows.CloseHandle(snapshot)

	selfPID := uint32(os.Getpid())
	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	for err = windows.Process32First(snapshot, &entry); err == nil; err = windows.Process32Next(snapshot, &entry) {
		if entry.ProcessID == 0 || entry.ProcessID == selfPID {
			continue
		}
		imagePath, ok := processImagePath(entry.ProcessID)
		if !ok {
			continue
		}
		if exclude != "" && strings.EqualFold(imagePath, exclude) {
			continue
		}
		if len(imagePath) > len(prefix) && strings.EqualFold(imagePath[:len(prefix)], prefix) {
			return true, nil
		}
	}
	if err != windows.ERROR_NO_MORE_FILES {
		return false, err
	}
	return false, nil
}

func processImagePath(pid uint32) (string, bool) {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return "", false
	}
	defer windows.CloseHandle(handle)
	buf := make([]uint16, windows.MAX_LONG_PATH)
	size := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(handle, 0, &buf[0], &size); err != nil {
		return "", false
	}
	return windows.UTF16ToString(buf[:size]), true
}

func killResidualRuntimeProcessesPowerShell(appRoot, exePath string) error {
	psScript := `param([string]$Root, [string]$ExcludePath)
$ErrorActionPreference = 'SilentlyContinue'
if ([string]::IsNullOrWhiteSpace($Root) -or -not (Test-Path -LiteralPath $Root)) { exit 0 }
$root = [System.IO.Path]::GetFullPath($Root).TrimEnd('\') + '\'
$exclude = ''
if (-not [string]::IsNullOrWhiteSpace($ExcludePath)) {
  $exclude = [System.IO.Path]::GetFullPath($ExcludePath)
}
function Get-AntChromeResidualProcesses {
  @(
    Get-CimInstance Win32_Process | Where-Object {
      $_.ExecutablePath -and (
        $_.ExecutablePath.StartsWith(($root + 'bin\'), [System.StringComparison]::OrdinalIgnoreCase) -or
        $_.ExecutablePath.StartsWith(($root + 'chrome\'), [System.StringComparison]::OrdinalIgnoreCase)
      ) -and (
        $exclude -eq '' -or
        -not $_.ExecutablePath.Equals($exclude, [System.StringComparison]::OrdinalIgnoreCase)
      )
    }
  )
}
$browserProcesses = @(
  Get-CimInstance Win32_Process | Where-Object {
    $_.CommandLine -and
    (
      ($_.ExecutablePath -and $_.ExecutablePath.StartsWith(($root + 'chrome\'), [System.StringComparison]::OrdinalIgnoreCase)) -or
      ($_.CommandLine -match '(?i)--user-data-dir(?:=|\s)' -and $_.CommandLine -match [regex]::Escape($root))
    )
  }
)
$targets = @(Get-AntChromeResidualProcesses | Where-Object {
  $_.ExecutablePath.StartsWith(($root + 'bin\'), [System.StringComparison]::OrdinalIgnoreCase)
} | Sort-Object ProcessId -Descending)
if ($browserProcesses.Count -gt 0 -or $targets.Count -eq 0) { exit 0 }
foreach ($p in $targets) {
  try { Stop-Process -Id $p.ProcessId -Force -ErrorAction Stop } catch {}
}
Start-Sleep -Milliseconds 400
$left = @(Get-AntChromeResidualProcesses | Where-Object {
  $_.ExecutablePath.StartsWith(($root + 'bin\'), [System.StringComparison]::OrdinalIgnoreCase)
})
if ($left.Count -gt 0) {
  $names = ($left | ForEach-Object { $_.Name + '#' + $_.ProcessId }) -join ', '
  Write-Host ('still running: ' + $names)
  exit 1
}
exit 0
`

	tempFile, err := os.CreateTemp("", "ant-chrome-cleanup-*.ps1")
	if err != nil {
		return fmt.Errorf("创建清理脚本失败: %w", err)
	}
	scriptPath := tempFile.Name()
	if _, err := tempFile.WriteString(psScript); err != nil {
		tempFile.Close()
		_ = os.Remove(scriptPath)
		return fmt.Errorf("写入清理脚本失败: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		_ = os.Remove(scriptPath)
		return fmt.Errorf("关闭清理脚本失败: %w", err)
	}
	defer os.Remove(scriptPath)

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	powershellPath := `C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe`
	if _, err := os.Stat(powershellPath); err != nil {
		if fallbackPath, lookErr := exec.LookPath("powershell.exe"); lookErr == nil {
			powershellPath = fallbackPath
		} else {
			return fmt.Errorf("未找到 powershell.exe")
		}
	}

	cmd := exec.CommandContext(ctx, powershellPath, "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", filepath.Clean(scriptPath), "-Root", appRoot, "-ExcludePath", exePath)
	hideWindow(cmd)

	output, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("清理残留进程超时")
	}
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		return fmt.Errorf("清理残留进程失败: %s", message)
	}
	return nil
}
