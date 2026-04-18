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
	level := slog.LevelInfo
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		_ = (&level).UnmarshalText([]byte(v))
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})))

	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		slog.Warn("load .env", "err", err)
	}

	slog.Info("writekit starting", "pid", os.Getpid(), "log_level", level.String())

	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config", "err", err)
		os.Exit(1)
	}
	slog.Info("config loaded",
		"host", cfg.Host,
		"port", cfg.Port,
		"data_dir", cfg.DataDir,
		"max_pool_size", cfg.MaxPoolSize,
		"dev", cfg.Dev,
	)

	ctx := context.Background()

	var platformDB *platform.DB
	if !cfg.Local {
		db, err := platform.NewDB(ctx, cfg.DatabaseURL)
		if err != nil {
			slog.Error("connect postgres", "err", err)
			os.Exit(1)
		}
		defer db.Close()
		slog.Info("postgres connected")

		if err := db.Migrate(ctx); err != nil {
			slog.Error("migrate postgres", "err", err)
			os.Exit(1)
		}
		slog.Info("postgres migrated")
		platformDB = db
	} else {
		slog.Info("local mode: skipping postgres")
	}

	pool, err := tenant.NewPool(cfg.DataDir, cfg.MaxPoolSize)
	if err != nil {
		slog.Error("create tenant pool", "err", err, "data_dir", cfg.DataDir)
		os.Exit(1)
	}
	defer pool.Close()
	slog.Info("tenant pool created", "data_dir", cfg.DataDir, "max", cfg.MaxPoolSize)

	tFS, _ := fs.Sub(writekit.TemplatesFS, "templates")
	sFS, _ := fs.Sub(writekit.StaticFS, "static")

	application := app.New(cfg, platformDB, pool, tFS, sFS, writekit.AppFS, writekit.AdminFS)

	if err := application.Run(); err != nil {
		slog.Error("server exited with error", "err", err)
		os.Exit(1)
	}
	slog.Info("server exited cleanly")
}
