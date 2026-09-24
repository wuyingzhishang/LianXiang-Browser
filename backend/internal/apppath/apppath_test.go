package apppath

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWindowsProtectedInstallUsesUserStateRoot(t *testing.T) {
	t.Setenv("LOCALAPPDATA", filepath.Join(t.TempDir(), "LocalAppData"))
	programFiles := filepath.Join(t.TempDir(), "Program Files")
	t.Setenv("ProgramFiles", programFiles)
	installRoot := filepath.Join(programFiles, "链享浏览器")
	if err := os.MkdirAll(installRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	if !shouldDetachStateRoot("windows", installRoot) {
		t.Fatalf("an install below Program Files must use a detached state root")
	}

	stateRoot := userStateRootForOS("windows", installRoot)
	want := filepath.Join(os.Getenv("LOCALAPPDATA"), "链享浏览器")
	if stateRoot != want {
		t.Fatalf("state root = %q, want %q", stateRoot, want)
	}
}

func TestWindowsPortableInstallKeepsInstallRoot(t *testing.T) {
	t.Setenv("ProgramFiles", filepath.Join(t.TempDir(), "Program Files"))
	installRoot := filepath.Join(t.TempDir(), "portable", "链享浏览器")
	if err := os.MkdirAll(installRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	if shouldDetachStateRoot("windows", installRoot) {
		t.Fatalf("a writable portable install must keep its install root")
	}
}

func TestWindowsPortableMissingPathKeepsInstallRoot(t *testing.T) {
	installRoot := filepath.Join(t.TempDir(), "missing", "链享浏览器")
	if shouldDetachStateRoot("windows", installRoot) {
		t.Fatalf("Windows default data must remain beside the executable")
	}
}

func TestWindowsRelocatedProgramFilesUsesUserStateRoot(t *testing.T) {
	localAppData := filepath.Join(t.TempDir(), "LocalAppData")
	t.Setenv("LOCALAPPDATA", localAppData)
	// Simulate Program Files being relocated to D: while the process environment
	// still advertises the system-drive value.
	t.Setenv("ProgramFiles", `C:\Program Files`)
	t.Setenv("ProgramFiles(x86)", `C:\Program Files (x86)`)
	t.Setenv("ProgramW6432", `C:\Program Files`)

	installRoot := `D:\Program Files\链享浏览器`
	if !shouldDetachStateRoot("windows", installRoot) {
		t.Fatalf("a relocated Program Files install must use a detached state root")
	}

	want := filepath.Join(localAppData, "链享浏览器")
	if got := userStateRootForOS("windows", installRoot); got != want {
		t.Fatalf("state root = %q, want %q", got, want)
	}
}

func TestResolveForWindowsUsesStateRootExceptBin(t *testing.T) {
	localAppData := filepath.Join(t.TempDir(), "LocalAppData")
	t.Setenv("LOCALAPPDATA", localAppData)
	installRoot := filepath.Join(t.TempDir(), "missing", "链享浏览器")

	configPath := resolveForOS(installRoot, "config.yaml", "windows")
	if want := filepath.Join(installRoot, "config.yaml"); configPath != want {
		t.Fatalf("config path = %q, want %q", configPath, want)
	}

	binPath := resolveForOS(installRoot, "bin/xray.exe", "windows")
	if want := filepath.Join(installRoot, "bin", "xray.exe"); binPath != want {
		t.Fatalf("binary path = %q, want %q", binPath, want)
	}
}

func TestCopyDirIfMissingMergesIntoExistingStateDirectory(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	if err := os.WriteFile(filepath.Join(src, "app.db"), []byte("profiles"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(src, "profile-a", "Default"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "profile-a", "Default", "Preferences"), []byte("chrome"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dst, "existing.txt"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := copyDirIfMissing(src, dst); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(dst, "app.db")); err != nil || string(got) != "profiles" {
		t.Fatalf("app.db was not migrated: content=%q err=%v", got, err)
	}
	if got, err := os.ReadFile(filepath.Join(dst, "profile-a", "Default", "Preferences")); err != nil || string(got) != "chrome" {
		t.Fatalf("profile data was not migrated: content=%q err=%v", got, err)
	}
	if got, err := os.ReadFile(filepath.Join(dst, "existing.txt")); err != nil || string(got) != "keep" {
		t.Fatalf("existing state data changed: content=%q err=%v", got, err)
	}
}

func TestEnsureWritableLayoutWithCustomDirectoryCopiesExistingState(t *testing.T) {
	installRoot := t.TempDir()
	customRoot := filepath.Join(t.TempDir(), "selected")
	if err := os.MkdirAll(filepath.Join(installRoot, "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(installRoot, "data", "marker.txt"), []byte("profile-data"), 0o644); err != nil {
		t.Fatal(err)
	}

	SetCustomDataDir(customRoot)
	t.Cleanup(func() { SetCustomDataDir("") })
	if err := ensureWritableLayoutForOS(installRoot, "windows"); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(filepath.Join(customRoot, "data", "marker.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "profile-data" {
		t.Fatalf("custom data marker = %q, want profile-data", got)
	}
}

func TestEnsureWritableLayoutSkipsNestedInstallDataSource(t *testing.T) {
	installRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(installRoot, "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(installRoot, "data", "marker.txt"), []byte("profile-data"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Selecting install\data is unusual but valid input from a directory
	// picker. It must not make the startup copy walk its own output forever.
	SetCustomDataDir(filepath.Join(installRoot, "data"))
	t.Cleanup(func() { SetCustomDataDir("") })
	if err := ensureWritableLayoutForOS(installRoot, "windows"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(installRoot, "data", "data", "data")); err == nil {
		t.Fatal("nested data copy recursed into its own output")
	}
}

func TestEnsureWritableLayoutRejectsCustomFilePath(t *testing.T) {
	installRoot := t.TempDir()
	customPath := filepath.Join(t.TempDir(), "selected")
	if err := os.WriteFile(customPath, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}

	SetCustomDataDir(customPath)
	t.Cleanup(func() { SetCustomDataDir("") })
	if err := ensureWritableLayoutForOS(installRoot, "windows"); err == nil {
		t.Fatal("custom data file path should be rejected")
	}
}

func protectedInstallForTest(t *testing.T) (installRoot, stateRoot string) {
	t.Helper()
	localAppData := filepath.Join(t.TempDir(), "LocalAppData")
	t.Setenv("LOCALAPPDATA", localAppData)
	programFiles := filepath.Join(t.TempDir(), "Program Files")
	t.Setenv("ProgramFiles", programFiles)
	installRoot = filepath.Join(programFiles, "链享浏览器")
	stateRoot = filepath.Join(localAppData, "链享浏览器")
	for _, dir := range []string{filepath.Join(installRoot, "data"), filepath.Join(stateRoot, "data")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return installRoot, stateRoot
}

func writeFileWithTime(t *testing.T, path, content string, modTime time.Time) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatal(err)
	}
}

func TestAdoptInstallStateReplacesOlderStateDatabase(t *testing.T) {
	installRoot, stateRoot := protectedInstallForTest(t)
	now := time.Now()
	writeFileWithTime(t, filepath.Join(stateRoot, "data", "app.db"), "old", now.Add(-time.Hour))
	writeFileWithTime(t, filepath.Join(stateRoot, "data", "app.db-wal"), "old-wal", now.Add(-time.Hour))
	writeFileWithTime(t, filepath.Join(installRoot, "data", "app.db"), "new", now)

	adopted, err := adoptInstallStateForOS(installRoot, "windows")
	if err != nil {
		t.Fatal(err)
	}
	if len(adopted) != 1 || adopted[0] != filepath.Join("data", "app.db") {
		t.Fatalf("adopted = %#v, want data/app.db", adopted)
	}
	if got, _ := os.ReadFile(filepath.Join(stateRoot, "data", "app.db")); string(got) != "new" {
		t.Fatalf("state app.db = %q, want new", got)
	}
	if _, err := os.Stat(filepath.Join(stateRoot, "data", "app.db-wal")); !os.IsNotExist(err) {
		t.Fatalf("stale WAL must not stay beside the adopted database: %v", err)
	}
	backups, _ := filepath.Glob(filepath.Join(stateRoot, "data", "app.db.before-install-adopt-*"))
	if len(backups) != 1 {
		t.Fatalf("backups = %#v, want the replaced database kept", backups)
	}
	if got, _ := os.ReadFile(backups[0]); string(got) != "old" {
		t.Fatalf("backup content = %q, want old", got)
	}
}

func TestAdoptInstallStateKeepsNewerStateDatabase(t *testing.T) {
	installRoot, stateRoot := protectedInstallForTest(t)
	now := time.Now()
	writeFileWithTime(t, filepath.Join(installRoot, "data", "app.db"), "old", now.Add(-time.Hour))
	writeFileWithTime(t, filepath.Join(stateRoot, "data", "app.db"), "new", now)

	adopted, err := adoptInstallStateForOS(installRoot, "windows")
	if err != nil {
		t.Fatal(err)
	}
	if len(adopted) != 0 {
		t.Fatalf("adopted = %#v, want nothing", adopted)
	}
	if got, _ := os.ReadFile(filepath.Join(stateRoot, "data", "app.db")); string(got) != "new" {
		t.Fatalf("state app.db = %q, want new", got)
	}
}

func TestAdoptInstallStateIgnoresPortableInstall(t *testing.T) {
	t.Setenv("ProgramFiles", filepath.Join(t.TempDir(), "Program Files"))
	installRoot := filepath.Join(t.TempDir(), "portable")
	if err := os.MkdirAll(filepath.Join(installRoot, "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFileWithTime(t, filepath.Join(installRoot, "data", "app.db"), "db", time.Now())

	adopted, err := adoptInstallStateForOS(installRoot, "windows")
	if err != nil || len(adopted) != 0 {
		t.Fatalf("adopted = %#v, err = %v, want no-op", adopted, err)
	}
}
