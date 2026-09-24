package apppath

import (
	"fmt"
	"io"
	"lianxiang-browser/backend/internal/fsutil"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"sync"
	"time"
)

const appStateDirName = "lianxiang-browser"

type roots struct {
	installRoot   string
	stateRoot     string
	customDataDir string
	detached      bool
}

var rootsCache sync.Map
var customDataDirOverride string
var customDataDirMutex sync.RWMutex

// SetCustomDataDir 设置全局自定义数据目录
func SetCustomDataDir(path string) {
	customDataDirMutex.Lock()
	defer customDataDirMutex.Unlock()
	customDataDirOverride = strings.TrimSpace(path)
	rootsCache = sync.Map{}
}

// GetCustomDataDir 获取全局自定义数据目录
func GetCustomDataDir() string {
	customDataDirMutex.RLock()
	defer customDataDirMutex.RUnlock()
	return customDataDirOverride
}

// InstallRoot 返回应用安装根目录的绝对路径。
func InstallRoot(appRoot string) string {
	return detect(appRoot).installRoot
}

// StateRoot 返回应用可写状态目录的绝对路径。
func StateRoot(appRoot string) string {
	customDataDirMutex.RLock()
	custom := customDataDirOverride
	customDataDirMutex.RUnlock()

	if custom != "" {
		if abs, err := filepath.Abs(custom); err == nil {
			return abs
		}
		return filepath.Clean(custom)
	}

	return detect(appRoot).stateRoot
}

// DefaultStateRoot 返回未启用自定义目录时的稳定状态目录。
// 数据目录选择配置必须保存在这里，否则启用自定义目录后会失去读取配置
// 所需的入口，下一次启动也就无法恢复同一个目录。
func DefaultStateRoot(appRoot string) string {
	return detect(appRoot).stateRoot
}

// IsDetached 返回当前是否启用了“安装目录只读、状态目录独立”的模式。
func IsDetached(appRoot string) bool {
	return detect(appRoot).detached
}

// Resolve 将相对路径解析到安装目录或用户状态目录。
// 已安装到受保护目录的应用在启用 detached 模式后，除 bin/ 外的相对路径都会落到用户可写目录。
func Resolve(appRoot, p string) string {
	return resolveForOS(appRoot, p, goruntime.GOOS)
}

func resolveForOS(appRoot, p, goos string) string {
	p = fsutil.NormalizePathInput(p)
	if p == "" {
		return ""
	}
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}

	customDataDirMutex.RLock()
	custom := customDataDirOverride
	customDataDirMutex.RUnlock()

	if custom != "" && useStateRoot(p) {
		var base string
		if abs, err := filepath.Abs(custom); err == nil {
			base = abs
		} else {
			base = filepath.Clean(custom)
		}
		return filepath.Join(base, p)
	}

	root := detectForOS(appRoot, goos)
	base := root.installRoot
	if root.detached && useStateRoot(p) {
		base = root.stateRoot
	}
	return filepath.Join(base, p)
}

// EnsureWritableLayout 为需要 detached 状态目录的已安装应用准备首启所需的可写目录，
// 并把随包默认配置迁移到用户目录。
func EnsureWritableLayout(appRoot string) error {
	return ensureWritableLayoutForOS(appRoot, goruntime.GOOS)
}

func ensureWritableLayoutForOS(appRoot, goos string) error {
	root := detectForOS(appRoot, goos)
	customDataDirMutex.RLock()
	custom := strings.TrimSpace(customDataDirOverride)
	customDataDirMutex.RUnlock()
	if custom != "" {
		customRoot, err := filepath.Abs(custom)
		if err != nil {
			customRoot = filepath.Clean(custom)
		}
		if err := os.MkdirAll(customRoot, 0755); err != nil {
			return err
		}
		// MkdirAll succeeds for an existing directory even when the current
		// process cannot write to it. SQLite needs write access to the data
		// directory (in particular to create/update the WAL and SHM files), so
		// reject the selection before startup opens the database. main.go will
		// then fall back to the stable default state root.
		if !dirWritable(customRoot) {
			return fmt.Errorf("自定义数据目录不可写: %s", customRoot)
		}
		dataDir := filepath.Join(customRoot, "data")
		if err := os.MkdirAll(dataDir, 0755); err != nil {
			return err
		}
		if !dirWritable(dataDir) {
			return fmt.Errorf("自定义数据目录不可写: %s", dataDir)
		}
		if filepath.Clean(customRoot) == filepath.Clean(root.stateRoot) {
			return nil
		}

		// 自定义目录可能是在旧版本中只保存了开关，或用户点击保存后
		// 尚未完成迁移。启动时仅补齐目标中不存在的文件，避免覆盖用户
		// 已有数据，同时兼容安装目录和旧默认状态目录中的数据。
		for _, dir := range []string{"data", "chrome"} {
			targetDir := filepath.Join(customRoot, dir)
			for _, sourceDir := range []string{
				filepath.Join(root.stateRoot, dir),
				filepath.Join(root.installRoot, dir),
			} {
				// A user can select an existing install data directory. In that
				// case the destination would be nested inside the source and a
				// recursive copy could keep discovering its own output.
				if pathEqualOrWithin(targetDir, sourceDir) {
					continue
				}
				if err := copyDirIfMissing(sourceDir, targetDir); err != nil {
					return err
				}
			}
		}
		for _, file := range []string{"config.yaml", "proxies.yaml", "backup.local.yaml"} {
			if err := copyFileIfMissing(filepath.Join(root.stateRoot, file), filepath.Join(customRoot, file)); err != nil {
				return err
			}
			if err := copyFileIfMissing(filepath.Join(root.installRoot, file), filepath.Join(customRoot, file)); err != nil {
				return err
			}
		}
		return nil
	}
	if !root.detached {
		return nil
	}

	if err := os.MkdirAll(root.stateRoot, 0755); err != nil {
		return err
	}
	// The first detached launch may already have created state/data before the
	// application database is initialized. Merge the packaged data directory
	// afterward so that an existing install keeps its profiles and app.db.
	if err := copyDirIfMissing(
		filepath.Join(root.installRoot, "data"),
		filepath.Join(root.stateRoot, "data"),
	); err != nil {
		return err
	}

	if err := copyFileIfMissing(
		filepath.Join(root.installRoot, "config.yaml"),
		filepath.Join(root.stateRoot, "config.yaml"),
	); err != nil {
		return err
	}
	if err := copyFileIfMissing(
		filepath.Join(root.installRoot, "proxies.yaml"),
		filepath.Join(root.stateRoot, "proxies.yaml"),
	); err != nil {
		return err
	}
	if err := copyDirIfMissing(
		filepath.Join(root.installRoot, "chrome"),
		filepath.Join(root.stateRoot, "chrome"),
	); err != nil {
		return err
	}

	return nil
}

// installOnlyStateFiles 是只由应用写入、安装包不会携带的状态文件。
// config.yaml 不在其中：安装包会写入默认配置，按修改时间判断会误把默认值当成用户数据。
var installOnlyStateFiles = []string{
	filepath.Join("data", "app.db"),
	"proxies.yaml",
}

// AdoptInstallState 把以管理员身份运行时写入受保护安装目录、且比用户状态目录更新的
// 数据库和代理配置接回用户状态目录。被替换的旧文件会保留为带时间戳的备份。
// 仅用于修复旧版本把安装目录保存为自定义数据目录的情况，调用方需保证数据库尚未打开。
func AdoptInstallState(appRoot string) ([]string, error) {
	return adoptInstallStateForOS(appRoot, goruntime.GOOS)
}

func adoptInstallStateForOS(appRoot, goos string) ([]string, error) {
	root := detectForOS(appRoot, goos)
	if !root.detached {
		return nil, nil
	}
	stamp := time.Now().Format("20060102-150405")
	var adopted []string
	for _, rel := range installOnlyStateFiles {
		src := filepath.Join(root.installRoot, rel)
		dst := filepath.Join(root.stateRoot, rel)
		srcInfo, err := os.Stat(src)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return adopted, err
		}
		if srcInfo.IsDir() {
			continue
		}
		dstInfo, err := os.Stat(dst)
		if err == nil && !srcInfo.ModTime().After(dstInfo.ModTime()) {
			continue
		}
		if err != nil && !os.IsNotExist(err) {
			return adopted, err
		}
		// SQLite 的 -wal / -shm 必须与主库一起移动，否则会把旧日志套用到新库上。
		for _, suffix := range []string{"", "-wal", "-shm"} {
			if err := backupFileIfExists(dst+suffix, dst+suffix+".before-install-adopt-"+stamp); err != nil {
				return adopted, err
			}
		}
		for _, suffix := range []string{"", "-wal", "-shm"} {
			info, err := os.Stat(src + suffix)
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return adopted, err
			}
			if err := copyFile(src+suffix, dst+suffix, info.Mode().Perm()|0o600); err != nil {
				return adopted, err
			}
		}
		adopted = append(adopted, rel)
	}
	return adopted, nil
}

func backupFileIfExists(path, backupPath string) error {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return os.Rename(path, backupPath)
}

func detect(appRoot string) roots {
	return detectForOS(appRoot, goruntime.GOOS)
}

func detectForOS(appRoot, goos string) roots {
	normalized := normalizeRoot(appRoot)
	cacheKey := buildCacheKey(goos, normalized)
	if cached, ok := rootsCache.Load(cacheKey); ok {
		return cached.(roots)
	}

	root := roots{
		installRoot: normalized,
		stateRoot:   normalized,
	}
	if shouldDetachStateRoot(goos, normalized) {
		root.stateRoot = userStateRootForOS(goos, normalized)
		root.detached = root.stateRoot != "" && root.stateRoot != normalized
	}

	actual, _ := rootsCache.LoadOrStore(cacheKey, root)
	return actual.(roots)
}

func buildCacheKey(goos, root string) string {
	return normalizeGOOS(goos) + "\x00" + root
}

func normalizeGOOS(goos string) string {
	return strings.ToLower(strings.TrimSpace(goos))
}

func shouldDetachStateRoot(goos, installRoot string) bool {
	switch normalizeGOOS(goos) {
	case "windows":
		// The NSIS installer uses %ProgramFiles% by default. That directory can
		// be written by the elevated installer, but not by the normal user who
		// starts the shortcut later. Decide from the install location rather than
		// the current token so the first and subsequent launches share one state
		// root instead of splitting data or failing on the instance lock.
		return isWindowsProtectedInstall(installRoot)
	case "linux":
		return !dirWritable(installRoot)
	case "darwin":
		return isMacAppBundleRoot(installRoot) || !dirWritable(installRoot)
	default:
		return false
	}
}

func isWindowsProtectedInstall(installRoot string) bool {
	for _, candidate := range []string{
		os.Getenv("ProgramFiles"),
		os.Getenv("ProgramFiles(x86)"),
		os.Getenv("ProgramW6432"),
	} {
		candidate = strings.TrimSpace(candidate)
		if candidate != "" && pathEqualOrWithin(installRoot, candidate) {
			return true
		}
	}

	// Program Files can be relocated to another volume. In that case the
	// environment variables above still commonly point at the system drive,
	// while the installer may use a path such as D:\\Program Files\\链享浏览器.
	// Keep the detection aligned with Windows' protected-directory convention
	// so the first elevated launch and later normal launches share the same
	// user-writable state root.
	clean := strings.TrimSuffix(filepath.ToSlash(filepath.Clean(installRoot)), "/")
	// Normalize both separators so this decision remains deterministic in
	// tests that pass Windows paths while running on a non-Windows host.
	clean = strings.ReplaceAll(clean, "\\", "/")
	for _, segment := range strings.Split(clean, "/") {
		switch strings.ToLower(strings.TrimSpace(segment)) {
		case "program files", "program files (x86)":
			return true
		}
	}
	return false
}

func normalizeRoot(appRoot string) string {
	root := strings.TrimSpace(appRoot)
	if root == "" {
		if cwd, err := os.Getwd(); err == nil {
			root = cwd
		}
	}
	if root == "" {
		root = "."
	}
	if abs, err := filepath.Abs(root); err == nil {
		return abs
	}
	return filepath.Clean(root)
}

func userStateRootForOS(goos, fallback string) string {
	switch normalizeGOOS(goos) {
	case "windows":
		if base := strings.TrimSpace(os.Getenv("LOCALAPPDATA")); base != "" {
			return filepath.Join(base, "链享浏览器")
		}
		if base, err := os.UserConfigDir(); err == nil && strings.TrimSpace(base) != "" {
			return filepath.Join(base, "链享浏览器")
		}
	case "linux":
		if base := strings.TrimSpace(os.Getenv("XDG_DATA_HOME")); base != "" {
			return filepath.Join(base, appStateDirName)
		}
		if home := configuredHomeDir(); home != "" {
			return filepath.Join(home, ".local", "share", appStateDirName)
		}
	case "darwin":
		if home := configuredHomeDir(); home != "" {
			return filepath.Join(home, "Library", "Application Support", appStateDirName)
		}
	}
	if tmp := strings.TrimSpace(os.TempDir()); tmp != "" {
		return filepath.Join(tmp, appStateDirName)
	}
	return fallback
}

func configuredHomeDir() string {
	if home := strings.TrimSpace(os.Getenv("HOME")); home != "" {
		return home
	}
	if home, err := os.UserHomeDir(); err == nil {
		return strings.TrimSpace(home)
	}
	return ""
}

func isMacAppBundleRoot(dir string) bool {
	clean := strings.TrimSuffix(filepath.ToSlash(filepath.Clean(dir)), "/")
	lower := strings.ToLower(clean)
	return strings.HasSuffix(lower, ".app/contents/macos") || strings.HasSuffix(lower, ".app/contents/resources")
}

func dirWritable(dir string) bool {
	if strings.TrimSpace(dir) == "" {
		return false
	}
	file, err := os.CreateTemp(dir, ".ant-browser-write-test-*")
	if err != nil {
		return false
	}
	name := file.Name()
	_ = file.Close()
	_ = os.Remove(name)
	return true
}

func useStateRoot(p string) bool {
	clean := filepath.ToSlash(fsutil.NormalizePathInput(p))
	if clean == "" || clean == "." {
		return false
	}
	return clean != "bin" && !strings.HasPrefix(clean, "bin/")
}

func pathEqualOrWithin(path, root string) bool {
	pathAbs, pathErr := filepath.Abs(path)
	rootAbs, rootErr := filepath.Abs(root)
	if pathErr != nil || rootErr != nil {
		pathAbs = filepath.Clean(path)
		rootAbs = filepath.Clean(root)
	}
	rel, err := filepath.Rel(rootAbs, pathAbs)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func copyFileIfMissing(src, dst string) error {
	if _, err := os.Stat(dst); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	data, err := os.ReadFile(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

func copyDirIfMissing(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !info.IsDir() {
		return nil
	}
	if dstInfo, err := os.Stat(dst); err == nil {
		if !dstInfo.IsDir() {
			return fmt.Errorf("目标路径不是目录: %s", dst)
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	return filepath.Walk(src, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := dst
		if rel != "." {
			target = filepath.Join(dst, rel)
		}
		if info.IsDir() {
			dirMode := info.Mode().Perm() | 0700
			return os.MkdirAll(target, dirMode)
		}
		if _, err := os.Stat(target); err == nil {
			return nil
		} else if !os.IsNotExist(err) {
			return err
		}
		return copyFile(path, target, info.Mode().Perm())
	})
}

func copyFile(src, dst string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}
