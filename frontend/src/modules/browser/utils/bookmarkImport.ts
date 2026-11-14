import type { BrowserBookmark } from '../types'

// Match backend URL keys: only the scheme and host are case-insensitive.
export function bookmarkURLKey(rawURL: string): string {
  return rawURL.trim()
    .replace(/^([a-z][a-z0-9+.-]*):/i, scheme => scheme.toLowerCase())
    .replace(/^([a-z][a-z0-9+.-]*:\/\/(?:[^/@]*@)?)([^/?#]*)/i,
      (_, prefix: string, host: string) => prefix + host.toLowerCase())
}

const BOOKMARK_BAR_NAMES = new Set([
  'bookmarks bar',
  'bookmarks toolbar',
  'favorites bar',
  '书签栏',
  '书签工具栏',
  '收藏夹栏',
])

function directChild(element: Element, tagName: string): Element | undefined {
  const expected = tagName.toUpperCase()
  return Array.from(element.children).find(child => child.tagName === expected)
}

function nestedList(entry: Element): Element | undefined {
  const childList = directChild(entry, 'DL')
  if (childList) return childList

  let sibling = entry.nextElementSibling
  while (sibling && sibling.tagName !== 'DT') {
    if (sibling.tagName === 'DL') return sibling
    sibling = sibling.nextElementSibling
  }
  return undefined
}

function normalizeFolder(parts: string[]): string {
  return parts.map(part => part.trim()).filter(Boolean).join('/')
}

/** 解析 Chrome、Edge、Firefox 使用的 Netscape Bookmark HTML。 */
export function parseBookmarkHTML(html: string): BrowserBookmark[] {
  const document = new DOMParser().parseFromString(html, 'text/html')
  const rootList = document.querySelector('dl')
  if (!rootList) return []

  const bookmarks: BrowserBookmark[] = []

  const walk = (list: Element, folders: string[], rootLevel: boolean) => {
    const entries = Array.from(list.children).filter(child => child.tagName === 'DT')
    for (const entry of entries) {
      const anchor = directChild(entry, 'A') as HTMLAnchorElement | undefined
      if (anchor) {
        const url = (anchor.getAttribute('href') || '').trim()
        if (!url) continue
        bookmarks.push({
          name: (anchor.textContent || url).trim() || url,
          url,
          folder: normalizeFolder(folders),
          openOnStart: false,
        })
        continue
      }

      const heading = directChild(entry, 'H3')
      const childList = nestedList(entry)
      if (!heading || !childList) continue

      const folderName = (heading.textContent || '').trim()
      const isBookmarkBarRoot = rootLevel && BOOKMARK_BAR_NAMES.has(folderName.toLowerCase())
      const nextFolders = folderName && !isBookmarkBarRoot ? [...folders, folderName] : folders
      walk(childList, nextFolders, false)
    }
  }

  walk(rootList, [], true)
  return bookmarks
}
