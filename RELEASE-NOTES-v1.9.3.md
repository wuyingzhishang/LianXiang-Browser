# 链享浏览器 v1.9.3

v1.9.3 本次同版本更新大幅缩短应用启动时间，移除了个人介绍页面，并重新生成完整的 Windows 发布集。版本号保持不变。

## 主要更新

- 启动提速：运行数据目录同步改为指纹标记校验，布局未变化时跳过全量文件遍历；实测后端启动耗时由约 9.8 秒降至约 0.05 秒。
- 残留进程检查先用系统进程快照预判，无残留时不再调用 PowerShell；回收站清理延后 30 秒执行，不阻塞启动。
- 前端等待后端 `app:ready` 就绪信号后加载实例、内核、代理和分组列表，取代空列表轮询重试；代理绑定校验只读取一次代理目录。
- 启动前确认用户数据目录中没有活动浏览器进程后，清理异常退出遗留的 Chrome 单实例锁文件，降低浏览器进程在完成就绪前退出的概率。
- 正在运行的浏览器实例会保留锁文件，避免清理逻辑影响活动实例。
- 移除侧边栏“关于我”入口、个人介绍页面及其专属配置。
- Windows 安装包继续使用用户可写的运行数据目录，兼容安装在 `Program Files` 等受保护目录的场景。
- 生成本地 EXE、NSIS 安装包和便携 ZIP，三个产物的 SHA-256 统一写入 `SHA256SUMS.txt`。
- 本次构建未包含 Windows 浏览器内核；首次使用前请在应用内下载或自行准备兼容内核。

## Windows 下载

| 文件 | 用途 | SHA-256 |
| --- | --- | --- |
| `LianXiangBrowser-1.9.3-windows-amd64.exe` | 本地 EXE | `c82ab3acbb7523f234987d3803964f97fdeb6687ccbbfe9956d799a9a8364133` |
| `LianXiangBrowser-Setup-1.9.3.exe` | Windows 安装包 | `b7d2a5a169a6df5e6782d155d4a0a7702b0be13f56102973970aceedabd5edab` |
| `LianXiangBrowser-1.9.3-windows-amd64-portable.zip` | 便携 ZIP | `c3cff980429e9ffe18cae951fd69c6582c68c35fa1337968267d5a1016d42db2` |
| `SHA256SUMS.txt` | 三个产物的 SHA-256 汇总 | 以文件内容为准 |

## 升级提示

- 升级前请关闭所有浏览器实例，并备份 `data` 和用户数据目录。
- 安装版可以覆盖安装；便携版请解压到新的目录后再启动。
- 若仍遇到启动失败，请确认内核文件完整，并检查安全软件是否拦截了浏览器可执行文件。

构建和校验说明见 [docs/release-windows.md](docs/release-windows.md)，Release 简介模板见 [docs/github-release-template.md](docs/github-release-template.md)。
