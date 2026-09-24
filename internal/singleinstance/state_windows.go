//go:build windows

package singleinstance

import (
	"os"
	"path/filepath"
	"strings"

	"lianxiang-browser/backend"
)

func singleInstanceStateRoot(appRoot string) string {
	if dir := strings.TrimSpace(os.Getenv("LOCALAPPDATA")); dir != "" {
		return filepath.Join(dir, "链享浏览器")
	}
	if dir, err := os.UserConfigDir(); err == nil && strings.TrimSpace(dir) != "" {
		return filepath.Join(dir, "链享浏览器")
	}

	// The lock belongs to the application installation, not to the selected
	// data directory. Keeping it in the stable default state root means a
	// directory change cannot create a second application process or strand
	// the desktop shortcut on an unavailable drive.
	if root := backend.RuntimeDefaultStateRoot(appRoot); root != "" {
		return root
	}
	if root := backend.RuntimeStateRoot(appRoot); root != "" {
		return root
	}
	return appRoot
}
