package backend

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"lianxiang-browser/backend/internal/browser"

	"github.com/google/uuid"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type ChromeUserDataImportResult struct {
	Cancelled    bool   `json:"cancelled"`
	ProfileID    string `json:"profileId"`
	ProfileName  string `json:"profileName"`
	SourceDir    string `json:"sourceDir"`
	CopiedFiles  int    `json:"copiedFiles"`
	SkippedFiles int    `json:"skippedFiles"`
	Message      string `json:"message"`
}

type chromeUserDataCopyStats struct {
	Copied  int
	Skipped int
}

var chromeImportVolatileDirs = map[string]struct{}{
	"browsermetrics":         {},
	"cache":                  {},
	"code cache":             {},
	"component_crx_cache":    {},
	"crashpad":               {},
	"deferredbrowsermetrics": {},
	"gpucache":               {},
	"graphitedawncache":      {},
	"grshadercache":          {},
	"shadercache":            {},
}

var chromeImportVolatileFiles = map[string]struct{}{
	"devtoolsactiveport": {},
	"lock":               {},
	"lockfile":           {},
}

// BrowserChromeUserDataImport 选择 Chrome 的 --user-data-dir 目录并复制为独立实例。
func (a *App) BrowserChromeUserDataImport(profileName string) (ChromeUserDataImportResult, error) {
	a.maintenanceMu.Lock()
	defer a.maintenanceMu.Unlock()

	if a.ctx == nil {
		return ChromeUserDataImportResult{}, fmt.Errorf("应用上下文未初始化")
	}
	selectedDir, err := wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "选择 Chrome 的用户数据目录（--user-data-dir）",
	})
	if err != nil {
		return ChromeUserDataImportResult{}, fmt.Errorf("打开目录选择器失败: %w", err)
	}
	if strings.TrimSpace(selectedDir) == "" {
		return ChromeUserDataImportResult{Cancelled: true, Message: "已取消导入"}, nil
	}
	return a.importChromeUserDataFromPath(selectedDir, profileName)
}

func (a *App) importChromeUserDataFromPath(sourceDir, profileName string) (ChromeUserDataImportResult, error) {
	if a.browserMgr == nil || a.config == nil {
		return ChromeUserDataImportResult{}, fmt.Errorf("浏览器管理器未初始化")
	}

	sourceDir, err := validateChromeUserDataDir(sourceDir)
	if err != nil {
		return ChromeUserDataImportResult{}, err
	}
	processes, err := findBrowserUserDataProcesses(sourceDir)
	if err != nil {
		return ChromeUserDataImportResult{}, fmt.Errorf("检查 Chrome 目录占用状态失败: %w", err)
	}
	if len(processes) > 0 {
		return ChromeUserDataImportResult{}, fmt.Errorf("该目录仍被 Chrome 使用，请先关闭使用此目录的所有 Chrome 窗口后再导入")
	}

	profileID := uuid.NewString()
	profileName = strings.TrimSpace(profileName)
	if profileName == "" {
		profileName = buildChromeImportProfileName(sourceDir)
	}
	profile := &browser.Profile{
		ProfileId:   profileID,
		ProfileName: profileName,
		UserDataDir: "chrome-import-" + profileID,
		ProxyId:     "__direct__",
		ProxyConfig: "direct://",
		Tags:        []string{"Chrome 导入"},
		CreatedAt:   time.Now().Format(time.RFC3339),
		UpdatedAt:   time.Now().Format(time.RFC3339),
	}
	if _, _, err := a.browserMgr.ApplyDefaults(profile); err != nil {
		return ChromeUserDataImportResult{}, fmt.Errorf("初始化导入实例失败: %w", err)
	}

	targetDir := a.browserMgr.ResolveUserDataDir(profile)
	stagingDir := targetDir + ".chrome-import-" + uuid.NewString()
	if err := ensureSeparateChromeImportPaths(sourceDir, targetDir); err != nil {
		return ChromeUserDataImportResult{}, err
	}
	stats, err := copyChromeUserDataDir(sourceDir, stagingDir)
	if err != nil {
		_ = os.RemoveAll(stagingDir)
		return ChromeUserDataImportResult{}, fmt.Errorf("复制 Chrome 用户数据失败: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(targetDir), 0o755); err != nil {
		_ = os.RemoveAll(stagingDir)
		return ChromeUserDataImportResult{}, fmt.Errorf("创建实例数据目录失败: %w", err)
	}
	if _, err := os.Stat(targetDir); err == nil {
		_ = os.RemoveAll(stagingDir)
		return ChromeUserDataImportResult{}, fmt.Errorf("目标实例数据目录已存在，请重试")
	} else if !os.IsNotExist(err) {
		_ = os.RemoveAll(stagingDir)
		return ChromeUserDataImportResult{}, fmt.Errorf("检查实例数据目录失败: %w", err)
	}
	if err := os.Rename(stagingDir, targetDir); err != nil {
		_ = os.RemoveAll(stagingDir)
		return ChromeUserDataImportResult{}, fmt.Errorf("保存实例数据失败: %w", err)
	}

	a.browserMgr.InitData()
	a.browserMgr.Mutex.Lock()
	a.browserMgr.Profiles[profile.ProfileId] = profile
	if err := a.browserMgr.SaveProfiles(); err != nil {
		delete(a.browserMgr.Profiles, profile.ProfileId)
		_ = a.browserMgr.SaveProfiles()
		a.browserMgr.Mutex.Unlock()
		_ = os.RemoveAll(targetDir)
		return ChromeUserDataImportResult{}, fmt.Errorf("保存导入实例失败: %w", err)
	}
	a.browserMgr.Mutex.Unlock()
	if a.launchCodeSvc != nil {
		if code, codeErr := a.launchCodeSvc.EnsureCode(profile.ProfileId); codeErr == nil {
			a.browserMgr.Mutex.Lock()
			profile.LaunchCode = code
			a.browserMgr.Mutex.Unlock()
		}
	}

	return ChromeUserDataImportResult{
		ProfileID:    profile.ProfileId,
		ProfileName:  profile.ProfileName,
		SourceDir:    sourceDir,
		CopiedFiles:  stats.Copied,
		SkippedFiles: stats.Skipped,
		Message:      "Chrome 用户数据已导入",
	}, nil
}

func validateChromeUserDataDir(sourceDir string) (string, error) {
	sourceDir = strings.TrimSpace(sourceDir)
	if sourceDir == "" {
		return "", fmt.Errorf("请选择 Chrome 用户数据目录")
	}
	absDir, err := filepath.Abs(sourceDir)
	if err != nil {
		return "", fmt.Errorf("解析 Chrome 用户数据目录失败: %w", err)
	}
	absDir = filepath.Clean(absDir)
	info, err := os.Stat(absDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("Chrome 用户数据目录不存在")
		}
		return "", fmt.Errorf("读取 Chrome 用户数据目录失败: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("请选择 Chrome 的用户数据文件夹，而不是文件")
	}
	if info, err := os.Stat(filepath.Join(absDir, "Local State")); err != nil || info.IsDir() {
		return "", fmt.Errorf("所选目录缺少 Local State，不是有效的 Chrome --user-data-dir 目录")
	}
	entries, err := os.ReadDir(absDir)
	if err != nil {
		return "", fmt.Errorf("读取 Chrome 用户数据目录失败: %w", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() || !isChromeProfileDirName(entry.Name()) {
			continue
		}
		profileDir := filepath.Join(absDir, entry.Name())
		for _, marker := range []string{"Preferences", "History", "Bookmarks"} {
			if markerInfo, markerErr := os.Stat(filepath.Join(profileDir, marker)); markerErr == nil && !markerInfo.IsDir() {
				return absDir, nil
			}
		}
	}
	return "", fmt.Errorf("所选目录没有可导入的 Chrome 配置（Default 或 Profile *）")
}

func isChromeProfileDirName(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	return name == "default" || name == "guest profile" || name == "system profile" || strings.HasPrefix(name, "profile ")
}

func buildChromeImportProfileName(sourceDir string) string {
	name := strings.TrimSpace(filepath.Base(sourceDir))
	if name == "" || name == "." || name == string(filepath.Separator) {
		return "Chrome 导入实例"
	}
	return "Chrome-" + name
}

func ensureSeparateChromeImportPaths(sourceDir, targetDir string) error {
	source, err := canonicalChromeImportPath(sourceDir)
	if err != nil {
		return err
	}
	target, err := canonicalChromeImportPath(targetDir)
	if err != nil {
		return err
	}
	source = strings.ToLower(source)
	target = strings.ToLower(target)
	separator := string(os.PathSeparator)
	if source == target || strings.HasPrefix(target, strings.TrimRight(source, separator)+separator) || strings.HasPrefix(source, strings.TrimRight(target, separator)+separator) {
		return fmt.Errorf("Chrome 源目录与链享实例数据目录不能互相包含")
	}
	return nil
}

// Resolve existing ancestors as well, since the destination does not exist yet.
func canonicalChromeImportPath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err == nil {
		return filepath.Clean(resolved), nil
	}
	if !os.IsNotExist(err) {
		return "", err
	}
	parent := filepath.Dir(abs)
	if parent == abs {
		return "", err
	}
	resolvedParent, err := canonicalChromeImportPath(parent)
	if err != nil {
		return "", err
	}
	return filepath.Join(resolvedParent, filepath.Base(abs)), nil
}

func copyChromeUserDataDir(sourceDir, targetDir string) (chromeUserDataCopyStats, error) {
	stats := chromeUserDataCopyStats{}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return stats, err
	}
	err := filepath.WalkDir(sourceDir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == sourceDir {
			return nil
		}
		rel, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 || shouldSkipChromeImportPath(rel, entry.IsDir()) {
			stats.Skipped++
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		target := filepath.Join(targetDir, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := backupCopyFile(path, target); err != nil {
			return err
		}
		stats.Copied++
		return nil
	})
	return stats, err
}

func shouldSkipChromeImportPath(rel string, isDir bool) bool {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(rel)))
	if strings.HasPrefix(base, "singleton") || strings.HasSuffix(base, ".tmp") {
		return true
	}
	if _, ok := chromeImportVolatileFiles[base]; ok {
		return true
	}
	if isDir {
		_, ok := chromeImportVolatileDirs[base]
		return ok
	}
	return false
}
