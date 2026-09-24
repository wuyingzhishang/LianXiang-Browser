package backend

import (
	"context"
	"database/sql"
	"fmt"
	"lianxiang-browser/backend/internal/apppath"
	"lianxiang-browser/backend/internal/automation"
	"lianxiang-browser/backend/internal/browser"
	"lianxiang-browser/backend/internal/config"
	"lianxiang-browser/backend/internal/database"
	"lianxiang-browser/backend/internal/fsutil"
	"lianxiang-browser/backend/internal/launchcode"
	"lianxiang-browser/backend/internal/logger"
	"lianxiang-browser/backend/internal/proxy"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// startup 应用启动时调用
func (a *App) startup(ctx context.Context) {
	a.setRuntimeContext(ctx)
	if err := apppath.EnsureWritableLayout(a.appRoot); err != nil {
		runtime.LogFatal(ctx, fmt.Sprintf("初始化 Linux 用户数据目录失败: %v", err))
		return
	}
	if err := a.restoreCustomDataDirectory(); err != nil {
		logger.New("App").Warn("自定义数据目录迁移补偿失败", logger.F("error", err))
	}

	cfg := a.startupLoadConfig()
	a.config = cfg
	a.applyRuntimeConfig(cfg.Runtime)

	log := a.startupInitLogger(ctx, cfg)
	if err := killResidualRuntimeProcesses(a.appRoot); err != nil {
		log.Warn("清理上次异常退出遗留的代理进程失败", logger.F("error", err))
	}
	a.startupLogEnvironment(log, cfg)
	a.activateStableBackupLocalConfig()
	if err := a.prepareBackupLocalConfig(); err != nil {
		log.Error("本地备份配置初始化失败", logger.F("error", err))
	}

	if err := os.MkdirAll(a.resolveAppPath("data"), 0o755); err != nil {
		log.Error("创建 data 目录失败", logger.F("error", err))
	}

	a.ensureDefaultCores()
	if err := a.restoreDetachedInstallDatabase(cfg); err != nil {
		log.Warn("安装目录数据库迁移失败", logger.F("error", err))
	}
	a.startupInitInterceptor(log, cfg)

	db, err := a.startupInitDatabase(cfg)
	if err != nil {
		log.Error("初始化数据库失败", logger.F("error", err))
		runtime.LogFatal(ctx, fmt.Sprintf("初始化数据库失败: %v", err))
		return
	}
	a.db = db
	if apppath.IsDetached(a.appRoot) {
		if profiles, queryErr := countDatabaseRows(db.GetConn(), "browser_profiles"); queryErr == nil {
			log.Info("运行时数据库已打开", logger.F("path", a.resolveAppPath(cfg.Database.SQLite.Path)), logger.F("profiles", profiles))
		} else {
			log.Warn("读取运行时实例数量失败", logger.F("path", a.resolveAppPath(cfg.Database.SQLite.Path)), logger.F("error", queryErr))
		}
	}
	if err := db.Migrate(); err != nil {
		log.Error("数据库迁移失败", logger.F("error", err))
	}

	a.startupInitManagers(cfg, db)
	a.startupInitLaunchCode(log)
	a.startupInitLaunchServer(log)
	a.startupInitAutomation()
	a.startupInitBridgeHooks()
	a.startupInitSpeedScheduler()
	a.startupInitBackupScheduler()
	a.emitRuntimeEvent("app:ready")

	log.Info("应用启动成功")
}

// restoreCustomDataDirectory repairs the upgrade path where an older build
// created an empty SQLite file in the selected directory before the packaged
// data was copied. A populated source database remains authoritative while a
// populated custom database is always preserved.
func (a *App) restoreCustomDataDirectory() error {
	if a == nil || strings.TrimSpace(apppath.GetCustomDataDir()) == "" {
		return nil
	}
	targetRoot := a.appStateRootAbs()
	sources := []string{RuntimeDefaultStateRoot(a.appRoot), a.appRootAbs()}
	for _, sourceRoot := range sources {
		sourceRoot = filepath.Clean(strings.TrimSpace(sourceRoot))
		if sourceRoot == "" || strings.EqualFold(sourceRoot, filepath.Clean(targetRoot)) {
			continue
		}
		sourceDB := filepath.Join(sourceRoot, "data", "app.db")
		sourceCount, err := countSQLiteRows(sourceDB, "browser_profiles")
		if err != nil || sourceCount == 0 {
			continue
		}
		targetDB := filepath.Join(targetRoot, "data", "app.db")
		targetCount, targetErr := countSQLiteRows(targetDB, "browser_profiles")
		if targetErr != nil && !os.IsNotExist(targetErr) {
			return targetErr
		}
		if targetErr == nil && targetCount > 0 {
			return nil
		}
		if err := copyRuntimeFile(sourceDB, targetDB); err != nil {
			return err
		}
		for _, suffix := range []string{"-wal", "-shm"} {
			source := sourceDB + suffix
			target := targetDB + suffix
			if _, statErr := os.Stat(source); statErr == nil {
				if err := copyRuntimeFile(source, target); err != nil {
					return err
				}
			} else if !os.IsNotExist(statErr) {
				return statErr
			}
		}
		return nil
	}
	return nil
}

// restoreDetachedInstallDatabase recovers profiles from an older install when
// the first detached launch created an empty state database before migration.
// It only replaces a state database with zero profiles; an existing populated
// state database always wins.
func (a *App) restoreDetachedInstallDatabase(cfg *config.Config) error {
	if a == nil || cfg == nil || !apppath.IsDetached(a.appRoot) {
		return nil
	}

	dbPath := strings.TrimSpace(cfg.Database.SQLite.Path)
	if dbPath == "" || filepath.IsAbs(dbPath) {
		return nil
	}
	relativePath := fsutil.NormalizePathInput(dbPath)
	if relativePath == "" {
		return nil
	}
	installPath := filepath.Join(apppath.InstallRoot(a.appRoot), relativePath)
	statePath := a.resolveAppPath(relativePath)
	if strings.EqualFold(installPath, statePath) {
		return nil
	}
	if _, err := os.Stat(installPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	installCount, err := countSQLiteRows(installPath, "browser_profiles")
	if err != nil || installCount == 0 {
		return err
	}
	if _, err := os.Stat(statePath); err == nil {
		stateCount, countErr := countSQLiteRows(statePath, "browser_profiles")
		if countErr != nil {
			return countErr
		}
		if stateCount > 0 {
			return nil
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	if err := copyRuntimeFile(installPath, statePath); err != nil {
		return err
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		source := installPath + suffix
		target := statePath + suffix
		if _, err := os.Stat(source); err == nil {
			if err := copyRuntimeFile(source, target); err != nil {
				return err
			}
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func countSQLiteRows(path, table string) (int, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return 0, err
	}
	defer db.Close()
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "no such table") {
			return 0, nil
		}
		return 0, err
	}
	return count, nil
}

func countDatabaseRows(db *sql.DB, table string) (int, error) {
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func copyRuntimeFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

func (a *App) startupLoadConfig() *config.Config {
	cfg, err := LoadConfig(a.resolveAppPath("config.yaml"))
	if err != nil {
		return config.DefaultConfig()
	}
	return cfg
}

func (a *App) startupInitLogger(ctx context.Context, cfg *config.Config) *logger.Logger {
	logConfig := logger.LoggerConfig{
		Level:           cfg.Logging.Level,
		FileEnabled:     cfg.Logging.FileEnabled,
		FilePath:        a.resolveAppPath(cfg.Logging.FilePath),
		Format:          cfg.Logging.Format,
		BufferSize:      cfg.Logging.BufferSize,
		AsyncQueueSize:  cfg.Logging.AsyncQueueSize,
		FlushIntervalMs: cfg.Logging.FlushIntervalMs,
		Rotation: logger.RotationConfig{
			Enabled:      cfg.Logging.Rotation.Enabled,
			MaxSizeMB:    cfg.Logging.Rotation.MaxSizeMB,
			MaxAge:       cfg.Logging.Rotation.MaxAge,
			MaxBackups:   cfg.Logging.Rotation.MaxBackups,
			TimeInterval: cfg.Logging.Rotation.TimeInterval,
		},
	}
	logger.InitWithConfig(ctx, logConfig)
	return logger.New("App")
}

func (a *App) startupLogEnvironment(log *logger.Logger, cfg *config.Config) {
	log.Info("应用启动中...",
		logger.F("version", a.appVersion()),
		logger.F("max_memory_mb", cfg.Runtime.MaxMemoryMB),
		logger.F("gc_percent", cfg.Runtime.GCPercent),
	)
	if apppath.IsDetached(a.appRoot) {
		log.Info("检测到安装目录需要只读运行，已切换到用户数据目录",
			logger.F("install_root", apppath.InstallRoot(a.appRoot)),
			logger.F("state_root", apppath.StateRoot(a.appRoot)),
		)
	}
}

func (a *App) startupInitInterceptor(log *logger.Logger, cfg *config.Config) {
	if !cfg.Logging.Interceptor.Enabled {
		return
	}
	interceptorConfig := logger.InterceptorConfig{
		Enabled:         cfg.Logging.Interceptor.Enabled,
		LogParameters:   cfg.Logging.Interceptor.LogParameters,
		LogResults:      cfg.Logging.Interceptor.LogResults,
		SensitiveFields: cfg.Logging.Interceptor.SensitiveFields,
	}
	a.interceptor = logger.NewMethodInterceptor(log, interceptorConfig)
}

func (a *App) startupInitDatabase(cfg *config.Config) (*database.DB, error) {
	return database.NewDB(a.resolveAppPath(cfg.Database.SQLite.Path))
}

func (a *App) startupInitManagers(cfg *config.Config, db *database.DB) {
	a.browserMgr = browser.NewManager(cfg, a.appRoot)
	a.browserMgr.EventEmitter = a.emitRuntimeEvent
	a.xrayMgr = proxy.NewXrayManager(cfg, a.appRoot)
	a.clashMgr = proxy.NewClashManager(cfg, a.appRoot)
	a.singboxMgr = proxy.NewSingBoxManager(cfg, a.appRoot)
	a.browserMgr.CoreDownloadHTTPClientFactory = func(proxyConfig string, timeout time.Duration) (*http.Client, error) {
		client, _, err := a.unifiedProxyCoreHTTPClient(timeout, proxyConfig)
		return client, err
	}

	conn := db.GetConn()
	a.browserMgr.ProfileDAO = browser.NewSQLiteProfileDAO(conn)
	a.browserMgr.ProxyDAO = browser.NewSQLiteProxyDAO(conn)
	a.browserMgr.CoreDAO = browser.NewSQLiteCoreDAO(conn)
	a.browserMgr.BookmarkDAO = browser.NewSQLiteBookmarkDAO(conn)
	a.browserMgr.GroupDAO = browser.NewSQLiteGroupDAO(conn)
	a.browserMgr.ExtensionDAO = browser.NewSQLiteExtensionDAO(conn)

	a.migrateToSQLite()

	a.browserMgr.InitData()
	if err := a.browserMgr.CleanupExpiredTrash(); err != nil {
		logger.New("Browser").Error("启动清理回收站失败", logger.F("error", err))
	}
	a.autoDetectCores()
	a.loadProxies()
	a.reconcileProfileProxyBindings()
}

func (a *App) startupInitLaunchCode(log *logger.Logger) {
	launchCodeDAO := launchcode.NewSQLiteLaunchCodeDAO(a.db.GetConn())
	a.launchCodeSvc = launchcode.NewLaunchCodeService(launchCodeDAO)
	if err := a.launchCodeSvc.LoadAll(); err != nil {
		log.Error("LaunchCode 加载失败", logger.F("error", err))
	}
	a.browserMgr.CodeProvider = a.launchCodeSvc
}

func (a *App) startupInitLaunchServer(log *logger.Logger) {
	port := a.config.LaunchServer.Port
	a.launchServer = launchcode.NewLaunchServer(a.launchCodeSvc, a, a.browserMgr, port)
	a.launchServer.SetAPIAuthConfig(launchcode.APIAuthConfig{
		Enabled: a.config.LaunchServer.Auth.Enabled,
		APIKey:  a.config.LaunchServer.Auth.APIKey,
		Header:  a.config.LaunchServer.Auth.Header,
	})
	if err := a.launchServer.Start(); err != nil {
		log.Error("LaunchServer 启动失败", logger.F("error", err))
		return
	}
	log.Debug("LaunchServer 监听地址",
		logger.F("url", fmt.Sprintf("http://127.0.0.1:%d", a.launchServer.Port())),
		logger.F("preferred_port", port),
	)
}

func (a *App) startupInitAutomation() {
	a.automationMgr = automation.NewManager(a.appRoot, a.config, func(event string, payload any) {
		a.emitRuntimeEvent(event, payload)
	}, automation.Options{})
}

func (a *App) startupInitBridgeHooks() {
	a.xrayMgr.OnBridgeDied = func(key string, err error) {
		a.emitRuntimeEvent("proxy:bridge:died", map[string]interface{}{
			"engine": "xray",
			"key":    bridgeEventKey(key),
			"error":  errorMessage(err),
		})
	}
	a.singboxMgr.OnBridgeDied = func(key string, err error) {
		a.emitRuntimeEvent("proxy:bridge:died", map[string]interface{}{
			"engine": "singbox",
			"key":    bridgeEventKey(key),
			"error":  errorMessage(err),
		})
	}
}

func bridgeEventKey(key string) string {
	if len(key) <= 8 {
		return key
	}
	return key[:8]
}

func errorMessage(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func (a *App) startupInitSpeedScheduler() {
	a.ensureSpeedScheduler()
}

func (a *App) ensureSpeedScheduler() {
	if a == nil || a.config == nil || a.browserMgr == nil || a.browserMgr.ProxyDAO == nil {
		return
	}
	if a.speedScheduler == nil {
		a.speedScheduler = browser.NewProxySpeedScheduler(
			a.browserMgr.ProxyDAO,
			func(proxyId string) (bool, int64, string) {
				connectorType := config.NormalizeBrowserConnectorType(a.config.Browser.DefaultConnectorType)
				r := a.testProxySpeedWithConnector(proxyId, a.getLatestProxies(), connectorType)
				return r.Ok, r.LatencyMs, r.Error
			},
			browser.DefaultProxySpeedInterval,
			browser.DefaultProxySpeedConcurrency,
		)
	}
	a.speedScheduler.Start()
}
