import { getDesktopApp, observeDesktopCommands } from '../desktop/index.ts'

const MUTATION_METHOD_PATTERN = /(Create|Update|Delete|Save|Set|Import|Export|Install|Uninstall|Download|Start|Stop|Launch|Run|Clear|Move|Copy|Refresh|Sync|Validate|Toggle|Switch|Enable|Disable|Force|Quit|Open|Close|Apply|Bind|Unbind|Restart|Warmup|Probe|Check|Test)/
const SKIP_METHODS = new Set(['GetAppLogs', 'FrontendOperationLog'])
let installed = false

function stringifyError(error: unknown) {
  if (error instanceof Error) return error.message || String(error)
  if (typeof error === 'string') return error
  try { return JSON.stringify(error) } catch { return String(error) }
}

export function installDesktopOperationLogger() {
  if (installed) return
  installed = true
  observeDesktopCommands(({ method, success, durationMs, error }) => {
    if (SKIP_METHODS.has(method)) return
    const logger = getDesktopApp()?.FrontendOperationLog
    if (typeof logger !== 'function') return
    const level = success ? (MUTATION_METHOD_PATTERN.test(method) ? 'info' : 'debug') : 'error'
    const message = success ? '' : stringifyError(error)
    try {
      // Do not mutate host globals. Both synchronous and asynchronous logging
      // failures are diagnostic only and must not replace the operation result.
      void Promise.resolve(logger(level, method, success, Math.max(0, Math.round(durationMs)), String(message || '').slice(0, 1200))).catch(() => {})
    } catch { /* Logging must never break user operations. */ }
  })
}
