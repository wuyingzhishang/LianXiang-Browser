import type { DesktopCommandMap, DesktopEnvironment, DesktopEventCallback, DesktopHost, Unsubscribe } from './types.ts'

interface WailsRuntime {
  EventsOnMultiple?: (event: string, callback: DesktopEventCallback, count: number) => Unsubscribe
  EventsOn?: (event: string, callback: DesktopEventCallback) => Unsubscribe
  Environment?: () => Promise<DesktopEnvironment>
  WindowHide?: () => void
  WindowMinimise?: () => void
  BrowserOpenURL?: (url: string) => void
}

// Wails globals are accessed only here, and at call time (never cached before startup).
function globals() {
  return globalThis as typeof globalThis & {
    go?: { main?: { App?: DesktopCommandMap } }
    runtime?: WailsRuntime
  }
}

export const wailsHost: DesktopHost = {
  id: 'wails',
  async getBindings() {
    // Keep the generated wrapper's method availability and error semantics intact.
    return import('../wailsjs/go/main/App.js')
  },
  getCommands: () => globals().go?.main?.App ?? null,
  listen(event, callback) {
    const runtime = globals().runtime
    if (runtime?.EventsOnMultiple) return runtime.EventsOnMultiple(event, callback, -1)
    if (runtime?.EventsOn) return runtime.EventsOn(event, callback)
    return () => {}
  },
  environment() {
    const runtime = globals().runtime
    if (!runtime?.Environment) return Promise.reject(new Error('当前环境不支持桌面运行信息'))
    return runtime.Environment()
  },
  hide() {
    const runtime = globals().runtime
    if (!runtime?.WindowHide) throw new Error('当前环境不支持隐藏窗口')
    runtime.WindowHide()
  },
  minimise() {
    const runtime = globals().runtime
    if (!runtime?.WindowMinimise) throw new Error('当前环境不支持最小化窗口')
    runtime.WindowMinimise()
  },
  openURL(url) {
    const runtime = globals().runtime
    if (!runtime?.BrowserOpenURL) throw new Error('当前环境不支持打开外部链接')
    runtime.BrowserOpenURL(url)
  },
}
