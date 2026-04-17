package site

import (
	"github.com/go-chi/chi/v5"
	"writekit/internal/auth"
	"writekit/internal/config"
	"writekit/internal/events"
	"writekit/internal/og"
	"writekit/internal/platform"
	"writekit/internal/render"
	"writekit/internal/tenant"
)

type Handler struct {
	Pool       *tenant.Pool
	Config     *config.Config
	Engine     *render.Engine
	Bus        *events.Bus
	Cache      *Cache
	PlatformDB *platform.DB
	OG         *og.Renderer
}

func (h *Handler) Routes(r chi.Router) {
	r.Use(auth.OptionalWebAuth(h.PlatformDB))
	r.Get("/robots.txt", h.TenantRobotsTxt)
	r.Get("/sitemap.xml", h.TenantSitemap)
	r.Get("/", h.Index)
	r.Get("/search", h.Search)
	r.Get("/preview/{token}", h.Preview)
	r.Get("/preview/{token}/events", h.PreviewSSE)
	r.Get("/og/{slug}.png", h.PageOG)
	r.Get("/og/{collection}/{page}.png", h.CollectionPageOG)
	r.Get("/{slug}.md", h.RawMarkdown)
	r.Get("/{slug}", h.PageOrCollection)
	r.Get("/{collection}/{page}.md", h.RawCollectionMarkdown)
	r.Get("/{collection}/{page}", h.CollectionPage)
}
