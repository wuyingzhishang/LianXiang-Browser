package backend

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"lianxiang-browser/backend/internal/apppath"
)

// DataDirectoryInfo 数据目录信息
type DataDirectoryInfo struct {
	CurrentDataDir   string `json:"currentDataDir"`
	DefaultDataDir   string `json:"defaultDataDir"`
	CustomDataDir    string `json:"customDataDir"`
	UseCustomDataDir bool   `json:"useCustomDataDir"`
	IsCustomDirValid bool   `json:"isCustomDirValid"`
}

// DataDirectorySelectResult 选择数据目录结果
type DataDirectorySelectResult struct {
	Selected  bool   `json:"selected"`
	Directory string `json:"directory"`
}

// DataDirectoryMigrateOptions 数据迁移选项
type DataDirectoryMigrateOptions struct {
	SourceDir     string `json:"sourceDir"`
	TargetDir     string `json:"targetDir"`
	MigrateData   bool   `json:"migrateData"`
	MigrateChrome bool   `json:"migrateChrome"`
	MigrateConfig bool   `json:"migrateConfig"`
}

// DataDirectoryMigrateResult 数据迁移结果
type DataDirectoryMigrateResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
}

// DataDirectoryGetInfo 获取当前数据目录信息
func (a *App) DataDirectoryGetInfo() DataDirectoryInfo {
	if a == nil {
		return DataDirectoryInfo{}
	}

	config, _ := loadDataDirConfigForAppRoot(a.appRoot)

	currentDir := a.appStateRootAbs()
	defaultDir := RuntimeDefaultStateRoot(a.appRoot)
	customDir := NormalizeCustomDataDir(a.appRoot, config.CustomDataDir)

	isValid := false
	if customDir != "" && !IsProtectedInstallDataDir(a.appRoot, customDir) {
		if info, err := os.Stat(customDir); err == nil && info.IsDir() && isDataDirectoryWritable(customDir) {
			isValid = true
		}
	}

	return DataDirectoryInfo{
		CurrentDataDir:   currentDir,
		DefaultDataDir:   defaultDir,
		CustomDataDir:    customDir,
		UseCustomDataDir: config.UseCustomDataDir,
		IsCustomDirValid: isValid,
	}
}

// DataDirectorySelectDirectory 选择新的数据目录
func (a *App) DataDirectorySelectDirectory() DataDirectorySelectResult {
	if a == nil || a.ctx == nil {
		return DataDirectorySelectResult{Selected: false}
	}

	directory, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "选择数据目录",
	})

	if err != nil || strings.TrimSpace(directory) == "" {
		return DataDirectorySelectResult{Selected: false}
	}

	return DataDirectorySelectResult{
		Selected:  true,
		Directory: directory,
	}
}

// DataDirectorySetCustom 设置自定义数据目录
func (a *App) DataDirectorySetCustom(directory string, useCustom bool) (DataDirectoryInfo, error) {
	if a == nil {
		return DataDirectoryInfo{}, fmt.Errorf("应用未初始化")
	}

	directory = NormalizeCustomDataDir(a.appRoot, directory)

	if useCustom && directory == "" {
		return DataDirectoryInfo{}, fmt.Errorf("自定义数据目录路径不能为空")
	}

	if useCustom {
		// 受保护安装目录只有管理员进程可写，普通启动会回退到默认目录，
		// 所以即使当前进程可写也不能保存为数据目录。
		if IsProtectedInstallDataDir(a.appRoot, directory) {
			directory = RuntimeDefaultStateRoot(a.appRoot)
		}
		// 验证目录是否存在
		info, err := os.Stat(directory)
		if err != nil {
			if os.IsNotExist(err) {
				// 目录不存在，尝试创建
				if err := os.MkdirAll(directory, 0755); err != nil {
					return DataDirectoryInfo{}, fmt.Errorf("创建数据目录失败: %w", err)
				}
			} else {
				return DataDirectoryInfo{}, fmt.Errorf("访问数据目录失败: %w", err)
			}
		} else if !info.IsDir() {
			return DataDirectoryInfo{}, fmt.Errorf("指定路径不是目录: %s", directory)
		}
		if !isDataDirectoryWritable(directory) {
			if fallbackDir, ok := writableDefaultForInstallDir(a.appRoot, directory, isDataDirectoryWritable); ok {
				directory = fallbackDir
			} else {
				return DataDirectoryInfo{}, fmt.Errorf(
					"自定义数据目录不可写：%s。请改选当前用户可写的目录，例如：%s",
					directory,
					RuntimeDefaultStateRoot(a.appRoot),
				)
			}
		}
	}

	config := DataDirConfig{
		CustomDataDir:    directory,
		UseCustomDataDir: useCustom,
	}

	if err := SaveDataDirConfigForAppRoot(a.appRoot, config); err != nil {
		return DataDirectoryInfo{}, fmt.Errorf("保存数据目录配置失败: %w", err)
	}

	// 当前进程已经打开了旧状态目录中的数据库。只保存下次启动的选择，
	// 由 main 在初始化数据库前设置全局路径，避免运行中把数据写到两处。
	return a.DataDirectoryGetInfo(), nil
}

// writableDefaultForInstallDir redirects an unwritable selection of the
// application install directory to the normal writable state directory. This
// covers the common case where the user selects Program Files as the data
// directory, while keeping arbitrary unwritable custom paths as errors.
func writableDefaultForInstallDir(appRoot, requested string, writable func(string) bool) (string, bool) {
	installDir := apppath.InstallRoot(appRoot)
	return writableDefaultForInstallPath(installDir, RuntimeDefaultStateRoot(appRoot), requested, writable)
}

// IsProtectedInstallDataDir reports whether dir lies inside an install
// directory that normal user processes cannot write, such as Program Files.
// An elevated process (for example one launched from the installer finish
// page) can write there, so writability alone must not accept it.
func IsProtectedInstallDataDir(appRoot, dir string) bool {
	dir = strings.TrimSpace(dir)
	if dir == "" || !RuntimeUsesDetachedState(appRoot) {
		return false
	}
	return dataDirectoryPathEqualOrWithin(dir, apppath.InstallRoot(appRoot))
}

func writableDefaultForInstallPath(installDir, defaultDir, requested string, writable func(string) bool) (string, bool) {
	if !sameDataDirectoryPath(requested, installDir) {
		return "", false
	}
	if sameDataDirectoryPath(defaultDir, installDir) || !writable(defaultDir) {
		return "", false
	}
	return defaultDir, true
}

func isDataDirectoryWritable(directory string) bool {
	testFile, err := os.CreateTemp(directory, ".lianxiang-browser-write-test-*")
	if err != nil {
		return false
	}
	name := testFile.Name()
	closeErr := testFile.Close()
	removeErr := os.Remove(name)
	return closeErr == nil && removeErr == nil
}

func sameDataDirectoryPath(left, right string) bool {
	leftAbs, leftErr := filepath.Abs(filepath.Clean(left))
	rightAbs, rightErr := filepath.Abs(filepath.Clean(right))
	if leftErr != nil || rightErr != nil {
		return filepath.Clean(left) == filepath.Clean(right)
	}
	return strings.EqualFold(leftAbs, rightAbs)
}

func dataDirectoryPathEqualOrWithin(path, root string) bool {
	pathAbs, pathErr := filepath.Abs(filepath.Clean(path))
	rootAbs, rootErr := filepath.Abs(filepath.Clean(root))
	if pathErr != nil || rootErr != nil {
		return false
	}
	rel, err := filepath.Rel(rootAbs, pathAbs)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

// DataDirectoryMigrate 迁移数据到新目录
func (a *App) DataDirectoryMigrate(options DataDirectoryMigrateOptions) DataDirectoryMigrateResult {
	if a == nil {
		return DataDirectoryMigrateResult{
			Success: false,
			Error:   "应用未初始化",
		}
	}

	sourceDir := strings.TrimSpace(options.SourceDir)
	targetDir := strings.TrimSpace(options.TargetDir)

	if sourceDir == "" || targetDir == "" {
		return DataDirectoryMigrateResult{
			Success: false,
			Error:   "源目录和目标目录不能为空",
		}
	}

	if dataDirectoryPathEqualOrWithin(targetDir, sourceDir) && !sameDataDirectoryPath(targetDir, sourceDir) {
		return DataDirectoryMigrateResult{Success: false, Error: "目标目录不能位于源目录内部"}
	}

	// 验证源目录存在
	if _, err := os.Stat(sourceDir); err != nil {
		return DataDirectoryMigrateResult{
			Success: false,
			Error:   fmt.Sprintf("源目录不存在: %v", err),
		}
	}

	// 确保目标目录存在
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return DataDirectoryMigrateResult{
			Success: false,
			Error:   fmt.Sprintf("创建目标目录失败: %v", err),
		}
	}

	// 迁移 data 目录
	if options.MigrateData {
		dataSource := filepath.Join(sourceDir, "data")
		dataTarget := filepath.Join(targetDir, "data")
		if err := copyDirectoryIfExists(dataSource, dataTarget); err != nil {
			return DataDirectoryMigrateResult{
				Success: false,
				Error:   fmt.Sprintf("迁移 data 目录失败: %v", err),
			}
		}
	}

	// 迁移 chrome 目录
	if options.MigrateChrome {
		chromeSource := filepath.Join(sourceDir, "chrome")
		chromeTarget := filepath.Join(targetDir, "chrome")
		if err := copyDirectoryIfExists(chromeSource, chromeTarget); err != nil {
			return DataDirectoryMigrateResult{
				Success: false,
				Error:   fmt.Sprintf("迁移 chrome 目录失败: %v", err),
			}
		}
	}

	// 迁移配置文件
	if options.MigrateConfig {
		configFiles := []string{"config.yaml", "proxies.yaml", "backup.local.yaml"}
		for _, file := range configFiles {
			sourceFile := filepath.Join(sourceDir, file)
			targetFile := filepath.Join(targetDir, file)
			if err := copyFileIfExists(sourceFile, targetFile); err != nil {
				return DataDirectoryMigrateResult{
					Success: false,
					Error:   fmt.Sprintf("迁移配置文件 %s 失败: %v", file, err),
				}
			}
		}
	}

	return DataDirectoryMigrateResult{
		Success: true,
		Message: "数据迁移成功",
	}
}

// copyDirectoryIfExists 复制目录（如果存在）
func copyDirectoryIfExists(src, dst string) error {
	// 检查源目录是否存在
	srcInfo, err := os.Stat(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // 源目录不存在，跳过
		}
		return err
	}

	if !srcInfo.IsDir() {
		return fmt.Errorf("源路径不是目录: %s", src)
	}

	// 创建目标目录
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}

	// 遍历复制
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 计算相对路径
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		targetPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(targetPath, info.Mode())
		}

		return dataDirCopyFile(path, targetPath)
	})
}

// copyFileIfExists 复制文件（如果存在）
func copyFileIfExists(src, dst string) error {
	// 检查源文件是否存在
	if _, err := os.Stat(src); err != nil {
		if os.IsNotExist(err) {
			return nil // 源文件不存在，跳过
		}
		return err
	}

	return dataDirCopyFile(src, dst)
}

// copyFile 复制文件
func dataDirCopyFile(src, dst string) error {
	sourceData, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	return os.WriteFile(dst, sourceData, 0644)
}
