package backend

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	dataDirConfigFileName = "datadir.config.yaml"
	dataDirConfigDirName  = "链享浏览器"
)

// DataDirConfig 存储用户自定义的数据目录配置
type DataDirConfig struct {
	CustomDataDir    string `yaml:"custom_data_dir,omitempty"`
	UseCustomDataDir bool   `yaml:"use_custom_data_dir,omitempty"`
}

// defaultDataDirConfigPath 返回数据目录配置文件的默认路径
func defaultDataDirConfigPath() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	configDir = strings.TrimSpace(configDir)
	if configDir == "" {
		return ""
	}
	return filepath.Join(configDir, dataDirConfigDirName, dataDirConfigFileName)
}

// dataDirConfigPathForAppRoot keeps the selector config beside the default
// runtime state, so changing the runtime data root cannot hide the selector
// configuration from the next process.
func dataDirConfigPathForAppRoot(appRoot string) string {
	if root := strings.TrimSpace(RuntimeDefaultStateRoot(appRoot)); root != "" {
		return filepath.Join(root, dataDirConfigFileName)
	}
	return defaultDataDirConfigPath()
}

// dataDirConfigPathsForAppRoot returns every stable location used by the
// selector configuration.  The writable state root can change between
// launches (for example when the installer launches the first process with
// elevation), so keeping the install-root copy as a fallback prevents the
// custom directory choice from being lost on the next launch.
func dataDirConfigPathsForAppRoot(appRoot string) []string {
	candidates := []string{
		dataDirConfigPathForAppRoot(appRoot),
		filepath.Join(strings.TrimSpace(appRoot), dataDirConfigFileName),
		defaultDataDirConfigPath(),
	}
	result := make([]string, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if abs, err := filepath.Abs(candidate); err == nil {
			candidate = abs
		}
		candidate = filepath.Clean(candidate)
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		result = append(result, candidate)
	}
	return result
}

// LoadDataDirConfig 加载数据目录配置
func LoadDataDirConfig() (DataDirConfig, error) {
	path := defaultDataDirConfigPath()
	if path == "" {
		return DataDirConfig{}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DataDirConfig{}, nil
		}
		return DataDirConfig{}, fmt.Errorf("读取数据目录配置失败: %w", err)
	}

	var config DataDirConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return DataDirConfig{}, fmt.Errorf("解析数据目录配置失败: %w", err)
	}

	return config, nil
}

func loadDataDirConfigForAppRoot(appRoot string) (DataDirConfig, error) {
	for _, path := range dataDirConfigPathsForAppRoot(appRoot) {
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return DataDirConfig{}, fmt.Errorf("读取数据目录配置失败: %w", err)
		}
		var config DataDirConfig
		if err := yaml.Unmarshal(data, &config); err != nil {
			return DataDirConfig{}, fmt.Errorf("解析数据目录配置失败: %w", err)
		}
		return config, nil
	}
	return DataDirConfig{}, nil
}

// SaveDataDirConfigForAppRoot persists the selector in the current stable
// state location and, when possible, beside the executable as a migration
// fallback.  The latter is intentionally best effort because an installed
// application may live under a protected directory.
func SaveDataDirConfigForAppRoot(appRoot string, config DataDirConfig) error {
	paths := dataDirConfigPathsForAppRoot(appRoot)
	var firstErr error
	saved := false
	for _, path := range paths {
		if err := saveDataDirConfigAtPath(path, config); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		saved = true
	}
	if saved {
		return nil
	}
	if firstErr != nil {
		return firstErr
	}
	return fmt.Errorf("无法确定配置文件路径")
}

// LoadDataDirConfigForAppRoot loads the selector configuration before the
// application runtime is initialized.
func LoadDataDirConfigForAppRoot(appRoot string) (DataDirConfig, error) {
	return loadDataDirConfigForAppRoot(appRoot)
}

// SaveDataDirConfig 保存数据目录配置
func SaveDataDirConfig(config DataDirConfig) error {
	return saveDataDirConfigAtPath(defaultDataDirConfigPath(), config)
}

func saveDataDirConfigAtPath(path string, config DataDirConfig) error {
	if path == "" {
		return fmt.Errorf("无法确定配置文件路径")
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}

	return nil
}
