// Keep the existing DTOs and positional command arguments during the host migration.
export type DesktopBindings = typeof import('../wailsjs/go/main/App')
export type DesktopCommandMap = Record<string, (...args: any[]) => any>
export type Unsubscribe = () => void
export type DesktopEventCallback = (...data: any[]) => void

export interface DesktopEnvironment {
  buildType: string
  platform: string
  arch: string
}

// A replacement host must implement this contract before React mounts.
// This boundary does not serialize, normalize or persist application data.
export interface DesktopHost {
  readonly id: string
  getBindings(): Promise<Partial<DesktopBindings> | null>
  getCommands(): DesktopCommandMap | null
  listen(event: string, callback: DesktopEventCallback): Unsubscribe | Promise<Unsubscribe>
  environment(): Promise<DesktopEnvironment>
  hide(): void
  minimise(): void
  openURL(url: string): void
}

export interface DesktopCommandResult {
  method: string
  success: boolean
  durationMs: number
  error?: unknown
}
