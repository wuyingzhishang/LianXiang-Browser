# Windows 发布目录

Windows 发布由 `bat\publish.bat` 统一驱动，版本号读取 `wails.json` 的 `info.productVersion`。发布脚本每次都会从源码重新构建本地 EXE，清理旧的 Windows 输出，生成临时 staging，并在结束时删除 staging。

## 标准发布

在仓库根目录执行：

```powershell
.\bat\publish.bat W -Version 1.4.6
```

未显式传入 `-WindowsFormat` 时，Windows 目标默认生成完整三件套：

```text
build\bin\lianxiang-browser.exe
publish\output\LianXiang-Browser-Setup-1.4.6.exe
publish\output\LianXiang-Browser-1.4.6-windows-amd64-portable.zip
publish\output\SHA256SUMS-1.4.6.txt
```

`SHA256SUMS-<version>.txt` 同时记录本地 EXE、安装包和便携 ZIP 的 SHA-256。清单中的路径相对于仓库根目录，便于在构建机和发布归档中复核。

也可以只生成其中一种 Windows 包：

```powershell
.\bat\publish.bat W -Version 1.4.6 -WindowsFormat INSTALLER
.\bat\publish.bat W -Version 1.4.6 -WindowsFormat PORTABLE
```

只生成一种包时，SHA-256 清单仍会包含本次构建的本地 EXE 和所选包。

## 清理边界

脚本只会清理以下可再生文件：

- `publish\output\LianXiang-Browser-Setup-*.exe`
- `publish\output\LianXiang-Browser-*-windows-amd64-portable.zip`
- `publish\output\SHA256SUMS-*.txt`
- `publish\staging\`（始终为临时目录）
- 构建前的 `build\bin\lianxiang-browser.exe`

不会自动删除 `data\`、`chrome\`、`bin\`、用户配置、数据库或代理运行时。发布包中的 `data\` 为空目录，首次启动时由应用初始化。

## 依赖

- Windows 10/11 amd64
- Go 1.25+
- Node.js 22 LTS 与 npm
- Wails CLI 2.12.0
- NSIS（仅生成安装包时需要；脚本支持 `MAKENSIS_PATH`、`NSIS_PATH`、`NSIS_HOME`，也会查找 `.local-tools\nsis`）
- Microsoft Edge WebView2 Runtime（运行应用时需要）

发布前会校验 `publish\runtime-manifest.json` 中的 Xray 和 sing-box 哈希。缺少运行时或哈希不匹配会直接失败，不会复用旧包。

## 验收

```powershell
Get-Content .\publish\output\SHA256SUMS-1.4.6.txt
Get-FileHash .\build\bin\lianxiang-browser.exe -Algorithm SHA256
Get-FileHash .\publish\output\LianXiang-Browser-Setup-1.4.6.exe -Algorithm SHA256
Get-FileHash .\publish\output\LianXiang-Browser-1.4.6-windows-amd64-portable.zip -Algorithm SHA256
```

安装包和便携包都不包含开发机的 `data\app.db`、日志、账号、Cookie、Token、代理订阅或 `.env`。如果 `chrome\` 下存在有效的 Windows `chrome.exe`，发布脚本会按目录规则复制；否则只保留 `chrome\README.md` 占位说明。
