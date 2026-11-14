import { useEffect, useMemo, useRef, useState } from 'react'
import { Folder, FolderPlus, GripVertical, Plus, RefreshCw, RotateCcw, Settings2, Trash2, Upload } from 'lucide-react'
import { Button, Card, ConfirmModal, Input, Modal, Switch, toast } from '../../../shared/components'
import type { BrowserBookmark, BrowserProfile } from '../types'
import { fetchBookmarks, fetchBrowserProfiles, resetBookmarks, saveBookmarks, syncBookmarksToProfiles } from '../api'
import { bookmarkURLKey, parseBookmarkHTML } from '../utils/bookmarkImport'

interface BookmarkGroupView {
  folder: string
  entries: Array<{ item: BrowserBookmark; index: number }>
}

const fingerprintCheckBookmarkUrl = 'ant://fingerprint-check'
const isProtectedBookmark = (item: BrowserBookmark) => item.url.trim().toLowerCase() === fingerprintCheckBookmarkUrl

export function BookmarkSettingsPage() {
  const [items, setItems] = useState<BrowserBookmark[]>([])
  const [profiles, setProfiles] = useState<BrowserProfile[]>([])
  const [saving, setSaving] = useState(false)
  const [syncing, setSyncing] = useState(false)
  const [importing, setImporting] = useState(false)
  const [resetOpen, setResetOpen] = useState(false)
  const [syncOpen, setSyncOpen] = useState(false)
  const [dragIndex, setDragIndex] = useState<number | null>(null)
  const [profileSettingsIndex, setProfileSettingsIndex] = useState<number | null>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    fetchBookmarks().then(setItems).catch(error => toast.error(error?.message || '加载书签失败'))
    fetchBrowserProfiles().then(setProfiles).catch(error => toast.error(error?.message || '加载实例失败'))
  }, [])

  const profileSettingsBookmark = profileSettingsIndex === null ? null : items[profileSettingsIndex]

  const groups = useMemo<BookmarkGroupView[]>(() => {
    const result: BookmarkGroupView[] = []
    const byFolder = new Map<string, BookmarkGroupView>()
    items.forEach((item, index) => {
      const folder = (item.folder || '').trim()
      let group = byFolder.get(folder)
      if (!group) {
        group = { folder, entries: [] }
        byFolder.set(folder, group)
        result.push(group)
      }
      group.entries.push({ item, index })
    })
    return result
  }, [items])

  const handleChange = (index: number, field: keyof BrowserBookmark, value: string) => {
    if (field !== 'openOnStart' && isProtectedBookmark(items[index])) return
    setItems(prev => prev.map((item, itemIndex) => itemIndex === index ? { ...item, [field]: value } : item))
  }

  const handleAdd = (folder = '') => {
    setItems(prev => [...prev, { name: '', url: '', folder, openOnStart: false }])
  }

  const handleAddGroup = () => {
    const existing = new Set(groups.map(group => group.folder.toLowerCase()))
    let folder = '新分组'
    let suffix = 2
    while (existing.has(folder.toLowerCase())) {
      folder = `新分组 ${suffix}`
      suffix++
    }
    handleAdd(folder)
  }

  const handleRenameGroup = (folder: string, nextFolder: string) => {
    setItems(prev => prev.map(item => (item.folder || '').trim() === folder && !isProtectedBookmark(item) ? { ...item, folder: nextFolder } : item))
  }

  const handleDelete = (index: number) => {
    if (isProtectedBookmark(items[index])) return
    setItems(prev => prev.filter((_, itemIndex) => itemIndex !== index))
  }

  const handleOpenOnStartChange = (index: number, checked: boolean) => {
    setItems(prev => prev.map((item, itemIndex) => itemIndex === index ? { ...item, openOnStart: checked } : item))
  }

  const handleEnabledChange = (index: number, enabled: boolean) => {
    if (isProtectedBookmark(items[index])) return
    setItems(prev => prev.map((item, itemIndex) => itemIndex === index ? { ...item, disabled: !enabled } : item))
  }

  const handleProfileEnabledChange = (profileId: string, enabled: boolean) => {
    if (profileSettingsIndex === null) return
    setItems(prev => prev.map((item, itemIndex) => {
      if (itemIndex !== profileSettingsIndex) return item
      if (isProtectedBookmark(item)) return item
      const disabledProfileIds = new Set(item.disabledProfileIds || [])
      if (enabled) {
        disabledProfileIds.delete(profileId)
      } else {
        disabledProfileIds.add(profileId)
      }
      return { ...item, disabledProfileIds: Array.from(disabledProfileIds) }
    }))
  }

  const handleImport = async (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0]
    event.target.value = ''
    if (!file) return

    setImporting(true)
    try {
      const imported = parseBookmarkHTML(await file.text())
      if (imported.length === 0) {
        toast.error('未识别到书签，请选择浏览器导出的 HTML 文件')
        return
      }

      const existingURLs = new Set(items.map(item => bookmarkURLKey(item.url)).filter(Boolean))
      const added: BrowserBookmark[] = []
      let duplicateCount = 0
      for (const bookmark of imported) {
        const urlKey = bookmarkURLKey(bookmark.url)
        if (!urlKey || existingURLs.has(urlKey)) {
          duplicateCount++
          continue
        }
        existingURLs.add(urlKey)
        added.push(bookmark)
      }

      setItems(prev => [...prev, ...added])
      const groupCount = new Set(added.map(item => item.folder).filter(Boolean)).size
      const summary = [`已导入 ${added.length} 个书签`]
      if (groupCount > 0) summary.push(`保留 ${groupCount} 个分组`)
      if (duplicateCount > 0) summary.push(`跳过 ${duplicateCount} 个重复项`)
      toast.success(`${summary.join('，')}，请点击保存`)
    } catch (error: any) {
      toast.error(error?.message || '书签文件解析失败')
    } finally {
      setImporting(false)
    }
  }

  const handleSave = async () => {
    const valid = items.filter(item => item.name.trim() && item.url.trim())
    if (valid.length !== items.length) {
      toast.error('存在空的名称或 URL，请填写完整后保存')
      return
    }

    const seenURLs = new Set<string>()
    if (items.some(item => {
      const key = bookmarkURLKey(item.url)
      if (seenURLs.has(key)) return true
      seenURLs.add(key)
      return false
    })) {
      toast.error('存在重复 URL，请删除重复书签后保存')
      return
    }

    setSaving(true)
    try {
      await saveBookmarks(items)
      setItems(await fetchBookmarks())
      const result = await syncBookmarksToProfiles()
      const parts = ['书签已保存']
      if (result.synced > 0) parts.push(`已同步 ${result.synced} 个已有实例`)
      if (result.skipped > 0) parts.push(`跳过运行中 ${result.skipped} 个，停止后再同步`)
      if (result.failed > 0) parts.push(`失败 ${result.failed} 个`)
      const message = parts.join('，')
      if (result.failed > 0 || result.skipped > 0) {
        toast.warning(message)
      } else {
        toast.success(message)
      }
    } catch (error: any) {
      toast.error(error?.message || '书签保存失败')
    } finally {
      setSaving(false)
    }
  }

  const handleReset = async () => {
    try {
      await resetBookmarks()
      setItems(await fetchBookmarks())
      toast.success('已恢复默认书签')
    } catch (error: any) {
      toast.error(error?.message || '恢复默认书签失败')
    }
  }

  const handleSync = async () => {
    setSyncing(true)
    try {
      const result = await syncBookmarksToProfiles()
      const parts = [`已同步 ${result.synced} 个实例`]
      if (result.skipped > 0) parts.push(`跳过运行中 ${result.skipped} 个，停止后再同步`)
      if (result.failed > 0) parts.push(`失败 ${result.failed} 个`)
      const message = parts.join('，')
      if (result.failed > 0 || result.skipped > 0) {
        toast.warning(message)
      } else {
        toast.success(message)
      }
      setSyncOpen(false)
    } catch (error: any) {
      toast.error(error?.message || '同步失败')
    } finally {
      setSyncing(false)
    }
  }

  const handleDragStart = (index: number) => {
    if (isProtectedBookmark(items[index])) return
    setDragIndex(index)
  }
  const handleDragOver = (event: React.DragEvent, index: number) => {
    event.preventDefault()
    if (dragIndex === null || dragIndex === index) return
    const sourceFolder = (items[dragIndex]?.folder || '').trim()
    const targetFolder = (items[index]?.folder || '').trim()
    if (sourceFolder !== targetFolder || isProtectedBookmark(items[dragIndex]) || isProtectedBookmark(items[index])) return

    setItems(prev => {
      const next = [...prev]
      const [moved] = next.splice(dragIndex, 1)
      next.splice(index, 0, moved)
      return next
    })
    setDragIndex(index)
  }
  const handleDragEnd = () => setDragIndex(null)

  return (
    <div className="space-y-5 animate-fade-in">
      <input
        ref={fileInputRef}
        type="file"
        accept=".html,.htm,text/html"
        className="hidden"
        onChange={handleImport}
      />

      <div className="flex flex-col gap-3 xl:flex-row xl:items-center xl:justify-between">
        <div>
          <h1 className="text-xl font-semibold text-[var(--color-text-primary)]">默认书签</h1>
          <p className="text-sm text-[var(--color-text-muted)] mt-1">管理实例书签，可全局关闭或只对指定实例关闭</p>
        </div>
        <div className="flex flex-wrap gap-2">
          <Button variant="secondary" size="sm" onClick={() => fileInputRef.current?.click()} loading={importing}>
            <Upload className="w-4 h-4" />
            导入书签
          </Button>
          <Button variant="secondary" size="sm" onClick={() => setSyncOpen(true)} loading={syncing}>
            <RefreshCw className="w-4 h-4" />
            手动同步
          </Button>
          <Button variant="secondary" size="sm" onClick={() => setResetOpen(true)}>
            <RotateCcw className="w-4 h-4" />
            恢复默认
          </Button>
          <Button size="sm" onClick={handleSave} loading={saving}>保存</Button>
        </div>
      </div>

      <Card
        title={`书签列表（${items.length} 项）`}
        subtitle="分组使用“/”表示多级目录；组内可拖拽调整书签顺序"
      >
        <div className="space-y-4">
          {groups.map((group, groupIndex) => (
            <section
              key={groupIndex}
              className="overflow-hidden rounded-xl border border-[var(--color-border-default)] bg-[var(--color-bg-surface)]"
            >
              <div className="flex flex-col gap-2 border-b border-[var(--color-border-default)] bg-[var(--color-bg-muted)] px-3 py-2.5 sm:flex-row sm:items-center">
                <div className="flex min-w-0 flex-1 items-center gap-2">
                  <Folder className="h-4 w-4 shrink-0 text-[var(--color-text-muted)]" />
                  {group.folder ? (
                    <Input
                      value={group.folder}
                      onChange={event => handleRenameGroup(group.folder, event.target.value)}
                      placeholder="分组名称，如 工作/AI"
                      className="max-w-md"
                    />
                  ) : (
                    <span className="text-sm font-medium text-[var(--color-text-primary)]">书签栏（未分组）</span>
                  )}
                  <span className="shrink-0 text-xs text-[var(--color-text-muted)]">{group.entries.length} 项</span>
                </div>
                <Button variant="ghost" size="sm" onClick={() => handleAdd(group.folder)}>
                  <Plus className="h-4 w-4" />
                  添加书签
                </Button>
              </div>

              <div className="space-y-2 p-3">
                {group.entries.map(({ item, index }) => (
                  <div
                    key={index}
                    draggable={!isProtectedBookmark(item)}
                    onDragStart={() => handleDragStart(index)}
                    onDragOver={event => handleDragOver(event, index)}
                    onDragEnd={handleDragEnd}
                    className={`flex flex-col gap-2 rounded-xl p-2.5 shadow-[var(--shadow-sm)] transition-all duration-150 xl:flex-row xl:items-center ${
                      dragIndex === index
                        ? 'bg-[var(--color-accent-muted)] ring-1 ring-[var(--color-border-strong)]'
                        : item.disabled
                          ? 'bg-[var(--color-bg-muted)] opacity-65'
                          : 'bg-[var(--color-bg-muted)] hover:bg-[var(--color-bg-subtle)]'
                    }`}
                  >
                    <GripVertical className="hidden h-4 w-4 shrink-0 cursor-grab text-[var(--color-text-muted)] xl:block" />
                    <Input
                      value={item.name}
                      readOnly={isProtectedBookmark(item)}
                      onChange={event => handleChange(index, 'name', event.target.value)}
                      placeholder="名称，如 Google"
                      className="xl:w-44 xl:shrink-0"
                    />
                    <Input
                      value={item.url}
                      readOnly={isProtectedBookmark(item)}
                      onChange={event => handleChange(index, 'url', event.target.value)}
                      placeholder="https://..."
                      className="min-w-0 flex-1"
                    />
                    <div className="flex flex-wrap items-center justify-between gap-2 sm:justify-start xl:justify-end">
                      <label className="flex items-center gap-1.5 px-2 text-xs text-[var(--color-text-secondary)] whitespace-nowrap select-none">
                        <Switch
                          checked={!item.disabled}
                          disabled={isProtectedBookmark(item)}
                          onChange={enabled => handleEnabledChange(index, enabled)}
                        />
                        {!item.disabled ? '已启用' : '已关闭'}
                      </label>
                      <label className="flex items-center gap-1.5 px-2 text-xs text-[var(--color-text-secondary)] whitespace-nowrap select-none">
                        <input
                          type="checkbox"
                          checked={Boolean(item.openOnStart)}
                          onChange={event => handleOpenOnStartChange(index, event.target.checked)}
                          disabled={Boolean(item.disabled)}
                          className="h-4 w-4 rounded border-[var(--color-border-default)] accent-[var(--color-accent)]"
                        />
                        启动打开
                      </label>
                      <Button
                        type="button"
                        variant="ghost"
                        size="sm"
                        onClick={() => setProfileSettingsIndex(index)}
                        disabled={isProtectedBookmark(item)}
                        title="设置此书签在哪些实例中关闭"
                      >
                        <Settings2 className="h-4 w-4" />
                        关闭实例{(item.disabledProfileIds || []).length > 0 ? ` ${(item.disabledProfileIds || []).length}` : ''}
                      </Button>
                      <button
                        type="button"
                        aria-label={`删除书签 ${item.name || index + 1}`}
                        disabled={isProtectedBookmark(item)}
                        onClick={() => handleDelete(index)}
                        className="shrink-0 rounded p-1.5 text-[var(--color-text-muted)] transition-colors hover:bg-red-50 hover:text-red-500"
                      >
                        <Trash2 className="h-4 w-4" />
                      </button>
                    </div>
                  </div>
                ))}
              </div>
            </section>
          ))}

          {items.length === 0 && (
            <p className="py-8 text-center text-sm text-[var(--color-text-muted)]">
              暂无书签，可添加书签、创建分组或导入浏览器书签文件
            </p>
          )}
        </div>

        <div className="mt-4 grid gap-2 sm:grid-cols-2">
          <button
            type="button"
            onClick={() => handleAdd('')}
            className="flex items-center justify-center gap-2 rounded-xl bg-[var(--color-bg-muted)] py-2.5 text-sm text-[var(--color-text-primary)] shadow-[var(--shadow-sm)] transition-colors hover:bg-[var(--color-bg-subtle)]"
          >
            <Plus className="h-4 w-4" />
            添加未分组书签
          </button>
          <button
            type="button"
            onClick={handleAddGroup}
            className="flex items-center justify-center gap-2 rounded-xl bg-[var(--color-bg-muted)] py-2.5 text-sm text-[var(--color-text-primary)] shadow-[var(--shadow-sm)] transition-colors hover:bg-[var(--color-bg-subtle)]"
          >
            <FolderPlus className="h-4 w-4" />
            添加分组
          </button>
        </div>
      </Card>

      <ConfirmModal
        open={resetOpen}
        onClose={() => setResetOpen(false)}
        onConfirm={handleReset}
        title="恢复默认书签"
        content="将清除当前所有自定义书签和分组，恢复为内置默认列表。确定继续？"
        confirmText="确定恢复"
        danger
      />

      <ConfirmModal
        open={syncOpen}
        onClose={() => setSyncOpen(false)}
        onConfirm={handleSync}
        title="手动同步已有实例"
        content="会追加缺失的默认书签，并移除已关闭且由本应用写入的书签；不会删除用户手工创建的书签。运行中的实例会跳过。"
        confirmText="开始同步"
      />

      <Modal
        open={profileSettingsIndex !== null}
        onClose={() => setProfileSettingsIndex(null)}
        title={`单实例关闭${profileSettingsBookmark?.name ? ` · ${profileSettingsBookmark.name}` : ''}`}
        width="520px"
        footer={<Button onClick={() => setProfileSettingsIndex(null)}>完成</Button>}
      >
        <div className="space-y-3">
          <p className="text-sm text-[var(--color-text-secondary)]">
            关闭后，该书签不会写入所选实例；保存书签后对已停止实例生效。
          </p>
          {profileSettingsBookmark?.disabled && (
            <p className="rounded-lg bg-[var(--color-bg-muted)] px-3 py-2 text-xs text-[var(--color-text-muted)]">
              当前书签已全局关闭，下面的单实例设置会保留，但暂时不会显示在任何实例中。
            </p>
          )}
          <div className="divide-y divide-[var(--color-border-default)] border-y border-[var(--color-border-default)]">
            {profiles.map(profile => {
              const enabled = !(profileSettingsBookmark?.disabledProfileIds || []).includes(profile.profileId)
              return (
                <div key={profile.profileId} className="flex items-center justify-between gap-4 py-3">
                  <div className="min-w-0">
                    <p className="truncate text-sm font-medium text-[var(--color-text-primary)]">{profile.profileName || profile.profileId}</p>
                    <p className="truncate text-xs text-[var(--color-text-muted)]">{profile.launchCode || profile.profileId}</p>
                  </div>
                  <label className="flex shrink-0 items-center gap-2 text-sm text-[var(--color-text-secondary)]">
                    <Switch checked={enabled} onChange={checked => handleProfileEnabledChange(profile.profileId, checked)} />
                    {enabled ? '显示' : '关闭'}
                  </label>
                </div>
              )
            })}
            {profiles.length === 0 && (
              <p className="py-8 text-center text-sm text-[var(--color-text-muted)]">暂无可配置实例</p>
            )}
          </div>
        </div>
      </Modal>
    </div>
  )
}
