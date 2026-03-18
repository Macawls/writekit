package web

import (
	"github.com/go-chi/chi/v5"
	"writekit/internal/auth"
	"writekit/internal/config"
	"writekit/internal/email"
	"writekit/internal/platform"
	"writekit/internal/render"
	"writekit/internal/tenant"
)

type Handler struct {
	DB       *platform.DB
	Pool     *tenant.Pool
	Config   *config.Config
	Engine   *render.Engine
	Google   *auth.OAuthProvider
	Github   *auth.OAuthProvider
	Discord  *auth.OAuthProvider
	MCPAuth  *auth.MCPAuth
	Email    *email.Sender
}

func (h *Handler) Routes(r chi.Router) {

	r.Get("/auth/login", h.LoginPage)
	r.Get("/auth/callback/{provider}", h.OAuthCallback)
	r.Post("/auth/logout", h.Logout)

	r.Get("/.well-known/oauth-authorization-server", h.MCPAuth.WellKnown)
	r.Post("/oauth/register", h.MCPAuth.Register)
	r.Get("/oauth/authorize", h.OAuthAuthorize)
	r.Post("/oauth/authorize", h.OAuthAuthorizeSubmit)
	r.Post("/oauth/token", h.MCPAuth.TokenExchange)

	r.Get("/", h.Home)
	r.Get("/docs", h.Docs)
	r.Get("/llms.txt", h.LLMsTxt)

	r.Group(func(r chi.Router) {
		r.Use(auth.WebAuth(h.DB))

		r.Get("/dashboard", h.Dashboard)
		r.Get("/profile", h.ProfilePage)
		r.Post("/profile", h.ProfileUpdate)
		r.Post("/blogs", h.CreateBlog)

		r.Get("/billing", h.BillingPage)
		r.Post("/billing/checkout", h.BillingCheckout)
		r.Post("/billing/portal", h.BillingPortal)
	})

	r.Post("/stripe/webhook", h.StripeWebhook)
}
