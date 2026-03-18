package blog

import (
	"github.com/go-chi/chi/v5"
	"writekit/internal/config"
	"writekit/internal/email"
	"writekit/internal/events"
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
	Email      *email.Sender
}

func (h *Handler) Routes(r chi.Router) {
	r.Get("/", h.Index)
	r.Get("/posts/{slug}", h.Post)
	r.Get("/tags/{tag}", h.Tag)
	r.Get("/search", h.Search)
	r.Get("/preview/{token}", h.Preview)
	r.Get("/feed.xml", h.RSS)
	r.Post("/posts/{slug}/comments", h.SubmitComment)
}
