# 链享浏览器 v1.9.3

v1.9.3 聚焦 Windows 实例启动稳定性，并重新生成完整的 Windows 发布集。

## 主要更新

- 启动前确认用户数据目录中没有活动浏览器进程后，清理异常退出遗留的 Chrome 单实例锁文件，降低浏览器进程在完成就绪前退出的概率。
- 正在运行的浏览器实例会保留锁文件，避免清理逻辑影响活动实例。
- Windows 安装包继续使用用户可写的运行数据目录，兼容安装在 `Program Files` 等受保护目录的场景。
- 生成本地 EXE、NSIS 安装包和便携 ZIP，三个产物的 SHA-256 统一写入 `SHA256SUMS.txt`。

## Windows 下载

| 文件 | 用途 | SHA-256 |
| --- | --- | --- |
| `LianXiangBrowser-1.9.3-windows-amd64.exe` | 本地 EXE | `f6eced7a38a8c77e45d24d21d830aac3fdb60006ac004df6031cbe1505d6277f` |
| `LianXiangBrowser-Setup-1.9.3.exe` | Windows 安装包 | `e34ce7715fb632ef2ab9dde9f4bc4329286af81658420b9f56e14bb38f587a94` |
| `LianXiangBrowser-1.9.3-windows-amd64-portable.zip` | 便携 ZIP | `bf6fc16714485e3f2e55dbc747fd7028f056c97858496df751773776f54b32ad` |
| `SHA256SUMS.txt` | 三个产物的 SHA-256 汇总 | 以文件内容为准 |

## 升级提示

- 升级前请关闭所有浏览器实例，并备份 `data` 和用户数据目录。
- 安装版可以覆盖安装；便携版请解压到新的目录后再启动。
- 若仍遇到启动失败，请确认内核文件完整，并检查安全软件是否拦截了浏览器可执行文件。

构建和校验说明见 [docs/release-windows.md](docs/release-windows.md)，Release 简介模板见 [docs/github-release-template.md](docs/github-release-template.md)。
