# 链享浏览器 v1.9.3 Windows 发布包

本目录包含 v1.9.3 的 Windows 发行文件。

## 文件说明

- `LianXiangBrowser-Setup-1.9.3.exe`：NSIS 安装版，适合常规安装和覆盖升级。
- `LianXiangBrowser-1.9.3-windows-amd64.exe`：本地单文件 EXE，适合直接运行或自行集成。
- `LianXiangBrowser-1.9.3-windows-amd64-portable.zip`：便携版，解压到目录后运行，不写入安装程序注册信息。
- `SHA256SUMS.txt`：以上三个文件的 SHA-256 汇总。

## 升级提示

升级前请关闭所有浏览器实例，并备份 `data` 和用户数据目录。安装版可以覆盖安装；便携版请解压到新的目录后再启动。若启动失败，请确认浏览器内核文件完整，并检查安全软件是否拦截了浏览器可执行文件。

摘要应以同目录的 `SHA256SUMS.txt` 为准。完整更新说明见仓库根目录的 `RELEASE-NOTES-v1.9.3.md` 和 `CHANGELOG.md`。
