package backend

import (
	"errors"
	"fmt"
	"os/exec"
	stdruntime "runtime"
	"strings"
	"time"

	"lianxiang-browser/backend/internal/logger"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	windowsStatusBreakpoint              = uint32(0x80000003)
	browserEarlyBreakpointRecoveryWindow = 2 * time.Minute
)

func (a *App) waitBrowserProcess(input browserStartInput, monitor *browserProcessMonitor) {
	profileId := input.ProfileID
	err := monitor.Wait()

	log := logger.New("Browser")
	debugPort := 0
	profileName := profileId
	shouldMonitorDetached := false

	a.browserMgr.Mutex.Lock()
	profile, exists := a.browserMgr.Profiles[profileId]
	wasRunning := exists && profile.Running
	if exists {
		profileName = profile.ProfileName
		debugPort = profile.DebugPort
	}
	a.browserMgr.Mutex.Unlock()

	if wasRunning && debugPort > 0 {
		snapshot, changed := a.waitForBrowserDebugReady(profileId, debugPort, browserLauncherDetachGraceWindow)
		if warningSnapshot, warningChanged := a.finalizeDeferredStartTargets(profileId, debugPort); warningSnapshot != nil {
			snapshot = warningSnapshot
			changed = changed || warningChanged
		}
		if snapshot != nil && changed {
			log.Info("浏览器启动器进程退出后，调试接口延迟就绪",
				logger.F("profile_id", profileId),
				logger.F("debug_port", debugPort),
			)
			a.emitBrowserInstanceUpdated(snapshot)
		}

		a.browserMgr.Mutex.Lock()
		profile, exists = a.browserMgr.Profiles[profileId]
		if exists && profile.Running && profile.DebugPort == debugPort && profile.DebugReady && canConnectDebugPort(debugPort, 250*time.Millisecond) {
			delete(a.browserMgr.BrowserProcesses, profileId)
			profile.Pid = 0
			shouldMonitorDetached = true
		}
		a.browserMgr.Mutex.Unlock()
		if shouldMonitorDetached {
			log.Info("浏览器启动器进程已退出，切换为调试端口存活监控",
				logger.F("profile_id", profileId),
				logger.F("profile_name", profileName),
				logger.F("debug_port", debugPort),
			)
			a.waitDetachedBrowser(profileId, debugPort)
			return
		}
	}

	processRuntime := monitor.Runtime()
	shouldRetryBreakpoint := shouldRetryEarlyBrowserBreakpoint(
		err,
		processRuntime,
		!input.breakpointRetryDone,
	)

	a.browserMgr.Mutex.Lock()
	profile, exists = a.browserMgr.Profiles[profileId]
	wasRunning = exists && profile.Running
	if exists {
		profileName = profile.ProfileName
		a.markProfileStoppedLocked(profileId, profile)
		if wasRunning && err != nil && !shouldRetryBreakpoint {
			profile.LastError = fmt.Sprintf("实例运行异常退出：%s", err.Error())
		}
	}
	a.browserMgr.Mutex.Unlock()

	if wasRunning && err != nil && shouldRetryBreakpoint {
		log.Warn("浏览器启动后触发断点异常，尝试自动恢复",
			logger.F("profile_id", profileId),
			logger.F("profile_name", profileName),
			logger.F("runtime_ms", processRuntime.Milliseconds()),
			logger.F("error", err.Error()),
		)

		retryInput := input
		retryInput.breakpointRetryDone = true
		if _, retryErr := a.browserInstanceStartWithInput(retryInput); retryErr == nil {
			log.Info("浏览器断点异常自动恢复成功",
				logger.F("profile_id", profileId),
				logger.F("profile_name", profileName),
			)
			return
		} else {
			recoveryErr := fmt.Sprintf("%s；自动恢复失败：%s", err.Error(), retryErr.Error())
			a.browserMgr.Mutex.Lock()
			if current, ok := a.browserMgr.Profiles[profileId]; ok && current != nil {
				current.LastError = fmt.Sprintf("实例运行异常退出：%s", recoveryErr)
			}
			a.browserMgr.Mutex.Unlock()

			log.Error("浏览器断点异常自动恢复失败",
				logger.F("profile_id", profileId),
				logger.F("profile_name", profileName),
				logger.F("error", recoveryErr),
			)
			if a.ctx != nil {
				runtime.EventsEmit(a.ctx, "browser:instance:crashed", map[string]interface{}{
					"profileId":   profileId,
					"profileName": profileName,
					"error":       recoveryErr,
				})
			}
			return
		}
	}

	if wasRunning && err != nil {
		log.Error("浏览器进程异常退出", logger.F("profile_id", profileId), logger.F("profile_name", profileName), logger.F("error", err))
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "browser:instance:crashed", map[string]interface{}{
				"profileId":   profileId,
				"profileName": profileName,
				"error":       err.Error(),
			})
		}
	} else {
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "browser:instance:stopped", profileId)
		}
	}
}

func shouldRetryEarlyBrowserBreakpoint(err error, processRuntime time.Duration, retryAvailable bool) bool {
	return retryAvailable &&
		stdruntime.GOOS == "windows" &&
		processRuntime >= 0 &&
		processRuntime <= browserEarlyBreakpointRecoveryWindow &&
		isWindowsBreakpointExit(err)
}

func isWindowsBreakpointExit(err error) bool {
	if err == nil {
		return false
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ProcessState != nil {
		return uint32(exitErr.ExitCode()) == windowsStatusBreakpoint
	}

	return strings.Contains(strings.ToLower(err.Error()), "0x80000003")
}

func (a *App) waitDetachedBrowser(profileId string, debugPort int) {
	const (
		pollInterval = 500 * time.Millisecond
		maxMisses    = 3
	)

	log := logger.New("Browser")
	misses := 0
	for {
		if canConnectDebugPort(debugPort, 250*time.Millisecond) {
			misses = 0
			time.Sleep(pollInterval)
			continue
		}

		misses++
		if misses < maxMisses {
			time.Sleep(pollInterval)
			continue
		}

		profileName := profileId
		a.browserMgr.Mutex.Lock()
		profile, exists := a.browserMgr.Profiles[profileId]
		if !exists || !profile.Running || profile.DebugPort != debugPort {
			a.browserMgr.Mutex.Unlock()
			return
		}
		profileName = profile.ProfileName
		a.markProfileStoppedLocked(profileId, profile)
		a.browserMgr.Mutex.Unlock()

		log.Info("检测到浏览器调试端口关闭，实例已停止",
			logger.F("profile_id", profileId),
			logger.F("profile_name", profileName),
			logger.F("debug_port", debugPort),
		)
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "browser:instance:stopped", profileId)
		}
		return
	}
}
