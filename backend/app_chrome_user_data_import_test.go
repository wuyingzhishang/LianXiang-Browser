package backend

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"lianxiang-browser/backend/internal/browser"
	"lianxiang-browser/backend/internal/config"
)

func TestImportChromeUserDataCreatesIndependentProfile(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "chrome-source")
	writeChromeImportTestFile(t, sourceDir, "Local State", `{}`)
	writeChromeImportTestFile(t, sourceDir, filepath.Join("Default", "Preferences"), `{}`)
	writeChromeImportTestFile(t, sourceDir, filepath.Join("Default", "Bookmarks"), `{"roots":{}}`)
	writeChromeImportTestFile(t, sourceDir, filepath.Join("Default", "Cache", "cached.bin"), "cache")
	writeChromeImportTestFile(t, sourceDir, "SingletonLock", "locked")

	cfg := config.DefaultConfig()
	cfg.Browser.UserDataRoot = filepath.Join(root, "user-data")
	app := NewApp(root)
	app.config = cfg
	app.browserMgr = browser.NewManager(cfg, root)

	previousFinder := findBrowserUserDataProcesses
	findBrowserUserDataProcesses = func(string) ([]browserUserDataProcess, error) { return nil, nil }
	t.Cleanup(func() { findBrowserUserDataProcesses = previousFinder })

	result, err := app.importChromeUserDataFromPath(sourceDir, "本地 Chrome 5")
	if err != nil {
		t.Fatalf("importChromeUserDataFromPath returned error: %v", err)
	}
	if result.ProfileID == "" || result.ProfileName != "本地 Chrome 5" {
		t.Fatalf("unexpected result: %#v", result)
	}
	profile := app.browserMgr.Profiles[result.ProfileID]
	if profile == nil {
		t.Fatalf("imported profile %s was not registered", result.ProfileID)
	}
	targetDir := app.browserMgr.ResolveUserDataDir(profile)
	for _, rel := range []string{"Local State", filepath.Join("Default", "Preferences"), filepath.Join("Default", "Bookmarks")} {
		if _, err := os.Stat(filepath.Join(targetDir, rel)); err != nil {
			t.Fatalf("expected imported file %s: %v", rel, err)
		}
	}
	for _, rel := range []string{"SingletonLock", filepath.Join("Default", "Cache", "cached.bin")} {
		if _, err := os.Stat(filepath.Join(targetDir, rel)); !os.IsNotExist(err) {
			t.Fatalf("volatile file should be skipped: %s (err=%v)", rel, err)
		}
	}
	if result.CopiedFiles != 3 || result.SkippedFiles != 2 {
		t.Fatalf("unexpected copy stats: %#v", result)
	}
}

func TestValidateChromeUserDataDirRejectsProfileSubdirectory(t *testing.T) {
	sourceDir := t.TempDir()
	writeChromeImportTestFile(t, sourceDir, "Preferences", `{}`)
	if _, err := validateChromeUserDataDir(sourceDir); err == nil {
		t.Fatal("expected a Chrome profile subdirectory to be rejected")
	}
}

func TestImportChromeUserDataRejectsRunningSource(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	writeChromeImportTestFile(t, source, "Local State", `{}`)
	writeChromeImportTestFile(t, source, "Default/Preferences", `{}`)
	cfg := config.DefaultConfig()
	app := NewApp(root)
	app.config = cfg
	app.browserMgr = browser.NewManager(cfg, root)
	previousFinder := findBrowserUserDataProcesses
	findBrowserUserDataProcesses = func(string) ([]browserUserDataProcess, error) {
		return []browserUserDataProcess{{}}, nil
	}
	t.Cleanup(func() { findBrowserUserDataProcesses = previousFinder })
	if _, err := app.importChromeUserDataFromPath(source, ""); err == nil {
		t.Fatal("expected running source to be rejected")
	}
	if len(app.browserMgr.Profiles) != 0 {
		t.Fatal("running source registered a profile")
	}
}

func TestImportChromeUserDataRollsBackFailedSave(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	writeChromeImportTestFile(t, source, "Local State", `{}`)
	writeChromeImportTestFile(t, source, "Default/Preferences", `{}`)
	if err := os.Mkdir(filepath.Join(root, "config.yaml"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := config.DefaultConfig()
	cfg.Browser.UserDataRoot = filepath.Join(root, "target")
	app := NewApp(root)
	app.config = cfg
	app.browserMgr = browser.NewManager(cfg, root)
	previousFinder := findBrowserUserDataProcesses
	findBrowserUserDataProcesses = func(string) ([]browserUserDataProcess, error) { return nil, nil }
	t.Cleanup(func() { findBrowserUserDataProcesses = previousFinder })
	if _, err := app.importChromeUserDataFromPath(source, ""); err == nil {
		t.Fatal("expected save failure")
	}
	if len(app.browserMgr.Profiles) != 0 {
		t.Fatal("failed import left a registered profile")
	}
	entries, err := os.ReadDir(cfg.Browser.UserDataRoot)
	if err != nil || len(entries) != 0 {
		t.Fatalf("failed import left data: %v, %v", entries, err)
	}
	if _, err := os.Stat(filepath.Join(source, "Default/Preferences")); err != nil {
		t.Fatal("source was changed:", err)
	}
}

func TestChromeUserDataImportRejectsOverlappingPaths(t *testing.T) {
	root := t.TempDir()
	for i, paths := range [][2]string{{root, root}, {root, filepath.Join(root, "child")}, {filepath.Join(root, "child"), root}} {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			if err := ensureSeparateChromeImportPaths(paths[0], paths[1]); err == nil {
				t.Fatal("overlapping paths accepted")
			}
		})
	}
}

func writeChromeImportTestFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create test directory failed: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write test file failed: %v", err)
	}
}
