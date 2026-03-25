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
	r.Get("/search", h.Search)
	r.Get("/preview/{token}", h.Preview)
	r.Get("/{slug}", h.PageOrCollection)
	r.Get("/{collection}/{page}", h.CollectionPage)
	r.Post("/{slug}/comments", h.SubmitComment)
	r.Post("/{collection}/{page}/comments", h.SubmitCollectionComment)
}
