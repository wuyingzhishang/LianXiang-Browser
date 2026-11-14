import { useEffect, useState } from 'react'
import { FolderInput, PackageOpen } from 'lucide-react'

import { Button, FormItem, Input, Modal } from '../../../shared/components'

interface BrowserImportModalProps {
  open: boolean
  busy: boolean
  onClose: () => void
  onImportChrome: (profileName: string) => void
  onImportPackage: () => void
}

export function BrowserImportModal({
  open,
  busy,
  onClose,
  onImportChrome,
  onImportPackage,
}: BrowserImportModalProps) {
  const [profileName, setProfileName] = useState('')

  useEffect(() => {
    if (open) setProfileName('')
  }, [open])

  return (
    <Modal open={open} onClose={onClose} title="导入实例" width="620px" closable={!busy}>
      <div className="space-y-4">
        <section className="rounded-xl border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] p-4">
          <div className="flex items-start gap-3">
            <div className="mt-0.5 rounded-lg bg-[var(--color-accent-muted)] p-2 text-[var(--color-accent)]">
              <FolderInput className="h-5 w-5" />
            </div>
            <div className="min-w-0 flex-1">
              <h4 className="font-semibold text-[var(--color-text-primary)]">Chrome 用户数据目录</h4>
              <p className="mt-1 text-sm leading-6 text-[var(--color-text-muted)]">
                直接复制由 <code className="rounded bg-[var(--color-bg-muted)] px-1.5 py-0.5">--user-data-dir</code> 指定的完整目录，保留书签、扩展、历史记录和可用的登录数据。
              </p>
              <FormItem label="新实例名称" hint="留空时按目录名生成" className="mt-4">
                <Input
                  value={profileName}
                  onChange={(event) => setProfileName(event.target.value)}
                  placeholder="例如：本地 Chrome 5"
                  maxLength={80}
                  disabled={busy}
                />
              </FormItem>
              <div className="mt-3 rounded-lg bg-[var(--color-bg-muted)] px-3 py-2 text-xs leading-5 text-[var(--color-text-secondary)]">
                导入前请关闭使用该目录的全部 Chrome 窗口。请选择包含 Local State 和 Default 或 Profile 子目录的用户数据根目录。
              </div>
              <div className="mt-4 flex justify-end">
                <Button onClick={() => onImportChrome(profileName.trim())} loading={busy}>
                  选择目录并导入
                </Button>
              </div>
            </div>
          </div>
        </section>

        <section className="flex items-center gap-3 rounded-xl border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] p-4">
          <div className="rounded-lg bg-[var(--color-bg-muted)] p-2 text-[var(--color-text-secondary)]">
            <PackageOpen className="h-5 w-5" />
          </div>
          <div className="min-w-0 flex-1">
            <h4 className="font-medium text-[var(--color-text-primary)]">链享实例包</h4>
            <p className="mt-1 text-sm text-[var(--color-text-muted)]">导入由链享浏览器导出的实例 ZIP。</p>
          </div>
          <Button variant="secondary" onClick={onImportPackage} loading={busy}>
            选择 ZIP
          </Button>
        </section>
      </div>
    </Modal>
  )
}
