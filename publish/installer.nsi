; 链享浏览器 NSIS Installer Script
; Usage: makensis /DVERSION=1.9.4 /DSTAGINGDIR=C:\path\to\staging installer.nsi

Unicode True

!ifndef VERSION
  !define VERSION "1.9.4"
!endif

!ifndef OUTPUTDIR
  !define OUTPUTDIR "..\publish\output"
!endif
!ifndef FILEVERSION
  !define FILEVERSION "${VERSION}.0"
!endif
!ifndef STAGINGDIR
  !define STAGINGDIR "..\publish\staging"
!endif

!define PRODUCT_NAME    "链享浏览器"
!define PRODUCT_EXE     "lianxiang-browser.exe"
!define PRODUCT_ICON    "LianXiangBrowser.ico"
!define UNINSTALL_KEY   "Software\Microsoft\Windows\CurrentVersion\Uninstall\LianXiangBrowser"
!define INSTALL_DIR     "$PROGRAMFILES64\链享浏览器"
!define INSTALL_DIR_LEGACY "$PROGRAMFILES64\链享浏览器"
!define INSTALL_DIR_USER "$LOCALAPPDATA\Programs\链享浏览器"
!define ELEVATE_FLAG_INSTALL "$TEMP\lianxiang-browser-install-elevation.flag"
!define ELEVATE_FLAG_UNINSTALL "$TEMP\lianxiang-browser-uninstall-elevation.flag"
!define POWERSHELL_EXE  "$SYSDIR\WindowsPowerShell\v1.0\powershell.exe"

!include "MUI2.nsh"
!include "LogicLib.nsh"
!include "FileFunc.nsh"

!macro WriteCloseProcessScript HANDLE
  FileWrite ${HANDLE} "param([string]$$InstallDir, [string]$$ExcludePath, [switch]$$Force)$\r$\n"
  FileWrite ${HANDLE} "$$ErrorActionPreference = 'SilentlyContinue'$\r$\n"
  FileWrite ${HANDLE} "if ([string]::IsNullOrWhiteSpace($$InstallDir) -or -not (Test-Path -LiteralPath $$InstallDir)) { exit 0 }$\r$\n"
  FileWrite ${HANDLE} "$$root = [System.IO.Path]::GetFullPath($$InstallDir).TrimEnd('\') + '\'$\r$\n"
  FileWrite ${HANDLE} "$$exclude = ''$\r$\n"
  FileWrite ${HANDLE} "if (-not [string]::IsNullOrWhiteSpace($$ExcludePath)) { $$exclude = [System.IO.Path]::GetFullPath($$ExcludePath) }$\r$\n"
  FileWrite ${HANDLE} "function Get-LianXiangBrowserProcesses {$\r$\n"
  FileWrite ${HANDLE} "  @(Get-CimInstance Win32_Process | Where-Object {$\r$\n"
  FileWrite ${HANDLE} "    $$_.ExecutablePath -and $$_.ExecutablePath.StartsWith($$root, [System.StringComparison]::OrdinalIgnoreCase) -and ($$exclude -eq '' -or -not $$_.ExecutablePath.Equals($$exclude, [System.StringComparison]::OrdinalIgnoreCase))$\r$\n"
  FileWrite ${HANDLE} "  })$\r$\n"
  FileWrite ${HANDLE} "}$\r$\n"
  FileWrite ${HANDLE} "$$procs = @(Get-LianXiangBrowserProcesses)$\r$\n"
  FileWrite ${HANDLE} "if (-not $$procs -or $$procs.Count -eq 0) { exit 0 }$\r$\n"
  FileWrite ${HANDLE} "$$coreProcs = @($$procs | Where-Object { $$coreNames -contains $$_.Name.ToLowerInvariant() })$\r$\n"
  FileWrite ${HANDLE} "$$killed = @()$\r$\n"
  FileWrite ${HANDLE} "foreach ($$proc in $$coreProcs) {$\r$\n"
  FileWrite ${HANDLE} "  Stop-Process -Id $$proc.ProcessId -Force -ErrorAction SilentlyContinue$\r$\n"
  FileWrite ${HANDLE} "  if ($$?) { $$killed += ($$proc.Name + '#' + $$proc.ProcessId) }$\r$\n"
  FileWrite ${HANDLE} "}$\r$\n"
  FileWrite ${HANDLE} "if ($$killed.Count -gt 0) {$\r$\n"
  FileWrite ${HANDLE} "  Write-Host ('killed leftover cores: ' + ($$killed -join ', '))$\r$\n"
  FileWrite ${HANDLE} "  Start-Sleep -Milliseconds 600$\r$\n"
  FileWrite ${HANDLE} "}$\r$\n"
  FileWrite ${HANDLE} "$$left = @(Get-AntBrowserProcesses)$\r$\n"
  FileWrite ${HANDLE} "if (-not $$left -or $$left.Count -eq 0) { exit 0 }$\r$\n"
  FileWrite ${HANDLE} "$$names = ($$left | ForEach-Object { $$_.Name + '#' + $$_.ProcessId }) -join ', '$\r$\n"
  FileWrite ${HANDLE} "Write-Host ('running: ' + $$names)$\r$\n"
  FileWrite ${HANDLE} "if ($$Force) {$\r$\n"
  FileWrite ${HANDLE} "  foreach ($$p in $$procs) { try { Stop-Process -Id $$p.ProcessId -Force -ErrorAction Stop } catch {} }$\r$\n"
  FileWrite ${HANDLE} "  Start-Sleep -Milliseconds 500$\r$\n"
  FileWrite ${HANDLE} "  $$left = @(Get-LianXiangBrowserProcesses)$\r$\n"
  FileWrite ${HANDLE} "  if (-not $$left -or $$left.Count -eq 0) { exit 0 }$\r$\n"
  FileWrite ${HANDLE} "}$\r$\n"
  FileWrite ${HANDLE} "exit 1$\r$\n"
!macroend

Function CloseInstalledProcesses
  IfFileExists "$INSTDIR" 0 done

retry_powershell:
  IfFileExists "${POWERSHELL_EXE}" 0 fallback_taskkill

  GetTempFileName $0
  Delete $0
  StrCpy $0 "$0.ps1"
  FileOpen $1 $0 w
  !insertmacro WriteCloseProcessScript $1
  FileClose $1

  DetailPrint "检查安装目录中的运行进程: $INSTDIR"
  ExecWait '"${POWERSHELL_EXE}" -NoProfile -ExecutionPolicy Bypass -File "$0" -InstallDir "$INSTDIR" -ExcludePath ""' $2

  ${If} $2 == 0
    Goto done
  ${EndIf}

  MessageBox MB_YESNO|MB_ICONEXCLAMATION "检测到安装目录中仍有运行进程。$\r$\n$\r$\n目录：$INSTDIR$\r$\n$\r$\n点击“是”结束这些进程并继续安装；点击“否”先从窗口或托盘菜单正常退出应用后重新检查。" IDYES force_powershell IDNO retry_powershell

force_powershell:
  ExecWait '"${POWERSHELL_EXE}" -NoProfile -ExecutionPolicy Bypass -File "$0" -InstallDir "$INSTDIR" -ExcludePath "" -Force' $2
  ${If} $2 == 0
    Goto done
  ${EndIf}
  MessageBox MB_RETRYCANCEL|MB_ICONSTOP "仍有进程无法结束。请重启 Windows 或手动关闭相关进程后重试。" IDRETRY retry_powershell IDCANCEL install_abort

install_abort:
  Abort "安装已取消：安装目录中的旧进程仍未退出。"

fallback_taskkill:
  MessageBox MB_OK|MB_ICONSTOP "无法安全检查安装目录中的运行进程。为避免强制中断，安装已取消。"
  Abort "安装已取消：无法安全检查安装目录中的运行进程。"

done:
  Delete $0
FunctionEnd

Function un.CloseInstalledProcesses
  IfFileExists "$INSTDIR" 0 done

retry_powershell:
  IfFileExists "${POWERSHELL_EXE}" 0 fallback_taskkill

  GetTempFileName $0
  Delete $0
  StrCpy $0 "$0.ps1"
  FileOpen $1 $0 w
  !insertmacro WriteCloseProcessScript $1
  FileClose $1

  DetailPrint "检查安装目录中的运行进程: $INSTDIR"
  ExecWait '"${POWERSHELL_EXE}" -NoProfile -ExecutionPolicy Bypass -File "$0" -InstallDir "$INSTDIR" -ExcludePath "$INSTDIR\Uninstall.exe"' $2

  ${If} $2 == 0
    Goto done
  ${EndIf}

  MessageBox MB_YESNO|MB_ICONEXCLAMATION "检测到安装目录中仍有运行进程。$\r$\n$\r$\n目录：$INSTDIR$\r$\n$\r$\n点击“是”结束这些进程并继续卸载；点击“否”先从窗口或托盘菜单正常退出应用后重新检查。" IDYES force_powershell IDNO retry_powershell

force_powershell:
  ExecWait '"${POWERSHELL_EXE}" -NoProfile -ExecutionPolicy Bypass -File "$0" -InstallDir "$INSTDIR" -ExcludePath "$INSTDIR\Uninstall.exe" -Force' $2
  ${If} $2 == 0
    Goto done
  ${EndIf}
  MessageBox MB_RETRYCANCEL|MB_ICONSTOP "仍有进程无法结束。请重启 Windows 或手动关闭相关进程后重试。" IDRETRY retry_powershell IDCANCEL uninstall_abort

uninstall_abort:
  Abort "卸载已取消：安装目录中的旧进程仍未退出。"

fallback_taskkill:
  MessageBox MB_OK|MB_ICONSTOP "无法安全检查安装目录中的运行进程。为避免强制中断，卸载已取消。"
  Abort "卸载已取消：无法安全检查安装目录中的运行进程。"

done:
  Delete $0
FunctionEnd

Function RegisterDiagnosticsWatcher
  IfFileExists "$INSTDIR\data\diagnostics\install-lianxiang-browser-diagnostics-watcher.ps1" 0 done
  DetailPrint "Registering read-only diagnostics watcher"
  ExecWait '"${POWERSHELL_EXE}" -NoProfile -ExecutionPolicy Bypass -File "$INSTDIR\data\diagnostics\install-lianxiang-browser-diagnostics-watcher.ps1" -InstallDir "$INSTDIR"' $0
  ${If} $0 == 0
    DetailPrint "Diagnostics watcher registered"
  ${Else}
    DetailPrint "Warning: diagnostics watcher registration failed"
  ${EndIf}
done:
FunctionEnd

Function un.UnregisterDiagnosticsWatcher
  IfFileExists "$INSTDIR\data\diagnostics\install-lianxiang-browser-diagnostics-watcher.ps1" 0 done
  DetailPrint "Removing read-only diagnostics watcher"
  ExecWait '"${POWERSHELL_EXE}" -NoProfile -ExecutionPolicy Bypass -File "$INSTDIR\data\diagnostics\install-lianxiang-browser-diagnostics-watcher.ps1" -InstallDir "$INSTDIR" -Uninstall' $0
done:
FunctionEnd

Function PreserveLegacyBackupLocalConfig
  StrCpy $4 "$INSTDIR"
  ReadRegStr $3 HKLM "${UNINSTALL_KEY}" "InstallLocation"
  ${If} "$3" != ""
    IfFileExists "$3\backup.local.yaml" 0 legacy_use_current_install
    StrCpy $4 "$3"
  ${EndIf}

legacy_use_current_install:
  IfFileExists "$APPDATA\${PRODUCT_NAME}\backup.local.yaml" legacy_done
  IfFileExists "$4\backup.local.yaml" 0 legacy_done

  ClearErrors
  CreateDirectory "$APPDATA\${PRODUCT_NAME}"
  IfErrors legacy_failed
  DetailPrint "Preserving legacy backup credentials"
  ClearErrors
  CopyFiles /SILENT "$4\backup.local.yaml" "$APPDATA\${PRODUCT_NAME}"
  IfErrors legacy_failed
  Goto legacy_done

legacy_failed:
  MessageBox MB_ICONSTOP|MB_OK "Could not preserve the legacy backup credentials. Installation is cancelled to prevent OpenList / S3 Token loss."
  Abort "Installation cancelled: legacy backup credential migration failed."

legacy_done:
FunctionEnd

Function un.PreserveLegacyBackupLocalConfig
  IfFileExists "$INSTDIR\backup.local.yaml" 0 done
  IfFileExists "$APPDATA\${PRODUCT_NAME}\backup.local.yaml" done

  ClearErrors
  CreateDirectory "$APPDATA\${PRODUCT_NAME}"
  IfErrors preserve_failed

  DetailPrint "迁移旧版本地备份凭据到 $APPDATA\${PRODUCT_NAME}"
  ClearErrors
  CopyFiles "$INSTDIR\backup.local.yaml" "$APPDATA\${PRODUCT_NAME}\"
  IfErrors preserve_failed
  Goto done

preserve_failed:
  MessageBox MB_ICONSTOP|MB_OK "无法保存旧版本地备份凭据：$APPDATA\${PRODUCT_NAME}\backup.local.yaml。为避免 OpenList / S3 Token 丢失，卸载已取消。请检查该目录权限后重试。"
  Abort "卸载已取消：旧版本地备份凭据迁移失败。"

done:
FunctionEnd

Var InstallScope
Var ElevationAttempted
Var NeedElevation

; .onInit：老用户沿用原安装路径（机器级，需要管理员）；新用户装到当前用户目录（免 UAC）。
Function .onInit
  StrCpy $InstallScope "user"
  StrCpy $ElevationAttempted ""
  ${If} ${FileExists} "${ELEVATE_FLAG_INSTALL}"
    StrCpy $ElevationAttempted "1"
  ${EndIf}

  ; 1) 机器级旧安装：HKLM 注册记录，或注册表丢失但 Program Files 下确有真实数据
  ReadRegStr $0 HKLM "${UNINSTALL_KEY}" "InstallLocation"
  ${If} $0 == ""
    ${If} ${FileExists} "${INSTALL_DIR_LEGACY}\data\app.db"
      StrCpy $0 "${INSTALL_DIR_LEGACY}"
    ${EndIf}
  ${EndIf}

  ${If} $0 != ""
    StrCpy $InstallScope "machine"
    StrCpy $INSTDIR "$0"
    Call EnsureElevatedForInstall
    Return
  ${EndIf}

  ; 2) 用户级旧安装（本方案升级过）：沿用其路径
  ReadRegStr $1 HKCU "${UNINSTALL_KEY}" "InstallLocation"
  ${If} $1 != ""
    StrCpy $INSTDIR "$1"
  ${Else}
    StrCpy $INSTDIR "${INSTALL_DIR_USER}"
  ${EndIf}
FunctionEnd

; 机器级安装/升级必须提权：已提权直接返回；用户取消 UAC 则中止并提示，不会改动任何文件。
Function EnsureElevatedForInstall
  ClearErrors
  WriteRegStr HKLM "Software\AntBrowserElevationProbe" "probe" "1"
  IfErrors elevationRequired
  DeleteRegKey HKLM "Software\AntBrowserElevationProbe"
  Return

elevationRequired:
  ${If} $ElevationAttempted == "1"
    ; 上一轮提权重启仍未拿到管理员令牌（用户取消了 UAC 或选了标准账户）。
    ; 清掉标记，让下次运行可以重新尝试，本次明确中止。
    Delete "${ELEVATE_FLAG_INSTALL}"
    MessageBox MB_ICONSTOP|MB_OK "升级旧版本安装需要管理员权限。$\r$\n$\r$\n请右键安装程序选择“以管理员身份运行”，或在 UAC 提示中选择管理员账户。"
    Quit
  ${EndIf}

  FileOpen $2 "${ELEVATE_FLAG_INSTALL}" w
  FileClose $2
  ExecShell "runas" "$EXEPATH"
  Quit
FunctionEnd

; 仅在受保护或不可写目录时警告：默认的用户目录不触发，避免打扰新用户。
Function WarnInstallDir
  StrCpy $0 "$INSTDIR"

  StrLen $1 "$PROGRAMFILES64"
  ${If} $1 > 0
    StrCpy $2 "$0" $1
    ${If} "$2" == "$PROGRAMFILES64"
      Goto warn_dir
    ${EndIf}
  ${EndIf}

  StrLen $1 "$PROGRAMFILES"
  ${If} $1 > 0
    StrCpy $2 "$0" $1
    ${If} "$2" == "$PROGRAMFILES"
      Goto warn_dir
    ${EndIf}
  ${EndIf}

  StrLen $1 "$PROGRAMFILES32"
  ${If} $1 > 0
    StrCpy $2 "$0" $1
    ${If} "$2" == "$PROGRAMFILES32"
      Goto warn_dir
    ${EndIf}
  ${EndIf}

  ; 目标父目录不可写（普通权限建不了目录）同样警告。
  ; 先创建父目录：$LOCALAPPDATA\Programs 在个别系统上不存在，直接探测会误报。
  ${GetParent} "$0" $3
  CreateDirectory "$3"
  ClearErrors
  FileOpen $4 "$3\.ant-browser-write-probe" w
  ${If} ${Errors}
    Goto warn_dir
  ${EndIf}
  FileClose $4
  Delete "$3\.ant-browser-write-probe"

  Return

warn_dir:
  MessageBox MB_ICONINFORMATION|MB_OKCANCEL|MB_DEFBUTTON1 "当前目录位于 Windows 受保护路径。$\r$\n$\r$\n链享浏览器会自动将运行数据和单实例状态保存到当前用户可写目录；已有安装路径和用户数据无需迁移。$\r$\n$\r$\n点击“确定”继续安装，点击“取消”返回修改安装路径。" IDOK continue_install IDCANCEL cancel_install

continue_install:
  Return

cancel_install:
  Abort
FunctionEnd

; The installer runs elevated. Launching the app directly from the finish page
; would give it an administrator token: the protected install directory then
; looks writable, gets picked as the data directory, and the next normal launch
; falls back to the default directory. Chrome also drops elevation on its own,
; so instances started from that session cannot write their profile directory.
; Starting through the shell gives the app the same token as the shortcut.
Function LaunchAsCurrentUser
  Exec '"$WINDIR\explorer.exe" "$INSTDIR\${PRODUCT_EXE}"'
FunctionEnd

Name "${PRODUCT_NAME} ${VERSION}"
OutFile "${OUTPUTDIR}\LianXiangBrowser-Setup-${VERSION}.exe"
VIProductVersion "${FILEVERSION}"
VIAddVersionKey /LANG=2052 "ProductName" "链享浏览器"
VIAddVersionKey /LANG=2052 "ProductVersion" "${VERSION}"
VIAddVersionKey /LANG=2052 "FileVersion" "${FILEVERSION}"
VIAddVersionKey /LANG=2052 "FileDescription" "链享浏览器安装程序"
VIAddVersionKey /LANG=2052 "LegalCopyright" "Copyright © 2026 链享浏览器团队"
InstallDir "${INSTALL_DIR}"
InstallDirRegKey HKLM "${UNINSTALL_KEY}" "InstallLocation"
RequestExecutionLevel admin
!ifdef BESTCOMPRESSION
  SetCompressor /SOLID lzma
!else
  SetCompressor lzma
!endif

!define MUI_ICON "..\build\windows\icon.ico"
!define MUI_UNICON "..\build\windows\icon.ico"

!insertmacro MUI_PAGE_WELCOME
!define MUI_DIRECTORYPAGE_TEXT_TOP "请选择 链享浏览器 的安装目录。可以继续使用已有安装目录；安装到 Program Files 等受保护目录时，运行数据和单实例状态会保存到用户可写状态目录。"
!define MUI_DIRECTORYPAGE_TEXT_DESTINATION "安装目录"
!define MUI_PAGE_CUSTOMFUNCTION_LEAVE WarnInstallDir
!insertmacro MUI_PAGE_DIRECTORY
!define MUI_COMPONENTSPAGE_SMALLDESC
!insertmacro MUI_PAGE_COMPONENTS
!insertmacro MUI_PAGE_INSTFILES
!define MUI_FINISHPAGE_RUN
!define MUI_FINISHPAGE_RUN_TEXT "Launch 链享浏览器"
!define MUI_FINISHPAGE_RUN_FUNCTION LaunchAsCurrentUser
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES

!insertmacro MUI_LANGUAGE "SimpChinese"

Section "链享浏览器 (required)" SecMain
  SectionIn RO
  ; 机器级老用户保持“所有用户”的开始菜单与注册表；新用户写当前用户，不需要管理员。
  ${If} $InstallScope == "machine"
    SetShellVarContext all
  ${Else}
    SetShellVarContext current
  ${EndIf}
  Call CloseInstalledProcesses
  Call PreserveLegacyBackupLocalConfig
  SetOutPath "$INSTDIR"
  File "${STAGINGDIR}\${PRODUCT_EXE}"
  File /oname=${PRODUCT_ICON} "..\build\windows\icon.ico"
!if /FileExists "${STAGINGDIR}\config.yaml"
  IfFileExists "$INSTDIR\config.yaml" +2 0
    File "${STAGINGDIR}\config.yaml"
!else
  !echo "Warning: ${STAGINGDIR}\\config.yaml not found, installer will use runtime defaults."
!endif
!if /FileExists "${STAGINGDIR}\chrome\*"
  SetOutPath "$INSTDIR\chrome"
  File /r "${STAGINGDIR}\chrome\*"
  SetOutPath "$INSTDIR"
!endif
  CreateDirectory "$INSTDIR\data"
  SetOutPath "$INSTDIR\data\diagnostics"
  File /r "${STAGINGDIR}\data\diagnostics\*"
  SetOutPath "$INSTDIR"
  WriteRegStr SHCTX "${UNINSTALL_KEY}" "DisplayName"     "${PRODUCT_NAME}"
  WriteRegStr SHCTX "${UNINSTALL_KEY}" "DisplayVersion"  "${VERSION}"
  WriteRegStr SHCTX "${UNINSTALL_KEY}" "Publisher"       "链享浏览器团队"
  WriteRegStr SHCTX "${UNINSTALL_KEY}" "InstallLocation" "$INSTDIR"
  WriteRegStr SHCTX "${UNINSTALL_KEY}" "UninstallString" "$INSTDIR\Uninstall.exe"
  WriteRegStr SHCTX "${UNINSTALL_KEY}" "DisplayIcon"     "$INSTDIR\${PRODUCT_ICON}"
  WriteRegStr SHCTX "${UNINSTALL_KEY}" "NoModify"        "1"
  WriteRegStr SHCTX "${UNINSTALL_KEY}" "NoRepair"        "1"
  WriteUninstaller "$INSTDIR\Uninstall.exe"
  ; 诊断看护注册的是系统级计划任务，需要管理员；用户级安装跳过，不影响主功能。
  ${If} $InstallScope == "machine"
    Call RegisterDiagnosticsWatcher
  ${Else}
    DetailPrint "用户级安装：跳过系统级诊断看护注册（需要管理员权限）"
  ${EndIf}
  CreateDirectory "$SMPROGRAMS\${PRODUCT_NAME}"
  CreateShortcut "$SMPROGRAMS\${PRODUCT_NAME}\${PRODUCT_NAME}.lnk" "$INSTDIR\${PRODUCT_EXE}" "" "$INSTDIR\${PRODUCT_ICON}"
  CreateShortcut "$SMPROGRAMS\${PRODUCT_NAME}\Uninstall.lnk" "$INSTDIR\Uninstall.exe" "" "$INSTDIR\${PRODUCT_ICON}"
  Delete "${ELEVATE_FLAG_INSTALL}"
SectionEnd

Section "Proxy Runtime (xray / sing-box)" SecRuntime
  SectionIn RO
  SetOutPath "$INSTDIR\bin"
  File "${STAGINGDIR}\bin\xray.exe"
  File "${STAGINGDIR}\bin\sing-box.exe"
SectionEnd

Section /o "Desktop Shortcut" SecDesktop
  CreateShortcut "$DESKTOP\${PRODUCT_NAME}.lnk" "$INSTDIR\${PRODUCT_EXE}" "" "$INSTDIR\${PRODUCT_ICON}"
SectionEnd

!insertmacro MUI_FUNCTION_DESCRIPTION_BEGIN
  !insertmacro MUI_DESCRIPTION_TEXT ${SecMain}    "链享浏览器主程序和默认配置（必需）"
  !insertmacro MUI_DESCRIPTION_TEXT ${SecRuntime} "xray and sing-box proxy tools (vless/vmess/hysteria2)"
  !insertmacro MUI_DESCRIPTION_TEXT ${SecDesktop} "Create a shortcut on the desktop"
!insertmacro MUI_FUNCTION_DESCRIPTION_END

Section "Uninstall"
  ; 两种卸载需要管理员：文件在受保护目录（写探测失败），或旧版把卸载信息写在 HKLM。
  StrCpy $NeedElevation ""
  ReadRegStr $2 HKLM "${UNINSTALL_KEY}" "InstallLocation"
  ${If} $2 != ""
    StrCpy $NeedElevation "1"
  ${EndIf}
  ClearErrors
  FileOpen $0 "$INSTDIR\.uninstall-probe" w
  ${If} ${Errors}
    StrCpy $NeedElevation "1"
  ${Else}
    FileClose $0
    Delete "$INSTDIR\.uninstall-probe"
  ${EndIf}

  ; RequestExecutionLevel already elevates the uninstaller. Do not relaunch an
  ; administrator session and mistake its elevation flag for a failed UAC prompt.
  UserInfo::GetAccountType
  Pop $0
  ${If} $0 == "Admin"
    StrCpy $NeedElevation ""
  ${EndIf}
  ${If} $NeedElevation == "1"
    ${If} ${FileExists} "${ELEVATE_FLAG_UNINSTALL}"
      Delete "${ELEVATE_FLAG_UNINSTALL}"
      MessageBox MB_ICONSTOP|MB_OK "卸载旧版本安装需要管理员权限。$\r$\n$\r$\n请右键 Uninstall.exe 选择“以管理员身份运行”，或在 UAC 提示中选择管理员账户。"
      Quit
    ${EndIf}
    FileOpen $1 "${ELEVATE_FLAG_UNINSTALL}" w
    FileClose $1
    ExecShell "runas" "$INSTDIR\Uninstall.exe"
    Quit
  ${EndIf}
  Delete "${ELEVATE_FLAG_UNINSTALL}"

  Call un.CloseInstalledProcesses
  Call un.UnregisterDiagnosticsWatcher
  Call un.PreserveLegacyBackupLocalConfig

  Delete /REBOOTOK "$INSTDIR\${PRODUCT_EXE}"
  Delete /REBOOTOK "$INSTDIR\${PRODUCT_ICON}"
  Delete /REBOOTOK "$INSTDIR\proxies.yaml"
  Delete /REBOOTOK "$INSTDIR\Uninstall.exe"
  RMDir /r /REBOOTOK "$INSTDIR\bin"
  RMDir /r /REBOOTOK "$INSTDIR\chrome"
  ; 快捷方式可能建在当前用户（新装）或所有用户（旧装），两边都清一遍。
  SetShellVarContext current
  Delete /REBOOTOK "$SMPROGRAMS\${PRODUCT_NAME}\${PRODUCT_NAME}.lnk"
  Delete /REBOOTOK "$SMPROGRAMS\${PRODUCT_NAME}\Uninstall.lnk"
  RMDir /REBOOTOK "$SMPROGRAMS\${PRODUCT_NAME}"
  Delete /REBOOTOK "$DESKTOP\${PRODUCT_NAME}.lnk"
  SetShellVarContext all
  Delete /REBOOTOK "$SMPROGRAMS\${PRODUCT_NAME}\${PRODUCT_NAME}.lnk"
  Delete /REBOOTOK "$SMPROGRAMS\${PRODUCT_NAME}\Uninstall.lnk"
  RMDir /REBOOTOK "$SMPROGRAMS\${PRODUCT_NAME}"
  Delete /REBOOTOK "$DESKTOP\${PRODUCT_NAME}.lnk"
  DeleteRegKey HKLM "${UNINSTALL_KEY}"
  DeleteRegKey HKCU "${UNINSTALL_KEY}"
  MessageBox MB_ICONQUESTION|MB_YESNO|MB_DEFBUTTON2 "是否彻底清理所有用户数据？$\r$\n$\r$\n选择“是”将删除 data 目录（含数据库/实例数据）以及安装目录残留文件。$\r$\n此操作不可恢复。" IDYES un_remove_all_data IDNO un_keep_user_data

un_remove_all_data:
  RMDir /r /REBOOTOK "$INSTDIR\data"
  RMDir /r /REBOOTOK "$INSTDIR\logs"
  RMDir /r /REBOOTOK "$INSTDIR"
  Goto un_done

un_keep_user_data:
  RMDir /REBOOTOK "$INSTDIR"
  Goto un_done

un_done:
  IfFileExists "$INSTDIR\." 0 +2
    MessageBox MB_ICONEXCLAMATION|MB_OK "检测到部分文件仍被占用，已标记为重启后自动删除。"
SectionEnd
