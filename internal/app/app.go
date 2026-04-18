package app

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"writekit/internal/admin"
	"writekit/internal/api"
	"writekit/internal/auth"
	"writekit/internal/site"
	"writekit/internal/config"
	"writekit/internal/email"
	"writekit/internal/embedding"
	"writekit/internal/events"
	"writekit/internal/httplog"
	mcpserver "writekit/internal/mcp"
	"writekit/internal/og"
	"writekit/internal/platform"
	"writekit/internal/render"
	"writekit/internal/tenant"
	"writekit/internal/web"
	"github.com/stripe/stripe-go/v82"
)

type App struct {
	Config     *config.Config
	PlatformDB *platform.DB
	Pool       *tenant.Pool
	Bus        *events.Bus
	Embedder   *embedding.Client
	Worker     *EmbeddingWorker
	Router     http.Handler
}

func New(cfg *config.Config, platformDB *platform.DB, pool *tenant.Pool, templatesFS, staticFS, appFS, adminFS fs.FS) *App {
	bus := events.NewBus()

	if cfg.StripeSecretKey != "" {
		stripe.Key = cfg.StripeSecretKey
	}

	webTemplatesFS, _ := fs.Sub(templatesFS, "web")
	siteTemplatesFS, _ := fs.Sub(templatesFS, "themes/default")
	webEngine := render.New(webTemplatesFS, cfg.Dev)
	siteEngine := render.New(siteTemplatesFS, cfg.Dev)

	var google, github, discord *auth.OAuthProvider
	if cfg.GoogleClientID != "" {
		google = auth.NewGoogleProvider(cfg.GoogleClientID, cfg.GoogleClientSecret,
			cfg.BaseURL+"/auth/callback")
	}
	if cfg.GithubClientID != "" {
		github = auth.NewGithubProvider(cfg.GithubClientID, cfg.GithubClientSecret,
			cfg.BaseURL+"/auth/callback")
	}
	if cfg.DiscordClientID != "" {
		discord = auth.NewDiscordProvider(cfg.DiscordClientID, cfg.DiscordClientSecret,
			cfg.BaseURL+"/auth/callback")
	}

	mcpAuth := &auth.MCPAuth{DB: platformDB, BaseURL: cfg.BaseURL}

	emailSender := email.NewSender(cfg.SESFrom, cfg.SESRegion)

	mcpSrv := mcpserver.New(platformDB, pool, cfg, bus)

	cache := site.NewCache(bus)

	ogRenderer, err := og.New()
	if err != nil {
		slog.Warn("og renderer disabled", "err", err)
	}

	webHandler := &web.Handler{
		DB:      platformDB,
		Config:  cfg,
		Engine:  webEngine,
		Google:  google,
		Github:  github,
		Discord: discord,
		MCPAuth: mcpAuth,
		Email:   emailSender,
		Pool:    pool,
		OG:      ogRenderer,
	}

	siteHandler := &site.Handler{
		Pool:       pool,
		Config:     cfg,
		Engine:     siteEngine,
		Bus:        bus,
		Cache:      cache,
		PlatformDB: platformDB,
		OG:         ogRenderer,
	}

	embedClient := embedding.NewClient(cfg.OllamaHost, cfg.EmbeddingModel)

	apiHandler := &api.Handler{
		DB:       platformDB,
		Pool:     pool,
		Config:   cfg,
		Embedder: embedClient,
		Bus:      bus,
	}

	adminHandler := &admin.Handler{
		DB:     platformDB,
		Pool:   pool,
		Config: cfg,
		Email:  emailSender,
	}

	router := buildRouter(cfg, webHandler, siteHandler, apiHandler, adminHandler, mcpSrv, platformDB, staticFS, appFS, adminFS)

	worker := NewEmbeddingWorker(pool, bus, embedClient)

	return &App{
		Config:     cfg,
		PlatformDB: platformDB,
		Pool:       pool,
		Bus:        bus,
		Embedder:   embedClient,
		Worker:     worker,
		Router:     router,
	}
}

func buildRouter(cfg *config.Config, webHandler *web.Handler, siteHandler *site.Handler, apiHandler *api.Handler, adminHandler *admin.Handler, mcpSrv *mcpserver.Server, platformDB *platform.DB, staticFS, appFS, adminFS fs.FS) http.Handler {
	root := chi.NewRouter()
	root.Use(chimw.RealIP)
	root.Use(httplog.RequestIDMiddleware)
	root.Use(httplog.Recoverer)
	root.Use(httplog.Access)

	fileServer := http.FileServer(http.FS(staticFS))
	root.Handle("/static/*", http.StripPrefix("/static/", fileServer))

	webR := webRouter(cfg, webHandler, mcpSrv, platformDB)
	siteR := siteRouter(siteHandler)
	spaR := spaRouter(apiHandler, appFS, cfg, platformDB)
	adminR := adminSpaRouter(adminHandler, adminFS)

	root.HandleFunc("/*", func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		if i := strings.LastIndex(host, ":"); i > 0 {
			host = host[:i]
		}

		switch {
		case host == "app."+cfg.Host || (cfg.Dev && host == "app.localhost"):
			spaR.ServeHTTP(w, r)
		case host == "admin."+cfg.Host || (cfg.Dev && host == "admin.localhost"):
			adminR.ServeHTTP(w, r)
		case host == cfg.Host || (cfg.Dev && (host == "localhost" || host == "127.0.0.1")):
			webR.ServeHTTP(w, r)
		case strings.HasSuffix(host, "."+cfg.Host):
			siteR.ServeHTTP(w, r)
		default:
			httplog.FromContext(r.Context()).Warn("unknown host, returning 404", "host", host)
			http.NotFound(w, r)
		}
	})

	return root
}

func webRouter(cfg *config.Config, webHandler *web.Handler, mcpSrv *mcpserver.Server, platformDB *platform.DB) http.Handler {
	r := chi.NewRouter()

	webHandler.Routes(r)

	r.Group(func(r chi.Router) {
		r.Use(auth.MCPBearerAuth(platformDB, cfg.BaseURL))
		r.Handle("/mcp", mcpSrv.Handler())
	})

	return r
}

func spaRouter(apiHandler *api.Handler, appFS fs.FS, cfg *config.Config, platformDB *platform.DB) http.Handler {
	r := chi.NewRouter()

	apiHandler.Routes(r)

	distFS, _ := fs.Sub(appFS, "apps/user/dist")
	fileServer := http.FileServer(http.FS(distFS))

	r.Handle("/assets/*", fileServer)
	r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session")
		if err != nil {
			httplog.FromContext(r.Context()).Debug("spa: no session cookie, redirecting", "err", err)
			http.Redirect(w, r, cfg.BaseURL, http.StatusSeeOther)
			return
		}
		if _, err := platformDB.GetSession(r.Context(), cookie.Value); err != nil {
			httplog.FromContext(r.Context()).Debug("spa: invalid session, redirecting", "err", err)
			http.Redirect(w, r, cfg.BaseURL, http.StatusSeeOther)
			return
		}

		f, err := distFS.Open("index.html")
		if err != nil {
			httplog.FromContext(r.Context()).Error("open user spa index", "err", err)
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		defer f.Close()
		stat, _ := f.Stat()
		http.ServeContent(w, r, "index.html", stat.ModTime(), f.(io.ReadSeeker))
	})

	return r
}

func adminSpaRouter(adminHandler *admin.Handler, adminFS fs.FS) http.Handler {
	r := chi.NewRouter()

	adminHandler.Routes(r)

	distFS, _ := fs.Sub(adminFS, "apps/admin/dist")
	fileServer := http.FileServer(http.FS(distFS))

	r.Handle("/assets/*", fileServer)
	r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
		f, err := distFS.Open("index.html")
		if err != nil {
			httplog.FromContext(r.Context()).Error("open admin spa index", "err", err)
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		defer f.Close()
		stat, _ := f.Stat()
		http.ServeContent(w, r, "index.html", stat.ModTime(), f.(io.ReadSeeker))
	})

	return r
}

func siteRouter(siteHandler *site.Handler) http.Handler {
	r := chi.NewRouter()
	siteHandler.Routes(r)
	return r
}

func (a *App) ListenAddr() string {
	return fmt.Sprintf(":%d", a.Config.Port)
}

func (a *App) Run() error {
	a.Worker.Start(context.Background())
	addr := a.ListenAddr()
	slog.Info("server starting",
		"addr", addr,
		"host", a.Config.Host,
		"dev", a.Config.Dev,
		"base_url", a.Config.BaseURL,
		"stripe_configured", a.Config.StripeSecretKey != "",
		"email_configured", a.Config.SESFrom != "",
		"embedding_configured", a.Embedder.Enabled(),
	)
	if err := http.ListenAndServe(addr, a.Router); err != nil {
		slog.Error("http listen", "addr", addr, "err", err)
		return err
	}
	return nil
}
