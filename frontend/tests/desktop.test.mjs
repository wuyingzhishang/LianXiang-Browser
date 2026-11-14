import assert from 'node:assert/strict'
import { test } from 'node:test'
import { readFile, readdir } from 'node:fs/promises'
import * as generated from '../src/wailsjs/go/main/App.js'

let moduleId = 0
const freshBridge = () => import(`../src/desktop/index.ts?test=${++moduleId}`)
const tick = () => new Promise(resolve => setImmediate(resolve))

function host(overrides = {}) {
  return {
    id: 'fixture',
    getBindings: async () => null,
    getCommands: () => null,
    listen: () => () => {},
    environment: async () => ({ platform: 'windows', arch: 'amd64', buildType: 'production' }),
    hide() {}, minimise() {}, openURL() {},
    ...overrides,
  }
}

function setWails(t, app, runtime) {
  globalThis.window = globalThis
  globalThis.go = { main: { App: app } }
  globalThis.runtime = runtime
  t.after(() => {
    delete globalThis.window
    delete globalThis.go
    delete globalThis.runtime
  })
}

test('every generated command retains positional arguments, DTO identity and method availability', async t => {
  const sent = []
  const payload = Object.freeze({ id: '旧实例-123', path: 'D:\\旧数据\\Default', unknownField: null })
  const result = Object.freeze({ payload, items: Object.freeze([payload]), count: 9007199254740991 })
  const app = Object.fromEntries(Object.keys(generated).map(name => [name, (...args) => {
    sent.push({ name, args })
    return Promise.resolve(result)
  }]))
  setWails(t, app)
  const bridge = await freshBridge()
  const bindings = await bridge.getDesktopBindings()
  assert.deepEqual(Object.keys(bindings).sort(), Object.keys(generated).sort())
  assert.equal(bindings.NonexistentCommand, undefined)
  for (const [name, original] of Object.entries(generated)) {
    const args = Array.from({ length: original.length }, (_, index) =>
      [payload, '中文 & % / \\ 路径', false, null, 0, result][index % 6])
    assert.equal(await bindings[name](...args), result, name)
    assert.equal(sent.at(-1).name, name)
    assert.deepEqual(sent.at(-1).args, args, name)
    args.forEach((arg, index) => assert.equal(sent.at(-1).args[index], arg))
  }
  assert.equal(sent.length, Object.keys(generated).length, 'one host call per command, no replay')
})

test('command failures and return values survive diagnostics without mutating the host', async t => {
  const rejection = { code: 'EXISTING_DATA', message: '保存失败，保留原数据' }
  const syncError = new Error('同步失败')
  const commands = {
    ForceQuit: () => Promise.reject(rejection),
    QuitAppOnly: () => { throw syncError },
    ExtraCommand: () => 42,
  }
  const original = commands.ForceQuit
  setWails(t, commands)
  const bridge = await freshBridge()
  const reported = []
  bridge.observeDesktopCommands(() => { throw new Error('diagnostics failed') })
  const stop = bridge.observeDesktopCommands(result => reported.push(result))
  await assert.rejects(bridge.invokeDesktop('ForceQuit'), error => error === rejection)
  assert.throws(() => bridge.getDesktopApp().QuitAppOnly(), error => error === syncError)
  assert.equal(bridge.getDesktopApp().ExtraCommand(), 42)
  assert.deepEqual(reported.map(({ method, success }) => [method, success]), [
    ['ForceQuit', false], ['QuitAppOnly', false], ['ExtraCommand', true],
  ])
  assert.equal(commands.ForceQuit, original)
  stop()
  bridge.getDesktopApp().ExtraCommand()
  assert.equal(reported.length, 3)
})

test('replacement host uses the same commands and cannot switch after startup', async () => {
  const bridge = await freshBridge()
  let selectedName
  const result = { cancelled: false, profileId: 'same-id', sourceDir: 'D:\\Chrome 数据' }
  bridge.configureDesktopHost(host({
    getBindings: async () => ({ BrowserChromeUserDataImport: name => {
      selectedName = name
      return Promise.resolve(result)
    } }),
  }))
  assert.equal(await bridge.invokeDesktop('BrowserChromeUserDataImport', '保留的名称'), result)
  assert.equal(selectedName, '保留的名称')
  await assert.rejects(bridge.invokeDesktop('ForceQuit'), /不支持桌面命令/)
  assert.throws(() => bridge.configureDesktopHost(host()), /启动前配置/)
})

test('binding loading failure preserves legacy fallback without inventing success', async () => {
  const bridge = await freshBridge()
  bridge.configureDesktopHost(host({ getBindings: async () => { throw Error('load failed') } }))
  assert.equal(await bridge.getDesktopBindings(), null)
  await assert.rejects(bridge.invokeDesktop('BackupImportPackage'), /不支持桌面命令/)
})

test('Wails events preserve variadic payloads and one page cannot unsubscribe another', async t => {
  const listeners = new Set()
  let releases = 0
  setWails(t, {}, {
    EventsOnMultiple(name, callback, count) {
      assert.equal(name, 'backup:import:progress')
      assert.equal(count, -1)
      listeners.add(callback)
      return () => { releases++; listeners.delete(callback) }
    },
  })
  const bridge = await freshBridge()
  const first = [], second = []
  const offFirst = bridge.EventsOn('backup:import:progress', (...args) => first.push(args))
  const offSecond = bridge.EventsOn('backup:import:progress', (...args) => second.push(args))
  const payload = { phase: 'writing', progress: 15 }
  for (const listener of listeners) listener(payload, 'extra')
  offFirst()
  offFirst()
  for (const listener of listeners) listener(payload, 'next')
  assert.deepEqual(first, [[payload, 'extra']])
  assert.deepEqual(second, [[payload, 'extra'], [payload, 'next']])
  assert.equal(second[0][0], payload)
  assert.equal(releases, 1)
  offSecond()
  assert.equal(releases, 2)
})

test('async listener registration is cleaned up even when the page has already unmounted', async () => {
  const bridge = await freshBridge()
  let resolveRegistration, dispatch, releases = 0, calls = 0
  bridge.configureDesktopHost(host({ listen(_event, callback) {
    dispatch = callback
    return new Promise(resolve => { resolveRegistration = resolve })
  } }))
  const off = bridge.EventsOn('app:request-close', () => { calls++ })
  off()
  dispatch('late')
  resolveRegistration(() => { releases++ })
  await tick()
  off()
  assert.equal(releases, 1)
  assert.equal(calls, 0)
})

test('window operations are passed through without changing URLs or environment', async t => {
  const calls = []
  const environment = { platform: 'windows', arch: 'amd64', buildType: 'production' }
  setWails(t, {}, {
    Environment: () => Promise.resolve(environment),
    WindowHide: () => calls.push('hide'),
    WindowMinimise: () => calls.push('minimise'),
    BrowserOpenURL: url => calls.push(url),
  })
  const bridge = await freshBridge()
  assert.equal(await bridge.Environment(), environment)
  bridge.WindowHide()
  bridge.WindowMinimise()
  bridge.BrowserOpenURL('https://example.org/中文?q=a%20b#fragment')
  assert.deepEqual(calls, ['hide', 'minimise', 'https://example.org/中文?q=a%20b#fragment'])
})

test('preview can subscribe without Wails while real commands still fail', async () => {
  const bridge = await freshBridge()
  bridge.EventsOn('app:request-close', () => assert.fail('no host'))()
  await assert.rejects(bridge.Environment(), /不支持桌面运行信息/)
  await assert.rejects(bridge.invokeDesktop('QuitAppOnly'))
})

test('operation logging cannot swallow errors, recurse or alter Wails methods', async t => {
  const entries = []
  const failure = { code: 'ORIGINAL' }
  const app = {
    GetAppConfig: () => Promise.resolve({ version: 'compatible' }),
    ForceQuit: () => Promise.reject(failure),
    FrontendOperationLog: (...args) => { entries.push(args); return Promise.reject(Error('log failed')) },
  }
  const original = app.GetAppConfig
  setWails(t, app)
  const bridge = await import('../src/desktop/index.ts')
  const { installDesktopOperationLogger } = await import('../src/utils/desktopOperationLogger.ts')
  installDesktopOperationLogger()
  installDesktopOperationLogger()
  assert.deepEqual(await bridge.invokeDesktop('GetAppConfig'), { version: 'compatible' })
  await assert.rejects(bridge.invokeDesktop('ForceQuit'), error => error === failure)
  await tick()
  assert.equal(app.GetAppConfig, original)
  assert.equal(entries.length, 2)
  assert.deepEqual(entries.map(entry => entry.slice(0, 3)), [
    ['debug', 'GetAppConfig', true], ['error', 'ForceQuit', false],
  ])
})

test('business modules cannot bypass the desktop boundary', async () => {
  const root = new URL('../src/', import.meta.url)
  async function inspect(directory) {
    for (const entry of await readdir(directory, { withFileTypes: true })) {
      if (['wailsjs', 'desktop'].includes(entry.name)) continue
      const file = new URL(entry.name + (entry.isDirectory() ? '/' : ''), directory)
      if (entry.isDirectory()) { await inspect(file); continue }
      if (!/\.tsx?$/.test(entry.name)) continue
      const source = await readFile(file, 'utf8')
      assert.doesNotMatch(source, /(?:from\s*|import\s*\()['"][^'"]*wailsjs\//, file.pathname)
      assert.doesNotMatch(source, /\(window as any\)\.(?:go|runtime)\b/, file.pathname)
    }
  }
  await inspect(root)
})
