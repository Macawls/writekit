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
	"github.com/stripe/stripe-go/v82"
	"writekit/internal/admin"
	"writekit/internal/api"
	"writekit/internal/auth"
	"writekit/internal/config"
	"writekit/internal/email"
	"writekit/internal/events"
	"writekit/internal/httplog"
	mcpserver "writekit/internal/mcp"
	"writekit/internal/og"
	"writekit/internal/platform"
	"writekit/internal/render"
	"writekit/internal/site"
	"writekit/internal/tenant"
	"writekit/internal/web"
)

type App struct {
	Config        *config.Config
	PlatformDB    *platform.DB
	Pool          *tenant.Pool
	Bus           *events.Bus
	Router        http.Handler
	PreviewRouter http.Handler
}

func New(cfg *config.Config, platformDB *platform.DB, pool *tenant.Pool, templatesFS, staticFS, appFS, adminFS fs.FS) *App {
	bus := events.NewBus()

	siteTemplatesFS, _ := fs.Sub(templatesFS, "themes/default")
	siteEngine := render.New(siteTemplatesFS, cfg.Dev)

	mcpSrv := mcpserver.New(platformDB, pool, cfg, bus)
	cache := site.NewCache(bus)

	prerender := &site.PreRenderer{Pool: pool, Engine: siteEngine, Config: cfg, Bus: bus}
	prerender.Start()

	siteHandler := &site.Handler{
		Pool:       pool,
		Config:     cfg,
		Engine:     siteEngine,
		Bus:        bus,
		Cache:      cache,
		PlatformDB: platformDB,
	}

	apiHandler := &api.Handler{
		DB:     platformDB,
		Pool:   pool,
		Config: cfg,
		Bus:    bus,
	}

	var router, previewRouter http.Handler
	if cfg.Local {
		router = buildLocalRouter(cfg, siteHandler, apiHandler, mcpSrv, staticFS, appFS)
		if cfg.Dev {
			previewRouter = buildPreviewRouter(cfg, siteHandler, staticFS)
		}
	} else {
		if cfg.StripeSecretKey != "" {
			stripe.Key = cfg.StripeSecretKey
		}

		webTemplatesFS, _ := fs.Sub(templatesFS, "web")
		webEngine := render.New(webTemplatesFS, cfg.Dev)

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
		subscribeEmailHandlers(bus, emailSender, cfg)

		ogRenderer, err := og.New()
		if err != nil {
			slog.Warn("og renderer disabled", "err", err)
		}
		siteHandler.OG = ogRenderer

		webHandler := &web.Handler{
			DB:      platformDB,
			Config:  cfg,
			Engine:  webEngine,
			Google:  google,
			Github:  github,
			Discord: discord,
			MCPAuth: mcpAuth,
			Email:   emailSender,
			Bus:     bus,
			Pool:    pool,
			OG:      ogRenderer,
		}

		adminHandler := &admin.Handler{
			DB:     platformDB,
			Pool:   pool,
			Config: cfg,
			Email:  emailSender,
		}

		router = buildRouter(cfg, webHandler, siteHandler, apiHandler, adminHandler, mcpSrv, platformDB, staticFS, appFS, adminFS)
	}

	return &App{
		Config:        cfg,
		PlatformDB:    platformDB,
		Pool:          pool,
		Bus:           bus,
		Router:        router,
		PreviewRouter: previewRouter,
	}
}

func buildPreviewRouter(cfg *config.Config, siteHandler *site.Handler, staticFS fs.FS) http.Handler {
	root := chi.NewRouter()
	root.Use(chimw.RealIP)
	root.Use(httplog.RequestIDMiddleware)
	root.Use(httplog.Recoverer)
	root.Use(httplog.Access)
	root.Handle("/static/*", http.StripPrefix("/static/", staticFileServer(cfg, staticFS)))
	root.Group(func(r chi.Router) {
		siteHandler.Routes(r)
	})
	return root
}

func buildLocalRouter(cfg *config.Config, siteHandler *site.Handler, apiHandler *api.Handler, mcpSrv *mcpserver.Server, staticFS, appFS fs.FS) http.Handler {
	root := chi.NewRouter()
	root.Use(chimw.RealIP)
	root.Use(httplog.RequestIDMiddleware)
	root.Use(httplog.Recoverer)
	root.Use(httplog.Access)

	root.Handle("/static/*", http.StripPrefix("/static/", staticFileServer(cfg, staticFS)))

	root.Group(func(r chi.Router) {
		apiHandler.LocalRoutes(r)
	})

	root.Group(func(r chi.Router) {
		r.Use(auth.LocalAuth())
		r.Handle("/mcp", mcpSrv.Handler())
	})

	root.Route("/site", func(r chi.Router) {
		siteHandler.Routes(r)
	})

	distFS, _ := fs.Sub(appFS, "apps/user/dist")
	spaFileServer := http.FileServer(http.FS(distFS))
	root.Handle("/assets/*", spaFileServer)
	root.Get("/*", func(w http.ResponseWriter, r *http.Request) {
		f, err := distFS.Open("index.html")
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		defer f.Close()
		stat, _ := f.Stat()
		http.ServeContent(w, r, "index.html", stat.ModTime(), f.(io.ReadSeeker))
	})

	return root
}

func buildRouter(cfg *config.Config, webHandler *web.Handler, siteHandler *site.Handler, apiHandler *api.Handler, adminHandler *admin.Handler, mcpSrv *mcpserver.Server, platformDB *platform.DB, staticFS, appFS, adminFS fs.FS) http.Handler {
	root := chi.NewRouter()
	root.Use(chimw.RealIP)
	root.Use(httplog.RequestIDMiddleware)
	root.Use(httplog.Recoverer)
	root.Use(httplog.Access)

	root.Handle("/static/*", http.StripPrefix("/static/", staticFileServer(cfg, staticFS)))

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
			slug := strings.TrimSuffix(host, "."+cfg.Host)
			if _, err := platformDB.GetTenant(r.Context(), slug); err != nil {
				if newID, aliasErr := platformDB.GetTenantIDByAlias(r.Context(), slug); aliasErr == nil {
					target := "https://" + newID + "." + cfg.Host + r.URL.RequestURI()
					http.Redirect(w, r, target, http.StatusMovedPermanently)
					return
				}
			}
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

func staticFileServer(cfg *config.Config, staticFS fs.FS) http.Handler {
	if cfg.Dev {
		return http.FileServer(http.Dir("static"))
	}
	return http.FileServer(http.FS(staticFS))
}

func siteRouter(siteHandler *site.Handler) http.Handler {
	r := chi.NewRouter()
	siteHandler.Routes(r)
	return r
}

func (a *App) ListenAddr() string {
	if a.Config.Dev {
		return fmt.Sprintf("127.0.0.1:%d", a.Config.Port)
	}
	return fmt.Sprintf(":%d", a.Config.Port)
}

func (a *App) Run() error {
	addr := a.ListenAddr()
	slog.Info("server starting",
		"addr", addr,
		"host", a.Config.Host,
		"dev", a.Config.Dev,
		"base_url", a.Config.BaseURL,
		"stripe_configured", a.Config.StripeSecretKey != "",
		"email_configured", a.Config.SESFrom != "",
	)
	if a.PreviewRouter != nil {
		previewAddr := fmt.Sprintf("127.0.0.1:%d", a.Config.Port+1)
		go func() {
			slog.Info("preview site listener starting", "addr", previewAddr)
			if err := http.ListenAndServe(previewAddr, a.PreviewRouter); err != nil {
				slog.Error("preview http listen", "addr", previewAddr, "err", err)
			}
		}()
	}
	if err := http.ListenAndServe(addr, a.Router); err != nil {
		slog.Error("http listen", "addr", addr, "err", err)
		return err
	}
	return nil
}
