# 链享浏览器 v1.9.4

v1.9.4 加入书签管理与 Chrome 本地多开数据导入，改善退出弹窗和后台关闭流程，并更新 Windows 本地程序、安装包、便携包及 SHA-256 校验文件。

## 主要更新

- 书签支持分组、排序、导入及全局/单实例启用控制。
- 可将已有 Chrome 用户数据根目录复制为独立实例；导入前检查目录占用和路径重叠，跳过缓存与临时锁文件，失败时回滚新建内容。
- Windows WebView2 默认启用 GPU 加速，简化弹窗动画，移除退出前的固定等待。
- 后台异步清理浏览器与服务，退出请求去重，保留“仅退出应用”和“退出应用与浏览器”两种选择。
- 统一桌面命令、事件和窗口操作入口，保持现有调用与数据接口兼容。
- 发布输出可按版本隔离，清理操作限定为本次 staging，避免误删旧输出目录中的运行数据。

## 数据兼容

本版仍为 Go + Wails + React，前端使用系统 WebView；没有引入 Rust/Tauri，没有修改数据库结构、配置格式或实例数据路径。

升级前正常退出应用；安装版沿用原路径，保留已有配置和数据。更新便携目录时不要覆盖个人配置或删除实例目录，不需要先卸载或迁移数据库。

## Windows x64 下载

- `LianXiangBrowser-Setup-1.9.4.exe`：安装版。
- `LianXiangBrowser-1.9.4-windows-amd64-portable.zip`：便携版，包含 Xray、sing-box 和默认配置。
- `LianXiangBrowser-1.9.4-windows-amd64.exe`：单独主程序，用于已有完整环境或手动部署。
- `SHA256SUMS.txt`：三个产物的 SHA-256 校验清单。

安装包与便携包不包含用户数据库或 Chrome 内核。需要 WebView2 Runtime；首次使用请在“内核管理”中准备兼容内核。Chrome 登录凭据受系统和浏览器加密限制，目录导入不保证跨机器保留全部登录状态。

本次仅生成 Windows x64 产物。完整变更见 [CHANGELOG.md](CHANGELOG.md)，构建方式见 [Windows 发布说明](docs/release-windows.md)。
