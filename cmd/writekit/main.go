package main

import (
	"context"
	"io/fs"
	"log/slog"
	"os"

	"github.com/joho/godotenv"
	writekit "writekit"
	"writekit/internal/app"
	"writekit/internal/config"
	"writekit/internal/platform"
	"writekit/internal/tenant"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config", "err", err)
		os.Exit(1)
	}

	ctx := context.Background()

	platformDB, err := platform.NewDB(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("connect postgres", "err", err)
		os.Exit(1)
	}
	defer platformDB.Close()

	if err := platformDB.Migrate(ctx); err != nil {
		slog.Error("migrate postgres", "err", err)
		os.Exit(1)
	}

	pool, err := tenant.NewPool(cfg.DataDir, cfg.MaxPoolSize)
	if err != nil {
		slog.Error("create tenant pool", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	tFS, _ := fs.Sub(writekit.TemplatesFS, "templates")
	sFS, _ := fs.Sub(writekit.StaticFS, "static")

	application := app.New(cfg, platformDB, pool, tFS, sFS, writekit.AppFS, writekit.AdminFS)

	if err := application.Run(); err != nil {
		slog.Error("server error", "err", err)
		os.Exit(1)
	}
}
