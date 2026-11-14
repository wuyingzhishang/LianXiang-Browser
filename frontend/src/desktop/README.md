# 桌面接口边界

业务代码统一从本目录调用桌面能力。当前实现仍为 Wails；这一层不读写数据库或本地存储，不转换数据格式。

- `getDesktopBindings()`：获取与现有生成绑定兼容的命令对象；动态加载失败仍返回 `null`。
- `invokeDesktop(name, ...args)`：调用单个命令，由现有生成类型检查位置参数和返回类型。错误保持原样传递。
- `getDesktopApp()`：兼容旧代码对宿主额外方法的能力检测；返回受日志观察的命令对象，业务代码不得修改其方法。
- `EventsOn(name, callback)`：保留事件名和全部参数，返回当前订阅的取消函数。在 React effect 中返回该函数即可；不要以 `EventsOff(name)` 代替单个订阅的取消。
- `Environment`、`WindowHide`、`WindowMinimise`、`BrowserOpenURL`：现有窗口能力的宿主无关入口。
- `observeDesktopCommands()`：只观察结果、耗时和错误，不记录参数或返回数据。观察函数抛出错误不会改变命令结果。

`wails.ts` 是唯一允许访问 Wails 全局对象及动态加载生成绑定的实现。不要手动修改 `wailsjs`；生成绑定可继续使用原有脚本。

未来实现 Rust/Tauri 宿主时，遵守 `types.ts` 的 `DesktopHost` 契约，在 React 挂载前调用 `configureDesktopHost()`。运行中禁止切换宿主。异步事件注册由适配层负责处理卸载竞态。

生产宿主必须实现现有页面需要的命令及原有取消/失败语义。不得把不支持的写操作返回成成功，也不得借此更改 SQLite 表结构、配置键、数据路径或代理连接栈。当前适配层测试中的替代宿主只是契约夹具，不是已完成的 Rust 实现。

运行 `npm.cmd run test:desktop` 验证调用契约和接口边界（Node 22.18+ 或 24+）。测试不启动真实应用，不访问用户数据库。
