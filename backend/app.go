package backend

import (
	"lianxiang-browser/backend/internal/automation"
	"lianxiang-browser/backend/internal/browser"
	"lianxiang-browser/backend/internal/config"
	"lianxiang-browser/backend/internal/database"
	"lianxiang-browser/backend/internal/launchcode"
	"lianxiang-browser/backend/internal/logger"
	"lianxiang-browser/backend/internal/proxy"
	"context"
	"strings"
	"sync"
	"time"
)

type quitMode uint8

const (
	quitModeFull quitMode = iota
	quitModeAppOnly
)

// App 应用结构体
type App struct {
	ctx             context.Context
	config          *config.Config
	db              *database.DB
	interceptor     *logger.MethodInterceptor
	browserMgr      *browser.Manager
	xrayMgr         *proxy.XrayManager
	clashMgr        *proxy.ClashManager
	singboxMgr      *proxy.SingBoxManager
	launchCodeSvc   *launchcode.LaunchCodeService
	launchServer    *launchcode.LaunchServer
	automationMgr   *automation.Manager
	speedScheduler  *browser.ProxySpeedScheduler
	backupScheduler *backupScheduler
	appRoot         string
	version         string

	forceQuit                     bool
	quitMode                      quitMode
	quitMu                        sync.RWMutex
	runtimeMu                     sync.RWMutex
	runtimeStopped                bool
	backgroundTaskCtx             context.Context
	backgroundTaskCancel          context.CancelFunc
	backgroundTasks               sync.WaitGroup
	backgroundTasksBlocked        bool
	maintenanceMu                 sync.Mutex
	bridgeMu                      sync.Mutex
	profileBridgeRefs             map[string]profileProxyBridgeRef
	deferredStartTargetsMu        sync.Mutex
	deferredStartTargets          map[string]deferredStartTargetsPlan
	automationTargetMu            sync.Mutex
	automationTargetCursor        map[string]string
	profileWindowMarkersMu        sync.Mutex
	profileWindowMarkers          map[string]*profileWindowMarker
	browserProcessMonitors        map[string]*browserProcessMonitor
	backupLocalConfigPathOverride string
	stopServicesOnce              sync.Once
	finalizeOnce                  sync.Once
	// ready 在启动完成核心初始化（数据库、管理器、启动码服务）后关闭。
	// Wails 在 OnStartup 返回前就会分发前端的绑定调用，列表类接口据此等待，
	// 避免前端拿到空列表后反复轮询。
	ready     chan struct{}
	readyOnce sync.Once
}

// NewApp 创建新的应用实例
func NewApp(appRoot string, appVersion ...string) *App {
	version := ""
	if len(appVersion) > 0 {
		version = strings.TrimSpace(appVersion[0])
	}
	return &App{
		appRoot:                strings.TrimSpace(appRoot),
		version:                version,
		profileBridgeRefs:      make(map[string]profileProxyBridgeRef),
		deferredStartTargets:   make(map[string]deferredStartTargetsPlan),
		automationTargetCursor: make(map[string]string),
		profileWindowMarkers:   make(map[string]*profileWindowMarker),
		browserProcessMonitors: make(map[string]*browserProcessMonitor),
		ready:                  make(chan struct{}),
	}
}

// startupReadyWait 是列表接口等待后端初始化的上限。超时后按原逻辑返回空结果，
// 前端仍会在 app:ready 事件到达时刷新。
const startupReadyWait = 20 * time.Second

func (a *App) markReady() {
	if a == nil {
		return
	}
	a.readyOnce.Do(func() {
		if a.ready != nil {
			close(a.ready)
		}
		a.emitRuntimeEvent("app:ready")
	})
}

// waitReady 阻塞直到启动完成或超时，返回是否已就绪。
// 未通过 NewApp 创建的 App（例如测试中直接构造）视为已就绪。
func (a *App) waitReady(timeout time.Duration) bool {
	if a == nil || a.ready == nil {
		return true
	}
	select {
	case <-a.ready:
		return true
	default:
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-a.ready:
		return true
	case <-timer.C:
		return false
	}
}

func (a *App) appName() string {
	if a.config != nil {
		if name := strings.TrimSpace(a.config.App.Name); name != "" {
			return name
		}
	}
	return "链享浏览器"
}

func (a *App) appVersion() string {
	version := strings.TrimSpace(a.version)
	if version == "" {
		return "unknown"
	}
	return version
}
