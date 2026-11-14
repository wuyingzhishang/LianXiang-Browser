package browser

import (
	"encoding/json"
	"fmt"
	"lianxiang-browser/backend/internal/config"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// chromiumEpoch 是 Chrome FILETIME 的起始时间（1601-01-01 UTC）
var chromiumEpoch = time.Date(1601, 1, 1, 0, 0, 0, 0, time.UTC)

func toChromiumTime(t time.Time) string {
	return fmt.Sprintf("%d", t.Sub(chromiumEpoch).Microseconds())
}

// EnsureDefaultBookmarks 将默认书签合并到书签栏（已存在的 URL 不重复添加）
func EnsureDefaultBookmarks(userDataDir string, bookmarks []config.BrowserBookmark) error {
	return SyncDefaultBookmarks(userDataDir, bookmarks, bookmarks)
}

// SyncDefaultBookmarks adds enabled managed bookmarks and removes only managed
// nodes that are disabled. User-created nodes are left untouched.
func SyncDefaultBookmarks(userDataDir string, managedBookmarks, enabledBookmarks []config.BrowserBookmark) error {
	if len(managedBookmarks) == 0 && len(enabledBookmarks) == 0 {
		return nil
	}

	profileDir := filepath.Join(userDataDir, "Default")
	if err := os.MkdirAll(profileDir, 0755); err != nil {
		return fmt.Errorf("创建 profile 目录失败: %w", err)
	}

	bookmarksPath := filepath.Join(profileDir, "Bookmarks")

	// 尝试读取已有书签文件
	var root map[string]interface{}
	if data, err := os.ReadFile(bookmarksPath); err == nil {
		_ = json.Unmarshal(data, &root)
	}

	now := toChromiumTime(time.Now())
	if root == nil && len(enabledBookmarks) == 0 {
		return nil
	}

	// 初始化空结构
	if root == nil {
		root = newEmptyBookmarkRoot(now)
	}

	managedURLs := make(map[string]string, len(managedBookmarks))
	enabledURLs := make(map[string]bool, len(enabledBookmarks))
	for _, bookmark := range managedBookmarks {
		url := strings.TrimSpace(bookmark.URL)
		if url != "" {
			managedURLs[NormalizeBookmarkURL(url)] = bookmarkGUID(url)
		}
	}
	for _, bookmark := range enabledBookmarks {
		url := strings.TrimSpace(bookmark.URL)
		if url != "" {
			enabledURLs[NormalizeBookmarkURL(url)] = true
		}
	}
	removed := removeDisabledManagedBookmarks(root, managedURLs, enabledURLs)

	// 取出 bookmark_bar children，按整个书签树收集已有 URL 集合
	barChildren := extractBarChildren(root)
	existingURLs := collectRootURLs(root)

	// 计算当前最大 id，用于分配新 id
	maxID := findMaxID(root)

	// 把不存在的默认书签追加进去
	added := false
	for _, b := range enabledBookmarks {
		name := strings.TrimSpace(b.Name)
		url := strings.TrimSpace(b.URL)
		if name == "" || url == "" {
			continue
		}
		urlKey := NormalizeBookmarkURL(url)
		if existingURLs[urlKey] {
			continue
		}
		bookmarkNode := map[string]interface{}{
			"date_added":     now,
			"date_last_used": "0",
			"guid":           bookmarkGUID(url),
			"meta_info":      map[string]string{"power_bookmark_meta": ""},
			"name":           name,
			"type":           "url",
			"url":            url,
		}
		maxID++
		bookmarkNode["id"] = fmt.Sprintf("%d", maxID)
		barChildren, maxID = appendBookmarkNode(barChildren, bookmarkFolderParts(b.Folder), bookmarkNode, now, maxID, "")
		existingURLs[urlKey] = true
		added = true
	}

	if !added && !removed {
		return nil
	}

	// 写回
	roots := root["roots"].(map[string]interface{})
	bar := roots["bookmark_bar"].(map[string]interface{})
	bar["children"] = barChildren
	bar["date_modified"] = now
	roots["bookmark_bar"] = bar
	root["roots"] = roots

	out, err := json.MarshalIndent(root, "", "   ")
	if err != nil {
		return fmt.Errorf("序列化书签失败: %w", err)
	}
	return os.WriteFile(bookmarksPath, out, 0644)
}

func removeDisabledManagedBookmarks(root map[string]interface{}, managedURLs map[string]string, enabledURLs map[string]bool) bool {
	roots, ok := root["roots"].(map[string]interface{})
	if !ok {
		return false
	}
	changed := false
	for key, rawRoot := range roots {
		folder, ok := rawRoot.(map[string]interface{})
		if !ok {
			continue
		}
		children, ok := folder["children"].([]interface{})
		if !ok {
			continue
		}
		updated, removed := removeDisabledManagedNodes(children, managedURLs, enabledURLs)
		if removed {
			folder["children"] = updated
			roots[key] = folder
			changed = true
		}
	}
	root["roots"] = roots
	return changed
}

func removeDisabledManagedNodes(nodes []interface{}, managedURLs map[string]string, enabledURLs map[string]bool) ([]interface{}, bool) {
	updated := make([]interface{}, 0, len(nodes))
	changed := false
	for _, rawNode := range nodes {
		node, ok := rawNode.(map[string]interface{})
		if !ok {
			updated = append(updated, rawNode)
			continue
		}
		if node["type"] == "url" {
			url, _ := node["url"].(string)
			urlKey := NormalizeBookmarkURL(url)
			expectedGUID, managed := managedURLs[urlKey]
			guid, _ := node["guid"].(string)
			if managed && !enabledURLs[urlKey] && strings.EqualFold(guid, expectedGUID) {
				changed = true
				continue
			}
		}
		if node["type"] == "folder" {
			children, _ := node["children"].([]interface{})
			nextChildren, removed := removeDisabledManagedNodes(children, managedURLs, enabledURLs)
			if removed {
				node["children"] = nextChildren
				changed = true
			}
		}
		updated = append(updated, node)
	}
	return updated, changed
}

// ReplaceBookmarkURL 将已有书签中的 oldURL 更新为 newURL，用于修复运行时动态书签地址。
func ReplaceBookmarkURL(userDataDir string, oldURL string, newURL string) (bool, error) {
	oldURL = strings.TrimSpace(oldURL)
	newURL = strings.TrimSpace(newURL)
	if oldURL == "" || newURL == "" || strings.EqualFold(oldURL, newURL) {
		return false, nil
	}
	return replaceBookmarkURL(userDataDir, newURL, func(node map[string]interface{}) bool {
		urlValue, _ := node["url"].(string)
		return strings.EqualFold(strings.TrimSpace(urlValue), oldURL)
	})
}

func replaceBookmarkURL(userDataDir string, newURL string, match func(map[string]interface{}) bool) (bool, error) {
	bookmarksPath := filepath.Join(userDataDir, "Default", "Bookmarks")
	data, err := os.ReadFile(bookmarksPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	var root map[string]interface{}
	if err := json.Unmarshal(data, &root); err != nil {
		return false, fmt.Errorf("解析书签失败: %w", err)
	}
	roots, ok := root["roots"].(map[string]interface{})
	if !ok {
		return false, nil
	}
	changed := false
	for _, item := range roots {
		folder, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if children, ok := folder["children"].([]interface{}); ok {
			if replaceBookmarkURLInNodes(children, newURL, match) {
				folder["date_modified"] = toChromiumTime(time.Now())
				changed = true
			}
		}
	}
	if !changed {
		return false, nil
	}
	out, err := json.MarshalIndent(root, "", "   ")
	if err != nil {
		return false, fmt.Errorf("序列化书签失败: %w", err)
	}
	return true, os.WriteFile(bookmarksPath, out, 0644)
}

func replaceBookmarkURLInNodes(nodes []interface{}, newURL string, match func(map[string]interface{}) bool) bool {
	changed := false
	for _, item := range nodes {
		node, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if node["type"] == "url" {
			if match(node) {
				node["url"] = newURL
				node["guid"] = bookmarkGUID(newURL)
				changed = true
			}
			continue
		}
		if node["type"] == "folder" {
			if children, ok := node["children"].([]interface{}); ok {
				if replaceBookmarkURLInNodes(children, newURL, match) {
					node["date_modified"] = toChromiumTime(time.Now())
					changed = true
				}
			}
		}
	}
	return changed
}

// newEmptyBookmarkRoot 构建一个空的书签根结构
func newEmptyBookmarkRoot(now string) map[string]interface{} {
	return map[string]interface{}{
		"checksum": "",
		"version":  1,
		"roots": map[string]interface{}{
			"bookmark_bar": map[string]interface{}{
				"children":       []interface{}{},
				"date_added":     now,
				"date_last_used": "0",
				"date_modified":  now,
				"guid":           "0bc5d13f-2cba-5d74-951f-3f233fe6c908",
				"id":             "1",
				"name":           "书签栏",
				"type":           "folder",
			},
			"other": map[string]interface{}{
				"children":       []interface{}{},
				"date_added":     now,
				"date_last_used": "0",
				"date_modified":  "0",
				"guid":           "82b081ec-3dd3-529c-8475-ab6c344590dd",
				"id":             "2",
				"name":           "其他书签",
				"type":           "folder",
			},
			"synced": map[string]interface{}{
				"children":       []interface{}{},
				"date_added":     now,
				"date_last_used": "0",
				"date_modified":  "0",
				"guid":           "4cf2e351-0e85-532b-bb37-df045d8f8d0f",
				"id":             "3",
				"name":           "移动设备书签",
				"type":           "folder",
			},
		},
	}
}

// extractBarChildren 从根结构中提取书签栏 children
func extractBarChildren(root map[string]interface{}) []interface{} {
	var children []interface{}

	roots, ok := root["roots"].(map[string]interface{})
	if !ok {
		root["roots"] = map[string]interface{}{
			"bookmark_bar": map[string]interface{}{
				"children": []interface{}{},
				"type":     "folder",
				"name":     "书签栏",
			},
		}
		return children
	}

	bar, ok := roots["bookmark_bar"].(map[string]interface{})
	if !ok {
		roots["bookmark_bar"] = map[string]interface{}{
			"children": []interface{}{},
			"type":     "folder",
			"name":     "书签栏",
		}
		root["roots"] = roots
		return children
	}

	if c, ok := bar["children"].([]interface{}); ok {
		children = c
	}
	return children
}

func collectRootURLs(root map[string]interface{}) map[string]bool {
	existing := map[string]bool{}
	roots, ok := root["roots"].(map[string]interface{})
	if !ok {
		return existing
	}
	for _, item := range roots {
		folder, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if children, ok := folder["children"].([]interface{}); ok {
			collectURLs(children, existing)
		}
	}
	return existing
}

// collectURLs 递归收集所有书签 URL
func collectURLs(nodes []interface{}, out map[string]bool) {
	for _, n := range nodes {
		node, ok := n.(map[string]interface{})
		if !ok {
			continue
		}
		if node["type"] == "url" {
			if u, ok := node["url"].(string); ok {
				out[NormalizeBookmarkURL(u)] = true
			}
		} else if node["type"] == "folder" {
			if sub, ok := node["children"].([]interface{}); ok {
				collectURLs(sub, out)
			}
		}
	}
}

func bookmarkFolderParts(folder string) []string {
	parts := strings.FieldsFunc(folder, func(r rune) bool {
		return r == '/' || r == '\\'
	})
	clean := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			clean = append(clean, part)
		}
	}
	return clean
}

func appendBookmarkNode(children []interface{}, folders []string, bookmarkNode map[string]interface{}, now string, maxID int, parentPath string) ([]interface{}, int) {
	if len(folders) == 0 {
		return append(children, bookmarkNode), maxID
	}
	folderName := folders[0]
	folderPath := folderName
	if parentPath != "" {
		folderPath = parentPath + "/" + folderName
	}
	for index, rawChild := range children {
		folder, ok := rawChild.(map[string]interface{})
		if !ok || folder["type"] != "folder" {
			continue
		}
		name, _ := folder["name"].(string)
		if !strings.EqualFold(strings.TrimSpace(name), folderName) {
			continue
		}
		subChildren, _ := folder["children"].([]interface{})
		updated, nextMaxID := appendBookmarkNode(subChildren, folders[1:], bookmarkNode, now, maxID, folderPath)
		folder["children"] = updated
		folder["date_modified"] = now
		children[index] = folder
		return children, nextMaxID
	}
	maxID++
	newFolder := map[string]interface{}{
		"children":       []interface{}{},
		"date_added":     now,
		"date_last_used": "0",
		"date_modified":  now,
		"guid":           bookmarkGUID("folder:" + folderPath),
		"id":             fmt.Sprintf("%d", maxID),
		"name":           folderName,
		"type":           "folder",
	}
	updated, nextMaxID := appendBookmarkNode([]interface{}{}, folders[1:], bookmarkNode, now, maxID, folderPath)
	newFolder["children"] = updated
	return append(children, newFolder), nextMaxID
}

// findMaxID 遍历整个书签树找到最大数字 id。
func findMaxID(root map[string]interface{}) int {
	max := 0
	roots, ok := root["roots"].(map[string]interface{})
	if !ok {
		return max
	}
	for _, v := range roots {
		if folder, ok := v.(map[string]interface{}); ok {
			scanMaxID(folder, &max)
		}
	}
	return max
}

func scanMaxID(node map[string]interface{}, max *int) {
	if idStr, ok := node["id"].(string); ok {
		var n int
		fmt.Sscanf(idStr, "%d", &n)
		if n > *max {
			*max = n
		}
	}
	if children, ok := node["children"].([]interface{}); ok {
		for _, c := range children {
			if child, ok := c.(map[string]interface{}); ok {
				scanMaxID(child, max)
			}
		}
	}
}

// bookmarkGUID 根据 URL 生成稳定伪 GUID
func bookmarkGUID(url string) string {
	h := uint64(14695981039346656037)
	for _, c := range url {
		h ^= uint64(c)
		h *= 1099511628211
	}
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		h&0xffffffff, (h>>32)&0xffff,
		(h>>48)&0x0fff|0x4000,
		(h>>16)&0x3fff|0x8000,
		h&0xffffffffffff,
	)
}
