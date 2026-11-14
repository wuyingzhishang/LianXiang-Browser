# v1.9.4 — 书签管理、Chrome 多开导入与退出优化

本版加入书签分组与实例范围控制，支持将已有 Chrome 本地多开数据复制为独立实例，并优化退出弹窗及后台关闭流程。

- 书签支持分组、排序、导入和全局/单实例启用控制。
- Chrome 用户数据目录导入包含占用检查、路径检查、缓存过滤与失败回滚。
- WebView2 默认启用 GPU 加速，简化动画，移除退出前固定等待；后台清理不再阻塞退出请求。
- 统一桌面命令与事件接口，保留原有调用和数据格式。
- 更新 Windows 本地 EXE、NSIS 安装包、便携 ZIP 与 SHA-256 校验文件。

**数据兼容：**继续使用 Go + Wails + 系统 WebView，没有切换 Rust/Tauri，不更改数据库结构、配置格式或实例路径。升级时保留个人配置与数据，无需先卸载或迁移数据库。

**运行要求：**Windows x64、WebView2 Runtime。安装包和便携包附带 Xray 与 sing-box，本次未捆绑 Chrome 内核；首次使用请通过“内核管理”下载或导入兼容内核。单独 EXE 不包含外部运行时。

## 文件校验

| 文件 | SHA-256 |
| --- | --- |
| `LianXiangBrowser-1.9.4-windows-amd64.exe` | `b69212ecfa3b33794c0cb9126ccf3010231a881692adb4ff3102246c4265c47c` |
| `LianXiangBrowser-1.9.4-windows-amd64-portable.zip` | `3a7a1ce366e91c968e84b0b16d88b1179f02c9433350f2db03b7134add37bf7a` |
| `LianXiangBrowser-Setup-1.9.4.exe` | `081df390c5f4ac8fc087a39303b43a6324d5ac7399affa7114dbf678ecfa4fce` |

完整更新说明：[RELEASE-NOTES-v1.9.4.md](../RELEASE-NOTES-v1.9.4.md)。
