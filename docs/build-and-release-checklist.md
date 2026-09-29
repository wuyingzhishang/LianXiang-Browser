# 构建与发布检查清单 v1.9.3（同版本更新）

## 版本与文档

- [x] `wails.json` productVersion 为 `1.9.3`
- [x] `frontend/package.json` 和 `package-lock.json` 为 `1.9.3`
- [x] `frontend/package.json.md5` 已重新计算
- [x] `README.md`、`CHANGELOG.md` 和 Windows 发布文档已更新
- [x] GitHub Release 简介已写入 `docs/github-release-template.md`，并填入本次构建摘要
- [x] 版本号保持 `1.9.3`；本次更新不创建新版本号
- [x] 主 README、CHANGELOG、Release Notes 与 Windows 发布文档已同步“关于我”页面移除说明

## 构建与发布

- [x] Go 应用构建通过
- [x] 前端生产构建通过
- [x] Windows 本地 EXE 已生成
- [x] Windows NSIS 安装包已生成
- [x] Windows 便携 ZIP 已生成
- [x] 所有 Windows 产物的 SHA-256 汇总到单一 `SHA256SUMS.txt`
- [x] `SHA256SUMS.txt` 已在本次构建后生成并与产物匹配

## 产物

发布目录为 `release/v1.9.3/`，只包含 Windows 安装包、本地 EXE、便携 ZIP、统一的 `SHA256SUMS.txt` 和 `README.md`，不包含数据库、日志、备份凭据或 staging 临时文件。相同版本目录更新时，删除旧的多余产物后再核对清单。

## 验证

```powershell
Get-Content .\publish\output\SHA256SUMS.txt
Get-FileHash .\publish\output\LianXiangBrowser-Setup-1.9.3.exe -Algorithm SHA256
Get-FileHash .\publish\output\LianXiangBrowser-1.9.3-windows-amd64-portable.zip -Algorithm SHA256
```
