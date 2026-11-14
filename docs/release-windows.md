# Windows 发布说明

当前版本为 **v1.9.4**。本次提供 Windows x64 本地程序、NSIS 安装包、便携 ZIP 和 SHA-256 校验清单。

## 构建

需要 Go、Node.js/npm、Wails CLI、WebView2 和 NSIS。脚本优先使用仓库 `.local-tools` 内的工具链；NSIS 也可通过 `MAKENSIS_PATH`、`NSIS_PATH` 或 `NSIS_HOME` 指定。

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File bat\test-publish.ps1
powershell -NoProfile -ExecutionPolicy Bypass -File bat\publish.ps1 `
  -Target WINDOWS -WindowsFormat BOTH -Version 1.9.4 `
  -OutputDirectory publish/output/v1.9.4
```

`OutputDirectory` 为 Windows 专用选项，必须位于仓库 `publish/output` 内。不指定时兼容原来的 `publish/output` 位置。推荐为每个版本使用独立子目录。

脚本不会递归删除既有输出目录。它为每次构建创建独立 staging，仅清理自己创建的目录；`SHA256SUMS.txt` 只包含本次生成的产物，不会把运行程序后生成的配置、数据库或旧版本文件计入校验。

## 产物

本次构建输出 `publish/output/v1.9.4/`，整理后的发行目录为 `release/v1.9.4/`：

```text
LianXiangBrowser-1.9.4-windows-amd64.exe
LianXiangBrowser-Setup-1.9.4.exe
LianXiangBrowser-1.9.4-windows-amd64-portable.zip
SHA256SUMS.txt
README.md
```

README 仅放在整理后的发行目录。二进制构建产物不提交到 Git；发行说明和校验清单提交到 Git。旧版发行文件按明确的版本清单清理，不覆盖旧用户数据。本次已清理旧版独立发布文件和旧测试 EXE；最终文件保留在 `release/v1.9.4`。构建输出副本和临时验证目录的删除被自动审批策略阻止，暂时保留。

安装包和便携包使用相同 staging 内容，包含主程序、默认配置、Xray、sing-box、诊断脚本。本次构建目录没有 Chrome 内核，因此只携带内核说明文件；用户首次使用时需要下载或导入内核。

单独 EXE 仅含应用本体，不含上述外部运行时。升级已有完整目录时可替换主程序；新用户推荐安装包或便携包。

## 校验与验证

```powershell
Get-FileHash .\release\v1.9.4\LianXiangBrowser-Setup-1.9.4.exe -Algorithm SHA256
Get-Content .\release\v1.9.4\SHA256SUMS.txt
npm.cmd --prefix frontend run test:desktop
```

发布前核对：

- `wails.json`、前端包元数据、EXE/安装器版本资源均为 1.9.4。
- ZIP 和安装器内的主程序与本地 EXE 的 SHA-256 一致。
- 包内不包含数据库、用户配置、Cookie、日志、备份文件或敏感凭据；`config.yaml` 来自 `publish/config.init.yaml`。
- 在独立临时目录验证便携程序启动、版本显示和退出，不运行安装器覆盖真实安装。
- SHA-256 清单与最终发行目录中的三个文件逐项一致。

## 升级约定

v1.9.4 继续采用 Go + Wails + 系统 WebView，不执行新的数据库或实例路径迁移。安装器保留原安装位置及已有 `config.yaml`，打包不读取真实用户数据库。

更新便携目录时保留个人配置、数据目录选择文件、数据库和实例目录。不要用模板配置覆盖自己的配置；不需要先卸载旧版或重建实例。

本地生成完成不等于已发布 GitHub Release。发布页简介见 [`github-release-template.md`](github-release-template.md)，完整说明见 [`RELEASE-NOTES-v1.9.4.md`](../RELEASE-NOTES-v1.9.4.md)。
