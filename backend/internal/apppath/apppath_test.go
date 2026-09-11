package apppath

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWindowsInstalledRootUsesLocalAppDataWhenInstallDirIsReadOnly(t *testing.T) {
	installRoot := filepath.Join(t.TempDir(), "Program Files", "链享浏览器")
	localAppData := filepath.Join(t.TempDir(), "LocalAppData")
	t.Setenv("LOCALAPPDATA", localAppData)
	t.Setenv("APPDATA", "")

	root := detectForOS(installRoot, "windows")
	if !root.detached {
		t.Fatalf("expected an unwritable Windows install root to use detached state")
	}
	wantStateRoot := filepath.Join(localAppData, "链享浏览器")
	if root.stateRoot != wantStateRoot {
		t.Fatalf("state root = %q, want %q", root.stateRoot, wantStateRoot)
	}
	if got := resolveForOS(installRoot, "data/app.db", "windows"); got != filepath.Join(wantStateRoot, "data", "app.db") {
		t.Fatalf("resolved data path = %q", got)
	}
	if got := resolveForOS(installRoot, "bin/xray.exe", "windows"); got != filepath.Join(installRoot, "bin", "xray.exe") {
		t.Fatalf("resolved runtime path = %q, want install directory", got)
	}
}

func TestEnsureWritableLayoutMigratesExistingInstalledData(t *testing.T) {
	base := t.TempDir()
	installRoot := filepath.Join(base, "Program Files", "链享浏览器")
	localAppData := filepath.Join(base, "LocalAppData")
	if err := os.MkdirAll(filepath.Join(installRoot, "data", "profiles"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(installRoot, "config.yaml"), []byte("app: installed\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(installRoot, "data", "app.db"), []byte("database"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(installRoot, "data", "profiles", "one.txt"), []byte("profile"), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LOCALAPPDATA", localAppData)
	t.Setenv("APPDATA", "")
	// The temporary install directory is writable in the test process. Force the
	// detected layout to represent a protected installation directory.
	installRootAbs := normalizeRoot(installRoot)
	rootsCache.Store(buildCacheKey("windows", installRootAbs), roots{
		installRoot: installRootAbs,
		stateRoot:   filepath.Join(localAppData, "链享浏览器"),
		detached:    true,
	})

	if err := ensureWritableLayoutForOS(installRoot, "windows"); err != nil {
		t.Fatalf("ensure writable layout: %v", err)
	}
	stateRoot := filepath.Join(localAppData, "链享浏览器")
	for _, path := range []string{
		filepath.Join(stateRoot, "config.yaml"),
		filepath.Join(stateRoot, "data", "app.db"),
		filepath.Join(stateRoot, "data", "profiles", "one.txt"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected migrated file %q: %v", path, err)
		}
	}
}
