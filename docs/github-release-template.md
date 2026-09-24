# GitHub Release 简介：v1.9.3

## 标题

```text
v1.9.3 - Windows 实例启动稳定性与完整发布包
```

## 描述

v1.9.3 修复异常退出或升级后残留 Chrome 单实例锁导致浏览器进程在就绪前退出的问题，并重新生成完整 Windows 发布集。

### 主要更新

- 启动前检查用户数据目录中的浏览器进程；确认没有活动进程后，清理 `SingletonLock`、`SingletonCookie` 和 `SingletonSocket` 残留文件。
- 保留正在运行实例的锁文件，避免误伤活动浏览器进程。
- 安装到 `Program Files` 等受保护目录时，将运行数据和单实例锁放到当前用户可写目录。
- 自定义数据目录启动前验证根目录和 `data` 子目录的写入能力；不可写时自动改用用户可写的稳定状态目录。
- Windows 发布包包含本地 EXE、NSIS 安装包、便携 ZIP 和 SHA-256 校验文件；三个产物的摘要统一写入 `SHA256SUMS.txt`。

### Windows 下载

| 文件 | 说明 | SHA-256 |
| --- | --- | --- |
| `LianXiangBrowser-1.9.3-windows-amd64.exe` | 本地 EXE | `f6eced7a38a8c77e45d24d21d830aac3fdb60006ac004df6031cbe1505d6277f` |
| `LianXiangBrowser-Setup-1.9.3.exe` | Windows 安装包 | `e34ce7715fb632ef2ab9dde9f4bc4329286af81658420b9f56e14bb38f587a94` |
| `LianXiangBrowser-1.9.3-windows-amd64-portable.zip` | 便携 ZIP | `bf6fc16714485e3f2e55dbc747fd7028f056c97858496df751773776f54b32ad` |

发布页上传三个 Windows 产物和统一的 `SHA256SUMS.txt` 即可；摘要文件列出每个产物的 SHA-256，以上摘要已由本次构建结果核对。

### 升级提示

- 升级前请关闭浏览器实例并备份 `data` 和用户数据目录。
- 安装版可直接覆盖安装，便携版可解压到新的目录。
- 更改数据目录后需要重启应用使配置生效。

完整记录见 [CHANGELOG.md](../CHANGELOG.md)，构建说明见 [bat/README.md](../bat/README.md)。

