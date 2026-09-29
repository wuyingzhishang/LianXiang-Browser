# Windows 发布产物

链享浏览器的 Windows 发布由 `bat\publish.bat` 统一生成。脚本会先清理 `publish\output\` 和 `publish\staging\` 中的旧内容，保证目录只保留本次发布结果。版本号沿用 `wails.json` 中的 `1.9.3`，本次为同版本功能更新，不创建新版本目录或升级版本号。

## 生成 v1.9.3 Windows 发布集

推荐使用 PowerShell 7：

```powershell
pwsh -NoProfile -ExecutionPolicy Bypass -File bat\publish.ps1 `
  -Target WINDOWS -WindowsFormat BOTH -Version 1.9.3
```

批处理入口：

```bat
bat\publish.bat both -Version 1.9.3
```

如果 NSIS 不在 PATH 中，设置 `NSIS_PATH`、`NSIS_HOME` 或 `MAKENSIS_PATH` 指向 NSIS 安装目录或 `makensis.exe`。

## 输出

全部文件写入 `publish\output\`：

```text
LianXiangBrowser-1.9.3-windows-amd64.exe
LianXiangBrowser-Setup-1.9.3.exe
LianXiangBrowser-1.9.3-windows-amd64-portable.zip
SHA256SUMS.txt
```

`SHA256SUMS.txt` 将本地 EXE、安装包和便携 ZIP 的 SHA-256 汇总到一个文本文件。

安装包和便携 ZIP 使用同一份 staging 内容，包含应用本体、默认配置、Xray、sing-box、诊断脚本以及可发现的浏览器内核；发布包不包含用户数据库、日志和敏感配置。

## 校验

```powershell
Get-FileHash .\publish\output\LianXiangBrowser-Setup-1.9.3.exe -Algorithm SHA256
Get-Content .\publish\output\SHA256SUMS.txt
```

`publish\output\` 和 `publish\staging\` 不提交到 Git。重新运行脚本会删除旧产物和校验文件，避免把旧版本混入 Release。

## 同步发行目录与文档

构建通过后，将 `publish\output\` 的三个 Windows 二进制产物和 `SHA256SUMS.txt` 同步到 `release\v1.9.3\`，保留该目录的 `README.md`，删除其中本次产物清单以外的旧文件。发行目录只保留以下内容：

```text
LianXiangBrowser-Setup-1.9.3.exe
LianXiangBrowser-1.9.3-windows-amd64.exe
LianXiangBrowser-1.9.3-windows-amd64-portable.zip
README.md
SHA256SUMS.txt
```

`SHA256SUMS.txt` 只记录三个可下载产物；不要把 README 或摘要文件本身列入摘要。Release 页面上传这三个二进制文件及 `SHA256SUMS.txt`，简介以 `docs/github-release-template.md` 为准。
