package main

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/oklog/ulid/v2"
	writekit "writekit"
	"writekit/internal/app"
	"writekit/internal/auth"
	"writekit/internal/config"
	"writekit/internal/desksettings"
	"writekit/internal/platform"
	"writekit/internal/tenant"
)

var (
	wailsCtx        context.Context
	settingsStore   *desksettings.Store
	resolvedDataDir string
	resolvedMCPURL  string
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	os.Setenv("LOCAL", "true")
	if os.Getenv("HOST") == "" {
		os.Setenv("HOST", "localhost")
	}

	if wakeExistingInstance() {
		slog.Info("existing instance found, asked it to surface")
		os.Exit(0)
	}

	listener, port, err := bindStablePort()
	if err != nil {
		slog.Error("bind loopback", "err", err)
		os.Exit(1)
	}
	os.Setenv("PORT", fmt.Sprintf("%d", port))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config", "err", err)
		os.Exit(1)
	}

	if s, err := desksettings.Open(); err == nil {
		settingsStore = s
		if loaded, err := s.Load(); err == nil {
			if os.Getenv("DATA_DIR") == "" && loaded.DataDir != "" {
				if err := os.MkdirAll(loaded.DataDir, 0755); err == nil {
					cfg.DataDir = loaded.DataDir
				} else {
					slog.Warn("custom data dir unusable, falling back to default", "dir", loaded.DataDir, "err", err)
				}
			}
			if !loaded.OnboardingComplete && dataDirLooksUsed(cfg.DataDir) {
				loaded.OnboardingComplete = true
				_ = s.Save(loaded)
			}
		}
	} else {
		slog.Warn("open settings store", "err", err)
	}

	resolvedDataDir = cfg.DataDir
	resolvedMCPURL = fmt.Sprintf("http://127.0.0.1:%d/mcp", port)

	if err := initWorkspaces(cfg.DataDir, settingsStore); err != nil {
		slog.Error("init workspaces", "err", err)
		os.Exit(1)
	}

	if err := writePortFile(port); err != nil {
		slog.Warn("write port file", "err", err)
	}

	desksettings.PickFolder = func(title string) (string, error) {
		if wailsCtx == nil {
			return "", fmt.Errorf("window not ready")
		}
		return wailsruntime.OpenDirectoryDialog(wailsCtx, wailsruntime.OpenDialogOptions{Title: title})
	}

	desksettings.ShowWindow = func() {
		if wailsCtx != nil {
			wailsruntime.WindowShow(wailsCtx)
			wailsruntime.WindowUnminimise(wailsCtx)
		}
	}

	pool, err := tenant.NewPool(cfg.DataDir, cfg.MaxPoolSize)
	if err != nil {
		slog.Error("create tenant pool", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	tFS, _ := fs.Sub(writekit.TemplatesFS, "templates")
	sFS, _ := fs.Sub(writekit.StaticFS, "static")
	application := app.New(cfg, (*platform.DB)(nil), pool, tFS, sFS, writekit.AppFS, writekit.AdminFS)

	server := &http.Server{
		Handler:           application.Router,
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		slog.Info("local server listening", "addr", listener.Addr().String())
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			slog.Error("server stopped", "err", err)
		}
	}()

	if !waitForReady(port, 3*time.Second) {
		slog.Warn("server did not become ready within 3s, loading webview anyway")
	}

	startTray(trayCallbacks{
		MCPURL: resolvedMCPURL,
		OnShow: func() {
			if wailsCtx != nil {
				wailsruntime.WindowShow(wailsCtx)
			}
		},
		OnCopyMCP: func() {
			if err := copyToClipboardOS(resolvedMCPURL); err != nil && wailsCtx != nil {
				wailsruntime.ClipboardSetText(wailsCtx, resolvedMCPURL)
			}
		},
		OnReveal: func() { revealInExplorer(resolvedDataDir) },
		OnQuit: func() {
			shutdownServer(server)
			os.Exit(0)
		},
	})

	proxyTarget, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", port))
	proxy := httputil.NewSingleHostReverseProxy(proxyTarget)

	startMinimized := false
	if settingsStore != nil {
		if s, err := settingsStore.Load(); err == nil {
			startMinimized = s.StartMinimized
		}
	}

	err = wails.Run(&options.App{
		Title:            "WriteKit",
		Width:            1280,
		Height:           860,
		MinWidth:         900,
		MinHeight:        600,
		StartHidden:      startMinimized,
		Frameless:        runtime.GOOS != "darwin",
		BackgroundColour: &options.RGBA{R: 250, G: 250, B: 250, A: 255},
		Mac: &mac.Options{
			TitleBar: mac.TitleBarHiddenInset(),
		},
		AssetServer: &assetserver.Options{
			Handler: proxy,
		},
		OnStartup: func(ctx context.Context) {
			wailsCtx = ctx
		},
		OnBeforeClose: func(ctx context.Context) bool {
			if settingsStore == nil {
				return false
			}
			s, err := settingsStore.Load()
			if err != nil || !s.CloseToTray {
				return false
			}
			wailsruntime.WindowHide(ctx)
			return true
		},
		OnShutdown: func(ctx context.Context) {
			shutdownServer(server)
		},
	})
	if err != nil {
		slog.Error("wails run", "err", err)
		os.Exit(1)
	}
}

func shutdownServer(server *http.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = server.Shutdown(ctx)
	quitTray()
}

func bindStablePort() (net.Listener, int, error) {
	for _, p := range []int{8787, 8788, 8789, 8790, 8791, 8792, 8793, 8794, 8795, 8796, 8797, 8798, 8799} {
		l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", p))
		if err == nil {
			return l, p, nil
		}
	}
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, 0, err
	}
	return l, l.Addr().(*net.TCPAddr).Port, nil
}

func writePortFile(port int) error {
	dir, err := os.UserConfigDir()
	if err != nil {
		return err
	}
	base := filepath.Join(dir, "WriteKit")
	if err := os.MkdirAll(base, 0755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(base, "port"), []byte(fmt.Sprintf("%d\n", port)), 0644)
}

func wakeExistingInstance() bool {
	port, err := readPortFile()
	if err != nil || port == 0 {
		return false
	}
	client := &http.Client{Timeout: 500 * time.Millisecond}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/api/local/info", port))
	if err != nil {
		return false
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false
	}
	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("http://127.0.0.1:%d/api/local/show", port), nil)
	if resp2, err := client.Do(req); err == nil {
		resp2.Body.Close()
	}
	return true
}

func readPortFile() (int, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return 0, err
	}
	b, err := os.ReadFile(filepath.Join(dir, "WriteKit", "port"))
	if err != nil {
		return 0, err
	}
	var p int
	if _, err := fmt.Sscanf(string(b), "%d", &p); err != nil {
		return 0, err
	}
	return p, nil
}

func initWorkspaces(dataDir string, store *desksettings.Store) error {
	var loaded desksettings.Settings
	if store != nil {
		if s, err := store.Load(); err == nil {
			loaded = s
		}
	}

	workspaces := loaded.Workspaces
	active := loaded.ActiveWorkspaceID

	if len(workspaces) == 0 {
		if _, err := os.Stat(filepath.Join(dataDir, "local.db")); err == nil {
			workspaces = []desksettings.Workspace{{ID: "local", Name: "My Site"}}
			active = "local"
		} else {
			id := ulid.Make().String()
			workspaces = []desksettings.Workspace{{ID: id, Name: "My Site"}}
			active = id
		}
	}

	if active == "" {
		active = workspaces[0].ID
	} else {
		found := false
		for _, w := range workspaces {
			if w.ID == active {
				found = true
				break
			}
		}
		if !found {
			active = workspaces[0].ID
		}
	}

	if store != nil && (loaded.ActiveWorkspaceID != active || len(loaded.Workspaces) != len(workspaces)) {
		loaded.Workspaces = workspaces
		loaded.ActiveWorkspaceID = active
		if err := store.Save(loaded); err != nil {
			return fmt.Errorf("save workspaces: %w", err)
		}
	}

	all := make([]auth.LocalWorkspace, len(workspaces))
	for i, w := range workspaces {
		all[i] = auth.LocalWorkspace{ID: w.ID, Name: w.Name}
	}
	auth.SetLocalWorkspaces(active, all)
	return nil
}

func dataDirLooksUsed(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".db" {
			return true
		}
	}
	return false
}

func waitForReady(port int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 200*time.Millisecond)
		if err == nil {
			conn.Close()
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false
}
