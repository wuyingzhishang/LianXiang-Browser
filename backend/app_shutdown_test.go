package backend

import (
	"context"
	"lianxiang-browser/backend/internal/browser"
	"sync"
	"testing"
	"time"
)

func TestFullQuitAcknowledgesBeforeCleanupAndRunsOnce(t *testing.T) {
	app := NewApp(t.TempDir())
	app.setRuntimeContext(context.Background())
	releaseTask := make(chan struct{})
	defer close(releaseTask)
	if err := app.startBackgroundTask(func(ctx context.Context) { <-releaseTask }); err != nil {
		t.Fatal(err)
	}
	quit := make(chan struct{}, 2)
	ack := make(chan struct{})
	go func() {
		app.requestQuit(quitModeFull, func() { quit <- struct{}{} })
		app.requestQuit(quitModeAppOnly, func() { quit <- struct{}{} })
		close(ack)
	}()
	select {
	case <-ack:
	case <-time.After(time.Second):
		t.Fatal("quit request blocked on background cleanup")
	}
	if !app.shouldStopRuntimeServicesOnShutdown() {
		t.Fatal("repeated quit changed the selected mode")
	}
	select {
	case <-quit:
		t.Fatal("quit happened before cleanup completed")
	default:
	}
	releaseTask <- struct{}{}
	select {
	case <-quit:
	case <-time.After(time.Second):
		t.Fatal("quit did not finish after cleanup")
	}
	select {
	case <-quit:
		t.Fatal("duplicate quit callback")
	default:
	}
}

func TestAppOnlyQuitDoesNotStopBrowserRuntime(t *testing.T) {
	app := NewApp(t.TempDir())
	app.setRuntimeContext(context.Background())
	// If app-only quit enters full cleanup, it would wait for this task.
	releaseTask := make(chan struct{})
	defer close(releaseTask)
	if err := app.startBackgroundTask(func(ctx context.Context) { <-releaseTask }); err != nil {
		t.Fatal(err)
	}
	quit := make(chan struct{})
	app.requestQuit(quitModeAppOnly, func() { close(quit) })
	select {
	case <-quit:
	case <-time.After(time.Second):
		t.Fatal("app-only quit entered full runtime cleanup")
	}
	if app.shouldStopRuntimeServicesOnShutdown() {
		t.Fatal("app-only mode was not retained")
	}
}

func TestQuitStateTransitions(t *testing.T) {
	app := NewApp(t.TempDir())
	if app.isQuitRequested() {
		t.Fatal("new app unexpectedly has a quit request")
	}
	if !app.shouldStopRuntimeServicesOnShutdown() {
		t.Fatal("new app should stop runtime services on shutdown")
	}

	app.setQuitMode(quitModeAppOnly)
	if !app.isQuitRequested() {
		t.Fatal("app-only quit did not set the quit request")
	}
	if app.shouldStopRuntimeServicesOnShutdown() {
		t.Fatal("app-only quit should preserve runtime services")
	}

	app.setQuitMode(quitModeFull)
	if !app.shouldStopRuntimeServicesOnShutdown() {
		t.Fatal("full quit should stop runtime services")
	}
}

func TestQuitStateConcurrentAccess(t *testing.T) {
	app := NewApp(t.TempDir())
	var waitGroup sync.WaitGroup

	for i := 0; i < 8; i++ {
		waitGroup.Add(1)
		go func(index int) {
			defer waitGroup.Done()
			for iteration := 0; iteration < 1000; iteration++ {
				if index%2 == 0 {
					app.setQuitMode(quitModeFull)
				} else {
					app.setQuitMode(quitModeAppOnly)
				}
				_ = app.isQuitRequested()
				_ = app.shouldStopRuntimeServicesOnShutdown()
			}
		}(i)
	}

	waitGroup.Wait()
}

func TestStopAppOnlyRuntimeServicesStopsSpeedScheduler(t *testing.T) {
	app := NewApp(t.TempDir())
	scheduler := browser.NewProxySpeedScheduler(nil, nil, time.Hour, 1)
	scheduler.Start()
	app.speedScheduler = scheduler

	app.stopAppOnlyRuntimeServices()

	if app.speedScheduler != nil {
		t.Fatal("app-only shutdown retained the speed scheduler")
	}
}
