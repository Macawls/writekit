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
	DB      *platform.DB
	Config  *config.Config
	Engine  *render.Engine
	Google  *auth.OAuthProvider
	Github  *auth.OAuthProvider
	Discord *auth.OAuthProvider
	MCPAuth *auth.MCPAuth
	Email   *email.Sender
	Pool    *tenant.Pool
}

func (h *Handler) Routes(r chi.Router) {

	r.Get("/auth/login", h.LoginPage)
	r.Get("/auth/login/{provider}", h.OAuthStart)
	r.Get("/auth/callback", h.OAuthCallback)
	r.Post("/auth/logout", h.Logout)
	r.Post("/auth/magic-link", h.MagicLinkRequest)
	r.Get("/auth/magic-link/verify", h.MagicLinkVerify)

	r.Get("/.well-known/oauth-authorization-server", h.MCPAuth.WellKnown)
	r.Get("/.well-known/oauth-protected-resource", h.MCPAuth.ProtectedResource)
	r.Post("/oauth/register", h.MCPAuth.Register)
	r.Get("/oauth/authorize", h.OAuthAuthorize)
	r.Post("/oauth/authorize", h.OAuthAuthorizeSubmit)
	r.Post("/oauth/token", h.MCPAuth.TokenExchange)

	r.Post("/register", h.MCPAuth.Register)
	r.Get("/authorize", h.OAuthAuthorize)
	r.Post("/authorize", h.OAuthAuthorizeSubmit)
	r.Post("/token", h.MCPAuth.TokenExchange)

	r.Group(func(r chi.Router) {
		r.Use(auth.WebAuth(h.DB))
		r.Get("/settings", h.SettingsPage)
		r.Post("/settings/unlink/{accountID}", h.UnlinkAccount)
		r.Post("/settings/delete-account", h.DeleteAccount)
	})

	r.Get("/", h.Home)
	r.Get("/docs", h.Docs)
	r.Get("/llms.txt", h.LLMsTxt)

	r.Post("/stripe/webhook", h.StripeWebhook)
}
