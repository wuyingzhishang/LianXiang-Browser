package backend

import (
	"lianxiang-browser/backend/internal/apppath"
	"path/filepath"
	"strings"
)

// SetCustomDataDir 设置全局自定义数据目录
func SetCustomDataDir(path string) {
	apppath.SetCustomDataDir(path)
}

// GetCustomDataDir 获取全局自定义数据目录
func GetCustomDataDir() string {
	return apppath.GetCustomDataDir()
}

// NormalizeCustomDataDir converts a persisted or user-entered path to an
// absolute path anchored at the application root. Desktop shortcuts can use
// an arbitrary working directory, so relative paths must not depend on CWD.
func NormalizeCustomDataDir(appRoot, path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if filepath.IsAbs(path) {
		if abs, err := filepath.Abs(path); err == nil {
			return abs
		}
		return filepath.Clean(path)
	}
	base := strings.TrimSpace(appRoot)
	if base == "" {
		if cwd, err := filepath.Abs("."); err == nil {
			base = cwd
		}
	}
	if abs, err := filepath.Abs(filepath.Join(base, path)); err == nil {
		return abs
	}
	return filepath.Clean(filepath.Join(base, path))
}
