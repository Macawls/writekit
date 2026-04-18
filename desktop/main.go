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
	"time"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	writekit "writekit"
	"writekit/internal/app"
	"writekit/internal/config"
	"writekit/internal/platform"
	"writekit/internal/tenant"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	os.Setenv("LOCAL", "true")
	if os.Getenv("HOST") == "" {
		os.Setenv("HOST", "localhost")
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

	if err := writePortFile(port); err != nil {
		slog.Warn("write port file", "err", err)
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

	application.Worker.Start(context.Background())

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

	proxyTarget, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", port))
	proxy := httputil.NewSingleHostReverseProxy(proxyTarget)

	err = wails.Run(&options.App{
		Title:            "WriteKit",
		Width:            1280,
		Height:           860,
		MinWidth:         900,
		MinHeight:        600,
		BackgroundColour: &options.RGBA{R: 250, G: 250, B: 250, A: 255},
		AssetServer: &assetserver.Options{
			Handler: proxy,
		},
		OnShutdown: func(ctx context.Context) {
			_ = server.Shutdown(ctx)
		},
	})
	if err != nil {
		slog.Error("wails run", "err", err)
		os.Exit(1)
	}
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
