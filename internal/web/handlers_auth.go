package web

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"time"

	"github.com/go-chi/chi/v5"
	"writekit/internal/auth"
	"writekit/internal/platform"
)

func (h *Handler) LoginPage(w http.ResponseWriter, r *http.Request) {

	if r.URL.Query().Get("oauth") == "1" {
		h.renderOAuthLogin(w, r)
		return
	}
	h.Engine.Render(w, "login.html", map[string]any{
		"GoogleEnabled":  h.Config.GoogleClientID != "",
		"GithubEnabled":  h.Config.GithubClientID != "",
		"DiscordEnabled": h.Config.DiscordClientID != "",
	})
}

func (h *Handler) renderOAuthLogin(w http.ResponseWriter, r *http.Request) {
	h.Engine.Render(w, "login.html", map[string]any{
		"GoogleEnabled":  h.Config.GoogleClientID != "",
		"GithubEnabled":  h.Config.GithubClientID != "",
		"DiscordEnabled": h.Config.DiscordClientID != "",
		"OAuth":          true,
		"OAuthParams":    r.URL.Query().Encode(),
	})
}

func (h *Handler) OAuthCallback(w http.ResponseWriter, r *http.Request) {
	providerName := chi.URLParam(r, "provider")
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	var provider *auth.OAuthProvider
	switch providerName {
	case "google":
		provider = h.Google
	case "github":
		provider = h.Github
	case "discord":
		provider = h.Discord
	default:
		http.Error(w, "unknown provider", http.StatusBadRequest)
		return
	}

	if provider == nil {
		http.Error(w, "provider not configured", http.StatusBadRequest)
		return
	}

	token, err := provider.Exchange(r.Context(), code)
	if err != nil {
		slog.Error("oauth exchange failed", "err", err)
		http.Error(w, "authentication failed", http.StatusInternalServerError)
		return
	}

	info, err := provider.GetUserInfo(r.Context(), token)
	if err != nil {
		slog.Error("get user info failed", "err", err)
		http.Error(w, "failed to get user info", http.StatusInternalServerError)
		return
	}

	user, isNew, err := h.DB.UpsertUser(r.Context(), &platform.User{
		Email:         info.Email,
		Name:          info.Name,
		AvatarURL:     info.AvatarURL,
		OAuthProvider: providerName,
		OAuthID:       info.ID,
	})
	if err != nil {
		slog.Error("upsert user failed", "err", err)
		http.Error(w, "failed to save user", http.StatusInternalServerError)
		return
	}

	if isNew {
		go func() {
			if err := h.Email.SendWelcome(r.Context(), user.Email, user.Name); err != nil {
				slog.Error("send welcome email", "err", err)
			}
		}()
	}

	sess, err := h.DB.CreateSession(r.Context(), user.ID)
	if err != nil {
		slog.Error("create session failed", "err", err)
		http.Error(w, "failed to create session", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    sess.ID,
		Path:     "/",
		HttpOnly: true,
		Secure:   !h.Config.Dev,
		SameSite: http.SameSiteLaxMode,
		Expires:  sess.ExpiresAt,
	})

	if state != "" {
		http.Redirect(w, r, "/oauth/authorize?"+state, http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session")
	if err == nil {
		h.DB.DeleteSession(r.Context(), cookie.Value)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})

	http.Redirect(w, r, "/auth/login", http.StatusSeeOther)
}

func (h *Handler) OAuthAuthorize(w http.ResponseWriter, r *http.Request) {
	authReq := auth.ParseAuthRequest(r)

	if err := h.MCPAuth.ValidateAuthRequest(r, authReq); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	cookie, err := r.Cookie("session")
	if err != nil {

		loginURL := fmt.Sprintf("/auth/login?oauth=1&%s", r.URL.RawQuery)
		http.Redirect(w, r, loginURL, http.StatusSeeOther)
		return
	}

	sess, err := h.DB.GetSession(r.Context(), cookie.Value)
	if err != nil {
		loginURL := fmt.Sprintf("/auth/login?oauth=1&%s", r.URL.RawQuery)
		http.Redirect(w, r, loginURL, http.StatusSeeOther)
		return
	}

	user, err := h.DB.GetUser(r.Context(), sess.UserID)
	if err != nil {
		http.Error(w, "user not found", http.StatusInternalServerError)
		return
	}

	h.Engine.Render(w, "authorize.html", map[string]any{
		"User":    user,
		"AuthReq": authReq,
	})
}

func (h *Handler) OAuthAuthorizeSubmit(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session")
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	sess, err := h.DB.GetSession(r.Context(), cookie.Value)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	authReq := &auth.AuthRequest{
		ClientID:      r.FormValue("client_id"),
		RedirectURI:   r.FormValue("redirect_uri"),
		State:         r.FormValue("state"),
		Scope:         r.FormValue("scope"),
		CodeChallenge: r.FormValue("code_challenge"),
		CodeMethod:    r.FormValue("code_challenge_method"),
	}

	if r.FormValue("action") == "deny" {
		redirectURL := authReq.RedirectURI + "?error=access_denied"
		if authReq.State != "" {
			redirectURL += "&state=" + authReq.State
		}
		http.Redirect(w, r, redirectURL, http.StatusSeeOther)
		return
	}

	code, err := h.MCPAuth.IssueAuthCode(r, sess.UserID, authReq)
	if err != nil {
		slog.Error("issue auth code failed", "err", err)
		http.Error(w, "failed to issue code", http.StatusInternalServerError)
		return
	}

	redirectURL := auth.BuildRedirectURL(authReq.RedirectURI, code, authReq.State)
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

func (h *Handler) Home(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session")
	if err != nil {
		h.Engine.Render(w, "landing.html", nil)
		return
	}

	sess, err := h.DB.GetSession(r.Context(), cookie.Value)
	if err != nil {
		h.Engine.Render(w, "landing.html", nil)
		return
	}

	user, err := h.DB.GetUser(r.Context(), sess.UserID)
	if err != nil {
		h.Engine.Render(w, "landing.html", nil)
		return
	}

	tenants, _ := h.DB.ListTenantsByUser(r.Context(), user.ID)
	h.Engine.Render(w, "dashboard.html", map[string]any{
		"User":    user,
		"Tenants": tenants,
		"Host":    h.Config.Host,
	})
}

func (h *Handler) Dashboard(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	tenants := auth.TenantsFromContext(r.Context())

	h.Engine.Render(w, "dashboard.html", map[string]any{
		"User":    user,
		"Tenants": tenants,
		"Host":    h.Config.Host,
	})
}

func (h *Handler) CreateBlog(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())

	slug := r.FormValue("slug")
	name := r.FormValue("name")

	if !isValidSlug(slug) {
		http.Error(w, "invalid slug: use only lowercase letters, numbers, and hyphens", http.StatusBadRequest)
		return
	}

	if name == "" {
		name = slug
	}

	err := h.DB.CreateTenant(r.Context(), &platform.Tenant{
		ID:     slug,
		UserID: user.ID,
		Name:   name,
	})
	if err != nil {
		slog.Error("create tenant failed", "err", err)
		http.Error(w, "failed to create blog (slug may be taken)", http.StatusBadRequest)
		return
	}

	_, err = h.Pool.Get(slug)
	if err != nil {
		slog.Error("init tenant db failed", "err", err)
	}

	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

func getOAuthState(r *http.Request) string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func (h *Handler) getProvider(name string) *auth.OAuthProvider {
	switch name {
	case "google":
		return h.Google
	case "github":
		return h.Github
	case "discord":
		return h.Discord
	}
	return nil
}

func (h *Handler) getProviderAuthURL(provider *auth.OAuthProvider, state string) string {
	return provider.Config.AuthCodeURL(state)
}

var slugRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,62}[a-z0-9]$`)

func isValidSlug(s string) bool {
	return slugRegex.MatchString(s)
}

func (h *Handler) GoogleLoginURL(state string) string {
	if h.Google == nil {
		return ""
	}
	return h.Google.Config.AuthCodeURL(state)
}

func (h *Handler) GithubLoginURL(state string) string {
	if h.Github == nil {
		return ""
	}
	return h.Github.Config.AuthCodeURL(state)
}

func setSessionCookie(w http.ResponseWriter, sessionID string, expires time.Time, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		Expires:  expires,
	})
}
