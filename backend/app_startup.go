package backend

import (
	"context"
	"database/sql"
	"fmt"
	"io"
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
	startedAt := time.Now()
	phases := newStartupPhases()
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

	phases.mark("layout_config")

	log := a.startupInitLogger(ctx, cfg)
	if err := killResidualRuntimeProcesses(a.appRoot); err != nil {
		log.Warn("清理上次异常退出遗留的代理进程失败", logger.F("error", err))
	}
	phases.mark("residual_processes")
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
	phases.mark("backup_cores_restore")

	db, recovery, err := a.startupInitDatabase(cfg)
	if err != nil {
		log.Error("初始化数据库失败", logger.F("error", err))
		runtime.LogFatal(ctx, fmt.Sprintf("初始化数据库失败: %v", err))
		return
	}
	a.db = db
	a.reportDatabaseRecovery(log, recovery)
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
	if report, reconcileErr := db.Reconcile(a.resolveAppPath("data/db-backups")); reconcileErr != nil {
		log.Warn("数据库结构自愈对齐失败", logger.F("error", reconcileErr.Error()))
	} else if len(report.AddedColumns) > 0 || len(report.AddedTables) > 0 {
		log.Info("数据库结构已对齐", logger.F("report", report))
	}
	phases.mark("database")

	if err := a.startupInitManagers(cfg, db); err != nil {
		log.Error("浏览器数据初始化失败", logger.F("error", err))
		_ = db.Close()
		runtime.LogFatal(ctx, fmt.Sprintf("浏览器数据初始化失败: %v", err))
		return
	}
	phases.mark("managers")
	a.startupInitLaunchCode(log)
	a.startupInitLaunchServer(log)
	a.startupInitAutomation()
	a.startupInitBridgeHooks()
	a.startupInitSpeedScheduler()
	a.startupInitBackupScheduler()
	phases.mark("services")
	a.markReady()
	log.Info("应用启动成功",
		logger.F("elapsed_ms", time.Since(startedAt).Milliseconds()),
		logger.F("phases_ms", phases.String()),
	)

	// 回收站清理不影响首屏数据，放到就绪之后执行，避免与前端首次加载争用
	// SQLite 单连接和 Manager 锁。
	a.startupScheduleTrashCleanup()
}

// startupPhases 记录各启动阶段耗时，写入“应用启动成功”日志，便于定位启动变慢的阶段。
type startupPhases struct {
	last  time.Time
	parts []string
}

func newStartupPhases() *startupPhases {
	return &startupPhases{last: time.Now()}
}

func (p *startupPhases) mark(name string) {
	now := time.Now()
	p.parts = append(p.parts, fmt.Sprintf("%s=%d", name, now.Sub(p.last).Milliseconds()))
	p.last = now
}

func (p *startupPhases) String() string {
	return strings.Join(p.parts, " ")
}

// startupTrashCleanupDelay 推迟回收站清理：过期实例的数据目录可能很大，
// 删除期间会持有 Manager 锁，放在首屏加载之后执行。
const startupTrashCleanupDelay = 30 * time.Second

func (a *App) startupScheduleTrashCleanup() {
	mgr := a.browserMgr
	if mgr == nil {
		return
	}
	_ = a.startBackgroundTask(func(ctx context.Context) {
		timer := time.NewTimer(startupTrashCleanupDelay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		if err := mgr.CleanupExpiredTrash(); err != nil {
			logger.New("Browser").Error("启动清理回收站失败", logger.F("error", err))
		}
	})
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

// copyRuntimeFile 以流式方式复制数据库等大文件，避免整个文件读入内存。
func copyRuntimeFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
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

func (a *App) startupInitDatabase(cfg *config.Config) (*database.DB, database.RecoveryInfo, error) {
	return database.OpenWithRecovery(
		a.resolveAppPath(cfg.Database.SQLite.Path),
		a.resolveAppPath("data/db-backups"),
	)
}

// reportDatabaseRecovery 把数据库自愈结果写进日志并广播给前端，避免数据被重置后无人知晓。
func (a *App) reportDatabaseRecovery(log *logger.Logger, recovery database.RecoveryInfo) {
	if recovery.MigrationBackup != "" {
		log.Info("数据库升级前已创建备份", logger.F("path", recovery.MigrationBackup))
	}
	if !recovery.Recovered {
		return
	}
	log.Error("数据库无法使用，已自动备份并重建空库（历史实例/代理/插件数据未恢复）",
		logger.F("reason", recovery.Reason),
		logger.F("backup", recovery.BrokenBackupPath),
		logger.F("parked", recovery.BrokenParkedPath))
	a.emitRuntimeEvent("app:db-recovered", map[string]any{
		"reason":      recovery.Reason,
		"backup_path": recovery.BrokenBackupPath,
		"parked_path": recovery.BrokenParkedPath,
		"recovered":   true,
	})
}

func (a *App) startupInitManagers(cfg *config.Config, db *database.DB) error {
	a.browserMgr = browser.NewManager(cfg, a.appRoot)
	a.browserMgr.EventEmitter = a.emitRuntimeEvent
	a.browserMgr.ProfileRuntimeDetector = func(profile *browser.Profile) bool {
		return browserProfileHasRuntime(a, profile, nil)
	}
	a.xrayMgr = proxy.NewXrayManager(cfg, a.appRoot)
	a.clashMgr = proxy.NewClashManager(cfg, a.appRoot)
	a.singboxMgr = proxy.NewSingBoxManager(cfg, a.appRoot)
	a.mieruMgr = proxy.NewMieruManager(cfg, a.appRoot)
	a.browserMgr.CoreDownloadHTTPClientFactory = func(proxyConfig string, timeout time.Duration) (*http.Client, error) {
		client, _, err := a.unifiedProxyCoreHTTPClient(timeout, proxyConfig)
		return client, err
	}

	conn := db.GetConn()
	a.browserMgr.ProfileDAO = browser.NewSQLiteProfileDAO(conn)
	a.browserMgr.ProxyDAO = browser.NewSQLiteProxyDAO(conn)
	a.browserMgr.ProxyVirtualGroupDAO = browser.NewSQLiteProxyVirtualGroupDAO(conn)
	a.browserMgr.ProxyVirtualGroupMemberStateDAO = browser.NewSQLiteProxyVirtualGroupMemberStateDAO(conn)
	a.browserMgr.CoreDAO = browser.NewSQLiteCoreDAO(conn)
	a.browserMgr.BookmarkDAO = browser.NewSQLiteBookmarkDAO(conn)
	a.browserMgr.GroupDAO = browser.NewSQLiteGroupDAO(conn)
	a.browserMgr.ExtensionDAO = browser.NewSQLiteExtensionDAO(conn)

	if err := a.migrateToSQLite(); err != nil {
		return err
	}

	a.browserMgr.InitData()
	a.autoDetectCores()
	if err := a.loadProxies(); err != nil {
		return err
	}
	a.reconcileProfileProxyBindings()
	return nil
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
	if a.xrayMgr != nil {
		a.xrayMgr.OnBridgeDied = func(key string, err error) {
			a.handleProxyBridgeDied(profileProxyBridgeEngineXray, key, err)
		}
	}
	if a.singboxMgr != nil {
		a.singboxMgr.OnBridgeDied = func(key string, err error) {
			a.handleProxyBridgeDied(profileProxyBridgeEngineSingBox, key, err)
		}
	}
	if a.clashMgr != nil {
		a.clashMgr.OnNodeBridgeDied = func(key string, err error) {
			a.handleProxyBridgeDied(profileProxyBridgeEngineMihomo, key, err)
		}
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
	interval, failureRetry, schedulerConcurrency := a.proxySpeedSchedule()
	if a.speedScheduler == nil {
		a.speedScheduler = browser.NewProxySpeedSchedulerWithRetry(
			a.browserMgr.ProxyDAO,
			nil,
			interval,
			failureRetry,
			schedulerConcurrency,
		)
		a.speedScheduler.SetProxySpeedTestFunc(func(item browser.Proxy) (bool, int64, string) {
			result := a.testSharedProxySpeed(item.ProxyId, true)
			a.emitRuntimeEvent("proxy:speed:result", result)
			return result.Ok, result.LatencyMs, result.Error
		})
		a.speedScheduler.SetResultCurrentFunc(func(item browser.Proxy, proxies []browser.Proxy) bool {
			return a.proxySpeedResultIsCurrent(item, proxies)
		})
		a.speedScheduler.SetResultPersistence(false)
		a.speedScheduler.SetPriorityFunc(func(item browser.Proxy) int {
			return a.virtualGroupProbePriority(item.ProxyId)
		})
		a.speedScheduler.SetIntervalFunc(func(item browser.Proxy) time.Duration {
			return a.virtualGroupProbeIntervalFor(item.ProxyId)
		})
		a.speedScheduler.SetProbeGateFunc(func(item browser.Proxy, checkedAt time.Time) bool {
			return a.virtualGroupProbeGate(item.ProxyId, checkedAt)
		})
	}
	a.speedScheduler.Start()
}

func (a *App) proxySpeedSchedule() (time.Duration, time.Duration, int) {
	settings := config.DefaultConfig().ProxyCheck
	if a != nil && a.config != nil {
		settings = a.config.ProxyCheck
	}
	settings = proxy.NormalizeCheckSettings(settings)
	interval := time.Duration(settings.SpeedIntervalMinutes) * time.Minute
	failureRetry := time.Duration(settings.FailureRetryMinutes) * time.Minute
	concurrency := settings.SpeedConcurrency
	return interval, failureRetry, concurrency
}

func (a *App) restartSpeedScheduler() {
	if a == nil {
		return
	}
	if a.speedScheduler != nil {
		a.speedScheduler.Stop()
		a.speedScheduler = nil
	}
	a.ensureSpeedScheduler()
}
