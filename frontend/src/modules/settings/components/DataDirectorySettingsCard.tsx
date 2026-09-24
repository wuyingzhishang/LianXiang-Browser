import { useEffect, useState } from 'react'
import { FolderOpen, HardDrive, RefreshCw } from 'lucide-react'
import { Button, Card, FormItem, Input, Switch, toast } from '../../../shared/components'
import { fetchDataDirectoryInfo, selectDataDirectory, setCustomDataDirectory, migrateDataDirectory } from '../api'
import type { DataDirectoryInfo, DataDirectoryMigrateOptions } from '../api'

function getErrorMessage(error: unknown, fallback: string) {
  if (error instanceof Error && error.message) return error.message
  if (typeof error === 'string' && error.trim()) return error
  if (error && typeof error === 'object' && 'message' in error) {
    const message = (error as { message?: unknown }).message
    if (typeof message === 'string' && message.trim()) return message
  }
  return fallback
}

function sameDirectoryPath(left: string | undefined, right: string | undefined) {
  const normalize = (value: string | undefined) =>
    (value || '').trim().replace(/[\\/]+$/, '').replace(/\//g, '\\').toLowerCase()
  return normalize(left) === normalize(right)
}

interface DataDirectorySettingsCardProps {
  onDataDirChange?: () => void
}

export function DataDirectorySettingsCard({ onDataDirChange }: DataDirectorySettingsCardProps) {
  const [info, setInfo] = useState<DataDirectoryInfo | null>(null)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [migrating, setMigrating] = useState(false)
  const [showMigrationOptions, setShowMigrationOptions] = useState(false)
  const [customDirInput, setCustomDirInput] = useState('')
  const [useCustom, setUseCustom] = useState(false)

  useEffect(() => {
    loadInfo()
  }, [])

  useEffect(() => {
    if (info) {
      setCustomDirInput(info.customDataDir || '')
      setUseCustom(info.useCustomDataDir)
    }
  }, [info])

  const loadInfo = async () => {
    setLoading(true)
    try {
      const data = await fetchDataDirectoryInfo()
      setInfo(data)
    } catch (error: any) {
      toast.error(getErrorMessage(error, '加载数据目录信息失败'))
    } finally {
      setLoading(false)
    }
  }

  const handleSelectDirectory = async () => {
    try {
      const result = await selectDataDirectory()
      if (result.selected) {
        setCustomDirInput(result.directory)
      }
    } catch (error: any) {
      toast.error(getErrorMessage(error, '选择目录失败'))
    }
  }

  const handleSave = async () => {
    if (useCustom && !customDirInput.trim()) {
      toast.error('请选择自定义数据目录')
      return
    }

    setSaving(true)
    try {
      const updated = await setCustomDataDirectory(customDirInput, useCustom)
      setInfo(updated)
      setCustomDirInput(updated.customDataDir || '')

      const redirected = useCustom && !sameDirectoryPath(customDirInput, updated.customDataDir)
      if (redirected) {
        toast.warning(`安装目录受 Windows 保护，普通启动无法写入，已改用：${updated.customDataDir}`)
      } else {
        toast.success('数据目录配置已保存')
      }

      if (useCustom && !sameDirectoryPath(updated.customDataDir, updated.currentDataDir)) {
        setShowMigrationOptions(true)
      } else {
        setShowMigrationOptions(false)
      }

      onDataDirChange?.()
    } catch (error: any) {
      toast.error(getErrorMessage(error, '保存数据目录配置失败'))
    } finally {
      setSaving(false)
    }
  }

  const handleMigrate = async () => {
    if (!info || !customDirInput) {
      return
    }

    setMigrating(true)
    try {
      const options: DataDirectoryMigrateOptions = {
        sourceDir: info.currentDataDir,
        targetDir: customDirInput,
        migrateData: true,
        migrateChrome: true,
        migrateConfig: true,
      }

      const result = await migrateDataDirectory(options)

      if (result.success) {
        toast.success('数据迁移成功，请重启应用以使用新数据目录')
        setShowMigrationOptions(false)
        await loadInfo()
      } else {
        toast.error(result.error || '数据迁移失败')
      }
    } catch (error: any) {
      toast.error(getErrorMessage(error, '数据迁移失败'))
    } finally {
      setMigrating(false)
    }
  }

  if (loading) {
    return (
      <Card title="数据目录设置">
        <div className="flex items-center justify-center py-8">
          <RefreshCw className="w-6 h-6 animate-spin text-[var(--color-text-muted)]" />
        </div>
      </Card>
    )
  }

  return (
    <>
      <Card title="数据目录设置">
        <div className="space-y-4">
          <div className="bg-[var(--color-bg-secondary)] rounded-lg p-4 space-y-2">
            <div className="flex items-center gap-2 text-sm">
              <HardDrive className="w-4 h-4 text-[var(--color-text-muted)]" />
              <span className="text-[var(--color-text-muted)]">当前数据目录：</span>
            </div>
            <div className="text-sm font-mono text-[var(--color-text-primary)] break-all pl-6">
              {info?.currentDataDir || '未知'}
            </div>
          </div>

          <div className="h-px bg-[var(--color-border-muted)]" />

          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium text-[var(--color-text-primary)]">使用自定义数据目录</p>
              <p className="text-xs text-[var(--color-text-muted)] mt-0.5">
                启用后可以自由选择数据存储位置，重启后生效
              </p>
            </div>
            <Switch checked={useCustom} onChange={setUseCustom} />
          </div>

          {useCustom && (
            <div className="space-y-3 pl-4 border-l-2 border-[var(--color-border-muted)]">
              <FormItem label="自定义数据目录">
                <div className="flex gap-2">
                  <Input
                    value={customDirInput}
                    onChange={e => setCustomDirInput(e.target.value)}
                    placeholder="请选择数据目录"
                    className="flex-1"
                  />
                  <Button onClick={handleSelectDirectory} variant="secondary" size="sm">
                    <FolderOpen className="w-4 h-4 mr-1" />
                    浏览
                  </Button>
                </div>
              </FormItem>

              {customDirInput && (
                <div className="text-xs space-y-1">
                  <div className="flex items-center gap-2">
                    <span className="text-[var(--color-text-muted)]">状态：</span>
                    {info?.isCustomDirValid ? (
                      <span className="text-green-600">✓ 目录有效</span>
                    ) : (
                      <span className="text-orange-600">! 目录不存在或无法访问</span>
                    )}
                  </div>
                </div>
              )}
            </div>
          )}

          <div className="flex justify-end gap-2 pt-2">
            <Button onClick={loadInfo} variant="secondary" size="sm" disabled={loading}>
              <RefreshCw className="w-4 h-4 mr-1" />
              刷新
            </Button>
            <Button onClick={handleSave} loading={saving} size="sm">
              保存配置
            </Button>
          </div>
        </div>
      </Card>

      {showMigrationOptions && (
        <Card title="数据迁移">
          <div className="space-y-4">
            <div className="bg-blue-50 dark:bg-blue-900/20 rounded-lg p-4">
              <p className="text-sm text-blue-900 dark:text-blue-100">
                检测到数据目录已更改，是否需要将现有数据迁移到新目录？
              </p>
            </div>

            <div className="space-y-2">
              <div className="text-sm">
                <span className="text-[var(--color-text-muted)]">源目录：</span>
                <span className="font-mono ml-2">{info?.currentDataDir}</span>
              </div>
              <div className="text-sm">
                <span className="text-[var(--color-text-muted)]">目标目录：</span>
                <span className="font-mono ml-2">{customDirInput}</span>
              </div>
            </div>

            <div className="bg-yellow-50 dark:bg-yellow-900/20 rounded-lg p-4">
              <p className="text-sm text-yellow-900 dark:text-yellow-100">
                ⚠️ 迁移将复制 data、chrome 目录和配置文件到新位置。完成后请重启应用。
              </p>
            </div>

            <div className="flex justify-end gap-2">
                <Button onClick={() => setShowMigrationOptions(false)} variant="secondary" size="sm">
                稍后迁移
              </Button>
              <Button onClick={handleMigrate} loading={migrating} size="sm">
                立即迁移
              </Button>
            </div>
          </div>
        </Card>
      )}
    </>
  )
}
