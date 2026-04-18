package web

import (
	"github.com/go-chi/chi/v5"
	"writekit/internal/auth"
	"writekit/internal/config"
	"writekit/internal/email"
	"writekit/internal/og"
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
	OG      *og.Renderer
}

func (h *Handler) Routes(r chi.Router) {

	r.Get("/auth/me", h.AuthMe)
	r.Get("/auth/login", h.LoginPage)
	r.Get("/auth/login/{provider}", h.OAuthStart)
	r.Get("/auth/callback", h.OAuthCallback)
	r.Post("/auth/logout", h.Logout)
	r.Post("/auth/magic-link", h.MagicLinkRequest)
	r.Get("/auth/magic-link/verify", h.MagicLinkVerify)

	r.Get("/.well-known/oauth-authorization-server", h.MCPAuth.WellKnown)
	r.Method("GET", "/.well-known/oauth-protected-resource", h.MCPAuth.ProtectedResourceHandler())
	r.Post("/oauth/register", h.MCPAuth.Register)
	r.Get("/oauth/authorize", h.OAuthAuthorize)
	r.Post("/oauth/token", h.MCPAuth.TokenExchange)

	r.Group(func(r chi.Router) {
		r.Use(auth.WebAuth(h.DB))
		r.Get("/settings", h.SettingsPage)
		r.Post("/settings/unlink/{accountID}", h.UnlinkAccount)
		r.Post("/settings/delete-account", h.DeleteAccount)
	})

	r.Get("/", h.Home)
	r.Get("/docs", h.Docs)
	r.Get("/download", h.Download)
	r.Get("/og.png", h.LandingOG)
	r.Get("/llms.txt", h.LLMsTxt)
	r.Get("/llms-full.txt", h.LLMsFullTxt)
	r.Get("/robots.txt", h.RobotsTxt)
	r.Get("/sitemap.xml", h.Sitemap)

	r.Post("/stripe/webhook", h.StripeWebhook)
}
