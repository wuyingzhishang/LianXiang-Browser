# 链享浏览器

链享浏览器是一款用于管理独立浏览器环境、代理出口、浏览器内核和自动化任务的本地桌面应用。

当前版本：`1.4.6`
主要技术：Go、Wails、React、TypeScript  
当前已验证平台：Windows amd64

## 二次开发来源与致谢

本项目基于以下开源仓库进行二次开发：

- 原作者/上游项目：`black-ant/Ant-Browser`
- 原仓库地址：<https://github.com/black-ant/Ant-Browser>
- 本次审计参考提交：`a66b27ec2929e236bbf7877c3b4cc437be9ff609`

感谢原作者及上游贡献者提供的浏览器实例管理、代理连接、自动化和桌面端基础实现。

“链享浏览器”是二次开发版本的名称，并非原作者发布的官方版本。二开中的品牌、安全基线、依赖版本和部分界面内容已经调整，问题反馈应先针对本项目排查，不应默认归因于上游作者。

> 重要：上游仓库和当前项目副本均未发现独立的 `LICENSE` 文件。没有许可证不等于自动获得复制、修改或商业分发授权。公开发布、销售或向第三方分发前，应先向上游权利人确认授权范围，并补齐适用的许可证和版权声明。

## 项目定位

链享浏览器适用于需要在一台设备上管理多个隔离浏览器环境的场景，例如：

- 多账号环境隔离
- 为不同实例绑定独立代理
- 浏览器内核和插件统一管理
- 本地测试与自动化任务
- 实例配置、用户数据和运行记录备份

本项目只提供环境管理能力。使用者应遵守目标网站规则、所在地区法律以及代理服务的使用约定。

## 主要功能

### 浏览器实例

- 创建、编辑、复制、启动、停止和删除实例
- 独立保存浏览器用户数据目录
- 配置启动参数、指纹参数、标签、分组和快捷启动码
- 导入、导出实例配置及用户数据
- 直接导入 Chrome 的 `--user-data-dir` 目录，并作为普通实例继续编辑指纹、代理和内核

### 默认书签

- 手动添加未分组书签或书签分组
- 使用 `工作/AI` 形式维护多级书签目录
- 导入 Chrome、Edge、Firefox 导出的 HTML 书签文件，并保留原目录结构
- 按 URL 自动跳过重复项，可增量同步到已有未运行实例
- 可全局关闭单条书签，也可只对指定实例关闭；实例再次启动时按当前设置写入书签栏
- 关闭书签时同步过滤“启动打开”网址，并只移除由链享浏览器写入的对应书签

### 窗口同步器

- 从运行中的实例里设置一个主窗口和一个或多个被控窗口
- 按实例分组筛选并批量选择窗口，随时恢复显示指定浏览器窗口
- 同步当前活动网页中的点击、确认键、输入内容、滚动位置和页面跳转
- 批量平铺、层叠、最大化、最小化或恢复所选浏览器窗口
- 向各窗口当前聚焦的输入框批量写入或清空文本，并逐窗口反馈执行结果
- 批量新建、跳转、刷新或关闭当前标签页，也可从主控窗口覆盖同步全部普通网页标签页
- 同步过程中实时显示实例状态、事件数量和连接错误，批量操作支持部分成功结果提示

窗口同步和批量管理使用实例本机 CDP 调试通道。网页事件同步只作用于当前活动网页；Chrome 地址栏、书签栏、扩展弹窗、下载框和系统对话框属于浏览器原生界面，不在同步范围内。覆盖同步标签页前会要求确认，并保留一个可用页面完成替换，避免因关闭最后一个标签页导致浏览器退出。

### 代理与网络

- 维护代理池并为单个实例绑定代理
- 支持代理测速、真实连通性检查和 IP 健康检查
- 支持 Xray、sing-box 和 Mihomo 连接栈
- 支持 HTTP、SOCKS5、Shadowsocks、VMess、VLESS、Trojan、Hysteria2、TUIC、AnyTLS 等协议，具体能力取决于所选运行时

默认 `xray` 连接栈实际是组合栈：

- `xray.exe`：主要处理 VMess、VLESS、Trojan、Shadowsocks 和链式代理
- `sing-box.exe`：主要处理 Hysteria2、TUIC、AnyTLS 等协议
- Mihomo：用于 Clash/Mihomo 兼容连接栈

### 内核与插件

- 添加和切换多个 Chromium/Chrome 内核
- 设置默认浏览器内核
- 安装、导入、启停和删除插件包
- 按实例控制插件启用状态

### 自动化与本地 API

- 导入和运行自动化脚本
- 选择一个或多个目标实例
- 保存脚本运行记录和结果
- 通过本地 Launch API 管理实例和运行状态
- API 默认仅监听本机，并支持 API Key 鉴权

## 安全说明

二开前已对上游源码、前端依赖和 Go 调用路径进行检查。当前结论：

- 未在源码层发现明显后门、远控上报、未知私有域名回传或开机持久化逻辑
- 前端 `npm audit` 当前为 0 vulnerabilities
- `govulncheck ./...` 当前未发现代码可达漏洞
- Launch API 已改为默认请求鉴权
- 鉴权启用但 API Key 为空时采用 fail-closed，不会放行 `/api/*`
- 构建脚本不再默认强制使用本机 `127.0.0.1:7890` 代理

完整记录见 [SECURITY_AUDIT.md](SECURITY_AUDIT.md)。

### 第三方二进制边界

仓库包含 Xray 和 sing-box 的原生可执行文件。源码审计不能证明这些二进制的内部行为，公开发布前应从官方 Release 重新下载并核对版本及哈希：

- Xray：<https://github.com/XTLS/Xray-core>
- sing-box：<https://github.com/SagerNet/sing-box>
- 固定来源：[publish/runtime-sources.json](publish/runtime-sources.json)
- 哈希清单：[publish/runtime-manifest.json](publish/runtime-manifest.json)

## 快速使用

### 运行已构建版本

Windows 可执行文件：

```text
build\bin\lianxiang-browser.exe
```

代理运行时应位于：

```text
build\bin\bin\xray.exe
build\bin\bin\sing-box.exe
```

直接启动 `lianxiang-browser.exe` 后，建议按以下顺序操作：

1. 在“内核管理”中添加可用的 Chromium/Chrome 内核。
2. 在“代理池配置”中导入代理并执行测速。
3. 在“实例列表”中新建浏览器实例，或导入已有 Chrome 用户数据。
4. 为实例选择内核、代理、标签和启动参数。
5. 启动实例并检查代理出口是否符合预期。
6. 需要批量操作时，再进入“自动化脚本”配置运行环境和任务。

### 准备浏览器内核

代理运行时不等于浏览器内核。运行实例前仍需准备 Chromium/Chrome：

```text
chrome\
  chromium-<version>\
    chrome.exe
```

也可以在应用的“内核管理”页面中配置其他本地路径。指纹 Chromium 可参考：

<https://github.com/adryfish/fingerprint-chromium>

### 导入现有 Chrome 用户数据

如果 Chrome 通过独立用户目录启动，例如：

```powershell
& 'C:\Program Files\Google\Chrome\Application\chrome.exe' --user-data-dir='D:\Gugeduo\5'
```

可以把这个目录直接复制为链享浏览器实例：

1. 关闭所有使用该用户数据目录的 Chrome 窗口。
2. 打开“实例列表”，点击“导入”。
3. 选择“Chrome 用户数据目录”，按需填写新实例名称。
4. 选择完整的 `D:\Gugeduo\5`，不要只选择其中的 `Default` 文件夹。
5. 导入完成后，可像普通实例一样编辑指纹、代理、内核和启动参数。

导入规则：

- 源目录必须包含 `Local State`，并至少包含一个有效的 `Default` 或 `Profile *` 配置目录。
- 系统会复制数据为新的独立实例，不会让链享浏览器与原 Chrome 同时写入源目录。
- 复制时会跳过缓存、临时文件、`Singleton*` 锁文件和其他无需迁移的运行态文件。
- 如果检测到源目录仍被 Chrome 占用，导入会停止并要求先关闭对应窗口。
- 书签、扩展、历史记录和可用的登录数据会随目录复制；受 Chrome 或 Windows 加密保护的数据可能仍受当前系统用户和浏览器内核限制。
- 修改导入实例的指纹并保存时，应用会提醒网站可能将其识别为新设备，并触发风控验证、退出登录或要求重新登录。

## Launch API 鉴权

API Key 不应硬编码到源码、示例配置或提交记录中。程序从环境变量 `LIANXIANG_BROWSER_API_KEY` 读取 Key。

PowerShell 临时启动示例：

```powershell
$key = [Guid]::NewGuid().ToString('N')
$env:LIANXIANG_BROWSER_API_KEY = $key
Write-Output $key
& '.\build\bin\lianxiang-browser.exe'
```

默认请求头：

```text
X-LianXiang-Api-Key
```

未设置 Key 时，普通桌面功能可以运行，但受保护的 `/api/*` 请求会被拒绝。应用内“文档中心”包含本地 API 的接口说明和调用示例。

## 从源码开发

### 环境要求

- Windows 10/11 amd64
- Go 1.25 或更高版本
- Node.js 22 LTS
- npm
- Wails CLI 2.12.0
- Microsoft Edge WebView2 Runtime

安装与项目版本一致的 Wails CLI：

```powershell
go install github.com/wailsapp/wails/v2/cmd/wails@v2.12.0
```

检查开发环境：

```powershell
wails doctor
```

### 安装前端依赖

```powershell
Set-Location '.\frontend'
npm install
Set-Location '..'
```

### 开发运行

稳定模式：

```powershell
.\bat\dev.bat
```

前端热更新模式：

```powershell
.\bat\dev.bat live
```

受限内存模式：

```powershell
.\bat\dev.bat limited
```

### 构建 Windows 可执行文件

```powershell
.\bat\build.bat
```

输出：

```text
build\bin\lianxiang-browser.exe
```

构建默认使用当前系统网络配置。如需显式代理：

```powershell
$env:LIANXIANG_BUILD_PROXY = 'http://127.0.0.1:7890'
$env:LIANXIANG_GOPROXY = 'https://goproxy.cn,direct'
.\bat\build.bat
```

脚本详细说明见 [bat/README.md](bat/README.md)。

## 验证命令

前端：

```powershell
Set-Location '.\frontend'
npm audit
npm run build
Set-Location '..'
```

Go：

```powershell
gofmt -w backend\bootstrap.go backend\internal\config\config_io.go backend\internal\config\config_defaults.go backend\internal\launchcode\auth.go backend\internal\launchcode\auth_test.go backend\internal\fsutil\path.go
go test ./...
govulncheck ./...
```

完整桌面构建：

```powershell
wails build
```

## 项目目录

```text
backend/       Go 后端、实例管理、代理桥接和自动化服务
frontend/      React + TypeScript 管理界面
bat/           Windows 开发、构建和发布脚本
bin/           Xray、sing-box 等代理运行时
chrome/        本地浏览器内核目录
data/          本地数据库、实例数据和运行记录
publish/       发布脚本、运行时来源和哈希清单
tools/         维护和发布辅助工具
config.yaml    默认应用配置
wails.json     Wails 应用与产品信息
```

## 数据与隐私

- 浏览器实例数据、SQLite 数据库和自动化运行记录默认保存在本地 `data/`。
- Chrome 用户数据导入会在实例数据根目录下创建独立副本；原始 `--user-data-dir` 不会被修改。
- `.env`、账号、Cookie、Token、代理订阅和本地敏感配置已列入 `.gitignore`。
- 不要把真实 API Key、账号、代理凭据或浏览器用户数据提交到代码仓库。
- 分享日志、截图、数据库或导出包前，应先检查账号、IP、Cookie、Token 和个人身份信息。

## 已知限制

- 当前二开版本仅完成 Windows amd64 的完整构建和启动验证。
- Linux/macOS 代码和发布脚本继承自上游，但本次二开尚未重新完成全平台打包验证。
- 当前没有随项目提供可直接使用的浏览器内核，需要用户自行配置。
- Chrome 加密的 Cookie、密码或令牌可能与 Windows 用户、Chrome 安装或安全机制绑定，复制目录不保证所有登录状态都能跨内核继续使用。
- Xray、sing-box 以及用户选择的 Chromium 内核属于第三方二进制，需要独立确认来源和安全性。
- Windows 可执行文件当前未进行代码签名，系统可能显示未知发布者提示。

## GitHub Releases

链享浏览器使用当前项目自己的 GitHub Releases 发布版本，不能直接使用上游 `black-ant/Ant-Browser` 的 Release 地址或文件名。

- 项目仓库：<https://github.com/TIANYA689/LianXiang-Browser>
- Releases：<https://github.com/TIANYA689/LianXiang-Browser/releases>
- 当前版本：<https://github.com/TIANYA689/LianXiang-Browser/releases/tag/v1.4.6>

当前源码版本为 `v1.4.6`。本次发布包含窗口同步器优化与 Chrome 151 启动断点异常自动恢复，Windows amd64 本地程序、安装包和便携包统一使用 `1.4.6`。Linux 和 macOS 脚本已保留，但对应产物需要在各自平台完成构建验证后再上传。

建议的首次上传流程：

```powershell
Set-Location 'D:\path\to\LianXiang-Browser'
git init
git add .
git commit -m 'chore: initialize LianXiang Browser'
git branch -M main
git remote add origin 'git@github.com:TIANYA689/LianXiang-Browser.git'
git push -u origin main
```

创建版本标签并推送：

```powershell
git switch main
git pull
git tag -a v1.4.6 -m 'LianXiang Browser v1.4.6'
git push origin v1.4.6
```

然后在你自己的 GitHub 仓库中进入 `Releases`，选择刚推送的 `v1.4.6` 标签创建发布，并上传构建产物。

Windows 发布：

```powershell
.\bat\publish.bat zip
.\bat\publish.bat both
```

产物名称：

```text
publish\output\LianXiang-Browser-Setup-<version>.exe
publish\output\LianXiang-Browser-<version>-windows-amd64-portable.zip
publish\output\SHA256SUMS-<version>.txt
```

其中 `zip` 只生成便携包，`both` 同时生成安装包和便携包；两种模式都会为本次 Windows 产物生成 SHA-256 校验文件。安装包需要 NSIS；便携包不依赖 NSIS。Linux 和 macOS 发布脚本会生成对应架构的 `.deb`、`.tar.gz`、`.app` 或 `.zip`，但当前二开版本尚未完成全平台重新验证。

发布前检查：

- 发布版本号应与 `wails.json` 的 `productVersion` 一致。
- 不要把 `data/`、数据库、浏览器用户目录、Cookie、Token、代理订阅或 `.env` 上传到 Release。
- Release 说明中应保留上游项目链接、二开说明和第三方二进制来源。
- Xray、sing-box 和浏览器内核应在发布前重新核对来源与哈希。

## 分支建议

上游仓库的分支不会自动出现在你的仓库中。你可以根据维护需要建立自己的分支：

```text
main              稳定版本，只合并已验证的代码
develop           日常开发和联调
feature/<name>    单个功能开发
fix/<name>        问题修复
release/v<ver>    发布前冻结与验收
```

创建开发分支示例：

```powershell
git switch -c develop
git push -u origin develop
git switch -c feature/profile-docs
git push -u origin feature/profile-docs
```

推荐流程是从 `feature/*` 合并到 `develop`，验证通过后再合并到 `main`，最后从 `main` 创建版本标签和 GitHub Release。

## 相关文档

- [安全审计记录](SECURITY_AUDIT.md)
- [更新记录](CHANGELOG.md)
- [v1.4.6 发布说明](publish/RELEASE_NOTES-1.4.6.md)
- [Windows 脚本说明](bat/README.md)
- [Linux 发布说明](publish/linux/README.md)
- [macOS 发布说明](publish/mac/README.md)

## 版权与授权

- 上游项目及其历史代码版权归原作者和各贡献者所有。
- 链享浏览器二开部分的署名不能替代上游版权声明。
- 原作者仓库：<https://github.com/black-ant/Ant-Browser>
- 在没有明确许可证或书面授权前，不应假设本项目可以自由商业分发。

公开发布前应补齐：上游授权确认、项目许可证、第三方依赖许可证清单、二进制来源证明和必要的版权声明。
