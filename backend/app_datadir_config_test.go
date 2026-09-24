package backend

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDataDirConfigForAppRootSurvivesCustomRootSelection(t *testing.T) {
	appRoot := t.TempDir()
	customRoot := filepath.Join(t.TempDir(), "selected")
	configPath := dataDirConfigPathForAppRoot(appRoot)

	if err := saveDataDirConfigAtPath(configPath, DataDirConfig{
		CustomDataDir:    customRoot,
		UseCustomDataDir: true,
	}); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadDataDirConfigForAppRoot(appRoot)
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.UseCustomDataDir || loaded.CustomDataDir != customRoot {
		t.Fatalf("loaded data directory config = %#v", loaded)
	}
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("stable data directory config was not persisted: %v", err)
	}
}

func TestLoadDataDirConfigForAppRootFallsBackToInstallRoot(t *testing.T) {
	appRoot := t.TempDir()
	installConfig := filepath.Join(appRoot, dataDirConfigFileName)
	want := DataDirConfig{CustomDataDir: filepath.Join(t.TempDir(), "selected"), UseCustomDataDir: true}
	if err := saveDataDirConfigAtPath(installConfig, want); err != nil {
		t.Fatal(err)
	}

	got, err := loadDataDirConfigForAppRoot(appRoot)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("loaded install-root data directory config = %#v, want %#v", got, want)
	}
}

func TestNormalizeCustomDataDirAnchorsRelativePathAtAppRoot(t *testing.T) {
	appRoot := t.TempDir()
	got := NormalizeCustomDataDir(appRoot, "selected")
	want := filepath.Join(appRoot, "selected")
	if got != want {
		t.Fatalf("normalized path = %q, want %q", got, want)
	}
}

func TestWritableDefaultForInstallPath(t *testing.T) {
	installDir := filepath.Join(t.TempDir(), "Program Files", "链享浏览器")
	defaultDir := filepath.Join(t.TempDir(), "链享浏览器")

	got, ok := writableDefaultForInstallPath(installDir, defaultDir, installDir, func(path string) bool {
		return path == defaultDir
	})
	if !ok || got != defaultDir {
		t.Fatalf("fallback = (%q, %v), want (%q, true)", got, ok, defaultDir)
	}

	if got, ok := writableDefaultForInstallPath(installDir, defaultDir, filepath.Join(installDir, "subdir"), func(string) bool { return true }); ok {
		t.Fatalf("nested custom path unexpectedly fell back to %q", got)
	}
	if got, ok := writableDefaultForInstallPath(installDir, defaultDir, installDir, func(string) bool { return false }); ok {
		t.Fatalf("unwritable default unexpectedly selected: %q", got)
	}
}

func TestProtectedInstallDataDirIsRejectedEvenWhenWritable(t *testing.T) {
	t.Setenv("LOCALAPPDATA", filepath.Join(t.TempDir(), "LocalAppData"))
	programFiles := filepath.Join(t.TempDir(), "Program Files")
	t.Setenv("ProgramFiles", programFiles)
	installRoot := filepath.Join(programFiles, "链享浏览器")
	if err := os.MkdirAll(installRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		t.Skip("protected install detection is Windows specific")
	}

	// The temp install directory is writable, as it is for an elevated
	// process; it must still be treated as unusable for data.
	if !IsProtectedInstallDataDir(installRoot, installRoot) {
		t.Fatal("install root below Program Files must be treated as protected")
	}
	if !IsProtectedInstallDataDir(installRoot, filepath.Join(installRoot, "data")) {
		t.Fatal("install data directory must be treated as protected")
	}
	if IsProtectedInstallDataDir(installRoot, filepath.Join(t.TempDir(), "elsewhere")) {
		t.Fatal("unrelated directory must not be treated as protected")
	}
}
