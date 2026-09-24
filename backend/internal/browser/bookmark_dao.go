package browser

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"lianxiang-browser/backend/internal/config"
)

// BookmarkDAO 默认书签持久化接口
type BookmarkDAO interface {
	List() ([]config.BrowserBookmark, error)
	ReplaceAll(bookmarks []config.BrowserBookmark) error
}

// SQLiteBookmarkDAO 基于 SQLite 的 BookmarkDAO 实现
type SQLiteBookmarkDAO struct {
	db *sql.DB
}

// NewSQLiteBookmarkDAO 创建 SQLiteBookmarkDAO
func NewSQLiteBookmarkDAO(db *sql.DB) *SQLiteBookmarkDAO {
	return &SQLiteBookmarkDAO{db: db}
}

// List 查询所有默认书签，按 sort_order 升序
func (d *SQLiteBookmarkDAO) List() ([]config.BrowserBookmark, error) {
	rows, err := d.db.Query(`
		SELECT name, url, COALESCE(open_on_start, 0),
		       COALESCE(folder, ''), COALESCE(disabled, 0), COALESCE(disabled_profile_ids, '[]')
		FROM browser_bookmarks ORDER BY sort_order ASC, id ASC`)
	if err != nil {
		return nil, fmt.Errorf("查询书签列表失败: %w", err)
	}
	defer rows.Close()

	var list []config.BrowserBookmark
	for rows.Next() {
		var b config.BrowserBookmark
		var openOnStart, disabled int
		var disabledProfileIDs string
		if err := rows.Scan(&b.Name, &b.URL, &openOnStart, &b.Folder, &disabled, &disabledProfileIDs); err != nil {
			return nil, fmt.Errorf("读取书签行失败: %w", err)
		}
		b.OpenOnStart = openOnStart != 0
		b.Disabled = disabled != 0
		b.DisabledProfileIDs = parseStringSlice(disabledProfileIDs)
		list = append(list, b)
	}
	return list, rows.Err()
}

// ReplaceAll 原子替换全部书签（事务保证）
func (d *SQLiteBookmarkDAO) ReplaceAll(bookmarks []config.BrowserBookmark) error {
	tx, err := d.db.Begin()
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM browser_bookmarks`); err != nil {
		return fmt.Errorf("清空书签失败: %w", err)
	}
	for i, b := range bookmarks {
		if b.Name == "" || b.URL == "" {
			continue
		}
		disabledIDs := marshalStringSlice(b.DisabledProfileIDs)
		if _, err := tx.Exec(
			`INSERT INTO browser_bookmarks (name, url, open_on_start, sort_order, folder, disabled, disabled_profile_ids) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			b.Name, b.URL, boolToInt(b.OpenOnStart), i, b.Folder, boolToInt(b.Disabled), disabledIDs,
		); err != nil {
			return fmt.Errorf("插入书签失败: %w", err)
		}
	}
	return tx.Commit()
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

// parseStringSlice 将 JSON 数组字符串解析为 []string；解析失败返回空切片。
func parseStringSlice(s string) []string {
	if s == "" || s == "[]" {
		return []string{}
	}
	var out []string
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return []string{}
	}
	return out
}

// marshalStringSlice 将 []string 序列化为 JSON 数组字符串。
func marshalStringSlice(v []string) string {
	if len(v) == 0 {
		return "[]"
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "[]"
	}
	return string(b)
}
