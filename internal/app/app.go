package app

import (
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"writekit/internal/api"
	"writekit/internal/auth"
	"writekit/internal/blog"
	"writekit/internal/config"
	"writekit/internal/email"
	"writekit/internal/events"
	mcpserver "writekit/internal/mcp"
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
	Router     http.Handler
}

func New(cfg *config.Config, platformDB *platform.DB, pool *tenant.Pool, templatesFS, staticFS, appFS fs.FS) *App {
	bus := events.NewBus()

	if cfg.StripeSecretKey != "" {
		stripe.Key = cfg.StripeSecretKey
	}

	webTemplatesFS, _ := fs.Sub(templatesFS, "web")
	blogTemplatesFS, _ := fs.Sub(templatesFS, "themes/default")
	webEngine := render.New(webTemplatesFS, cfg.Dev)
	blogEngine := render.New(blogTemplatesFS, cfg.Dev)

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

	cache := blog.NewCache(bus)

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
	}

	blogHandler := &blog.Handler{
		Pool:       pool,
		Config:     cfg,
		Engine:     blogEngine,
		Bus:        bus,
		Cache:      cache,
		PlatformDB: platformDB,
		Email:      emailSender,
	}

	apiHandler := &api.Handler{
		DB:     platformDB,
		Pool:   pool,
		Config: cfg,
	}

	router := buildRouter(cfg, webHandler, blogHandler, apiHandler, mcpSrv, platformDB, staticFS, appFS)

	return &App{
		Config:     cfg,
		PlatformDB: platformDB,
		Pool:       pool,
		Bus:        bus,
		Router:     router,
	}
}

func buildRouter(cfg *config.Config, webHandler *web.Handler, blogHandler *blog.Handler, apiHandler *api.Handler, mcpSrv *mcpserver.Server, platformDB *platform.DB, staticFS, appFS fs.FS) http.Handler {
	root := chi.NewRouter()
	root.Use(chimw.Logger)
	root.Use(chimw.Recoverer)
	root.Use(chimw.RealIP)

	fileServer := http.FileServer(http.FS(staticFS))
	root.Handle("/static/*", http.StripPrefix("/static/", fileServer))

	webR := webRouter(cfg, webHandler, mcpSrv, platformDB)
	blogR := blogRouter(blogHandler)
	spaR := spaRouter(apiHandler, appFS)

	root.HandleFunc("/*", func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		if i := strings.LastIndex(host, ":"); i > 0 {
			host = host[:i]
		}

		switch {
		case host == "app."+cfg.Host || (cfg.Dev && host == "app.localhost"):
			spaR.ServeHTTP(w, r)
		case host == cfg.Host || (cfg.Dev && (host == "localhost" || host == "127.0.0.1")):
			webR.ServeHTTP(w, r)
		case strings.HasSuffix(host, "."+cfg.Host):
			blogR.ServeHTTP(w, r)
		default:
			http.NotFound(w, r)
		}
	})

	return root
}

func webRouter(cfg *config.Config, webHandler *web.Handler, mcpSrv *mcpserver.Server, platformDB *platform.DB) http.Handler {
	r := chi.NewRouter()

	webHandler.Routes(r)

	r.Group(func(r chi.Router) {
		r.Use(auth.BearerAuth(platformDB, cfg.BaseURL))
		r.Handle("/mcp", mcpSrv.Handler())
	})

	return r
}

func spaRouter(apiHandler *api.Handler, appFS fs.FS) http.Handler {
	r := chi.NewRouter()

	apiHandler.Routes(r)

	distFS, _ := fs.Sub(appFS, "ui/dist")
	fileServer := http.FileServer(http.FS(distFS))

	r.Handle("/assets/*", fileServer)
	r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
		f, err := distFS.Open("index.html")
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		defer f.Close()
		stat, _ := f.Stat()
		http.ServeContent(w, r, "index.html", stat.ModTime(), f.(io.ReadSeeker))
	})

	return r
}

func blogRouter(blogHandler *blog.Handler) http.Handler {
	r := chi.NewRouter()
	blogHandler.Routes(r)
	return r
}

func (a *App) ListenAddr() string {
	return fmt.Sprintf(":%d", a.Config.Port)
}

func (a *App) Run() error {
	addr := a.ListenAddr()
	slog.Info("starting server", "addr", addr, "host", a.Config.Host)
	return http.ListenAndServe(addr, a.Router)
}
