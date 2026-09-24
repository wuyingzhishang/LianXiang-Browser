//go:build !windows

package singleinstance

import (
	"lianxiang-browser/backend"
	"path/filepath"
	"strings"
)

func singleInstanceStateRoot(appRoot string) string {
	if root := strings.TrimSpace(backend.RuntimeDefaultStateRoot(appRoot)); root != "" {
		return root
	}
	if root := strings.TrimSpace(backend.RuntimeStateRoot(appRoot)); root != "" {
		return root
	}
	return filepath.Join(appRoot, "data")
}
