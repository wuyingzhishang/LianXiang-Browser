# GitHub Release 简介：v1.9.3（同版本更新）

## 标题

```text
v1.9.3 - 启动提速、界面精简与 Windows 发布包更新
```

## 描述

v1.9.3 同版本更新：应用启动由约 10 秒缩短到 1 秒以内，移除个人介绍页面，重新构建 Windows 安装包、本地 EXE 和便携 ZIP。版本号保持不变。

### 主要更新

- 启动提速：运行数据目录同步改为指纹标记校验，布局未变化时跳过全量文件遍历；实测后端启动耗时由约 9.8 秒降至约 0.05 秒。
- 残留进程检查先用系统进程快照预判，无残留时不再调用 PowerShell；回收站清理延后 30 秒执行。
- 实例、内核、代理和分组列表在后端发出 `app:ready` 就绪信号后加载，取代空列表轮询重试；代理绑定校验只读取一次代理目录。
- 启动前检查用户数据目录中的浏览器进程；确认没有活动进程后，清理 `SingletonLock`、`SingletonCookie` 和 `SingletonSocket` 残留文件。
- 保留正在运行实例的锁文件，避免误伤活动浏览器进程。
- 移除侧边栏“关于我”入口及对应的个人介绍页面和配置。
- 安装到 `Program Files` 等受保护目录时，将运行数据和单实例锁放到当前用户可写目录。
- 自定义数据目录启动前验证根目录和 `data` 子目录的写入能力；不可写时自动改用用户可写的稳定状态目录。
- Windows 发布包包含本地 EXE、NSIS 安装包、便携 ZIP 和 SHA-256 校验文件；三个产物的摘要统一写入 `SHA256SUMS.txt`。
- 本次构建环境未提供可分发的 Windows 浏览器内核；首次使用前请在应用内下载或自行准备兼容内核。

### Windows 下载

| 文件 | 说明 | SHA-256 |
| --- | --- | --- |
| `LianXiangBrowser-1.9.3-windows-amd64.exe` | 本地 EXE | `c82ab3acbb7523f234987d3803964f97fdeb6687ccbbfe9956d799a9a8364133` |
| `LianXiangBrowser-Setup-1.9.3.exe` | Windows 安装包 | `b7d2a5a169a6df5e6782d155d4a0a7702b0be13f56102973970aceedabd5edab` |
| `LianXiangBrowser-1.9.3-windows-amd64-portable.zip` | 便携 ZIP | `c3cff980429e9ffe18cae951fd69c6582c68c35fa1337968267d5a1016d42db2` |

发布页上传三个 Windows 产物和统一的 `SHA256SUMS.txt`。上述摘要与本次 `release/v1.9.3/SHA256SUMS.txt` 一致。

### 升级提示

- 升级前请关闭浏览器实例并备份 `data` 和用户数据目录。
- 安装版可直接覆盖安装，便携版可解压到新的目录。
- 更改数据目录后需要重启应用使配置生效。

完整记录见 [CHANGELOG.md](../CHANGELOG.md)，构建说明见 [bat/README.md](../bat/README.md)。

