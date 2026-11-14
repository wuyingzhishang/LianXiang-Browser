import type { DesktopBindings, DesktopCommandMap, DesktopCommandResult, DesktopEventCallback, DesktopHost, Unsubscribe } from './types.ts'
import { wailsHost } from './wails.ts'

export type { DesktopHost, DesktopEnvironment } from './types.ts'

let host: DesktopHost = wailsHost
const subscriptions = new Map<string, Set<Unsubscribe>>()
const observers = new Set<(result: DesktopCommandResult) => void>()
const commandFacades = new WeakMap<object, object>()

// Host selection is a bootstrap concern; switching an active UI could route a
// pending operation or its completion event to a different backend.
let hostInUse = false
export function configureDesktopHost(next: DesktopHost): void {
  if (hostInUse) throw new Error('桌面宿主必须在界面启动前配置')
  host = next
}

function currentHost(): DesktopHost {
  hostInUse = true
  return host
}

export function observeDesktopCommands(observer: (result: DesktopCommandResult) => void): Unsubscribe {
  observers.add(observer)
  return () => { observers.delete(observer) }
}

function report(method: string, startedAt: number, success: boolean, error?: unknown) {
  const result = { method, success, durationMs: performance.now() - startedAt, error }
  for (const observer of observers) {
    try { observer(result) } catch { /* Diagnostics must never change a command result. */ }
  }
}

function wrapCommands<T extends object>(commands: T): T {
  const existing = commandFacades.get(commands)
  if (existing) return existing as T
  const methods = new Map<string, { original: Function; wrapped: Function }>()
  // Use a separate target: ES module exports have non-configurable descriptors.
  const facade = new Proxy({}, {
    get(_target, key) {
      const original = Reflect.get(commands, key)
      if (typeof key !== 'string' || typeof original !== 'function') return original
      const cached = methods.get(key)
      if (cached?.original === original) return cached.wrapped
      const wrapped = (...args: unknown[]) => {
        const startedAt = performance.now()
        try {
          const result = Reflect.apply(original, commands, args)
          if (result && typeof result.then === 'function') {
            return result.then((value: unknown) => {
              report(key, startedAt, true)
              return value
            }, (error: unknown) => {
              report(key, startedAt, false, error)
              throw error
            })
          }
          report(key, startedAt, true)
          return result
        } catch (error) {
          report(key, startedAt, false, error)
          throw error
        }
      }
      methods.set(key, { original, wrapped })
      return wrapped
    },
    has: (_target, key) => Reflect.has(commands, key),
    ownKeys: () => Reflect.ownKeys(commands),
    getOwnPropertyDescriptor(_target, key) {
      const descriptor = Reflect.getOwnPropertyDescriptor(commands, key)
      return descriptor ? { ...descriptor, configurable: true } : undefined
    },
  }) as T
  commandFacades.set(commands, facade)
  return facade
}

export async function getDesktopBindings(): Promise<Partial<DesktopBindings> | null> {
  try {
    const bindings = await currentHost().getBindings()
    return bindings ? wrapCommands(bindings) : null
  } catch {
    return null
  }
}

export function getDesktopApp(): DesktopCommandMap | null {
  const commands = currentHost().getCommands()
  return commands ? wrapCommands(commands) : null
}

export async function invokeDesktop<K extends keyof DesktopBindings>(
  method: K,
  ...args: Parameters<DesktopBindings[K]>
): Promise<Awaited<ReturnType<DesktopBindings[K]>>> {
  const bindings = await getDesktopBindings()
  const command = bindings?.[method] ?? getDesktopApp()?.[method]
  if (typeof command !== 'function') throw new Error(`当前环境不支持桌面命令：${method}`)
  return Reflect.apply(command, bindings, args)
}

// Keep the existing runtime names for a mechanical migration of call sites.
// The returned disposer also handles hosts whose registration is asynchronous.
export function EventsOn(event: string, callback: DesktopEventCallback): Unsubscribe {
  const selectedHost = currentHost()
  let active = true
  let release: Unsubscribe | undefined
  const listeners = subscriptions.get(event) ?? new Set<Unsubscribe>()
  subscriptions.set(event, listeners)
  const off = () => {
    if (!active) return
    active = false
    listeners.delete(off)
    if (listeners.size === 0) subscriptions.delete(event)
    release?.()
  }
  listeners.add(off)
  try {
    const registration = selectedHost.listen(event, (...data) => {
      if (active) callback(...data)
    })
    const attach = (unsubscribe: Unsubscribe) => {
      if (active) release = unsubscribe
      else unsubscribe()
    }
    if (typeof registration === 'function') attach(registration)
    else void registration.then(attach, error => {
      off()
      console.error(`桌面事件订阅失败：${event}`, error)
    })
  } catch (error) {
    off()
    throw error
  }
  return off
}

export function EventsOff(event: string, ...additionalEvents: string[]): void {
  for (const name of [event, ...additionalEvents]) {
    for (const off of [...(subscriptions.get(name) ?? [])]) off()
  }
}

export const Environment = () => currentHost().environment()
export const WindowHide = () => currentHost().hide()
export const WindowMinimise = () => currentHost().minimise()
export const BrowserOpenURL = (url: string) => currentHost().openURL(url)
