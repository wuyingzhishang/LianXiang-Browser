# 链享浏览器 v1.9.4 Windows 发布包

本目录提供 v1.9.4 的完整 Windows x64 发布集。主要更新为书签管理、Chrome 本地多开数据导入、退出响应优化和桌面接口兼容整理。

| 文件 | 用途 |
| --- | --- |
| `LianXiangBrowser-Setup-1.9.4.exe` | NSIS 安装包，可在原安装路径升级 |
| `LianXiangBrowser-1.9.4-windows-amd64.exe` | 单独主程序，用于已有完整环境 |
| `LianXiangBrowser-1.9.4-windows-amd64-portable.zip` | 便携包，包含主程序、Xray、sing-box、默认配置和诊断脚本 |
| `SHA256SUMS.txt` | 上述三个文件的 SHA-256 校验值 |

本版保留 Go + Wails + 系统 WebView 架构，沿用原数据库、配置和实例目录格式。升级时保留自己的配置与数据，不需要迁移数据库。安装包与便携包不包含用户数据库或 Chrome 内核；首次使用需准备 WebView2 Runtime 和兼容内核。

校验文件以本目录 `SHA256SUMS.txt` 为准。完整说明见 [v1.9.4 Release Notes](../../RELEASE-NOTES-v1.9.4.md)。旧版本独立发布文件已清理，历史变更保留在 [CHANGELOG](../../CHANGELOG.md)。
