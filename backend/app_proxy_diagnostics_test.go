package backend

import (
	"testing"

	"lianxiang-browser/backend/internal/browser"
	"lianxiang-browser/backend/internal/config"
	"lianxiang-browser/backend/internal/proxy"
)

func TestBrowserProxyBuildDiagnosticUsesConfiguredConnectorStack(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Browser.DefaultConnectorType = config.BrowserConnectorMihomo
	app := NewApp(t.TempDir())
	app.config = cfg
	app.browserMgr = browser.NewManager(cfg, app.appRoot)

	result := app.BrowserProxyBuildDiagnostic("", "name: wg\n"+
		"type: wireguard\n"+
		"server: wg.example.com\n"+
		"port: 51820\n"+
		"ip: 172.16.0.2\n"+
		"private-key: private\n"+
		"public-key: public\n")

	if !result.Ok {
		t.Fatalf("diagnostic failed: %+v", result)
	}
	if result.Engine != proxy.ProxyKernelMihomo {
		t.Fatalf("diagnostic engine = %q, want %q; result=%+v", result.Engine, proxy.ProxyKernelMihomo, result)
	}
}
