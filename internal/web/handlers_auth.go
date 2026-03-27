package web

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"net/mail"
	"net/url"
	"regexp"
	"strings"

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
	q := r.URL.Query()
	q.Del("oauth")
	h.Engine.Render(w, "login.html", map[string]any{
		"GoogleEnabled":  h.Config.GoogleClientID != "",
		"GithubEnabled":  h.Config.GithubClientID != "",
		"DiscordEnabled": h.Config.DiscordClientID != "",
		"OAuth":          true,
		"OAuthParams":    template.URL(q.Encode()),
	})
}

func (h *Handler) OAuthStart(w http.ResponseWriter, r *http.Request) {
	providerName := chi.URLParam(r, "provider")
	provider := h.getProvider(providerName)
	if provider == nil {
		http.Error(w, "unknown provider", http.StatusBadRequest)
		return
	}

	nonce := getOAuthState()

	cookieValue := providerName + ":" + nonce
	if r.URL.Query().Get("action") == "link" {
		cookieValue = providerName + ":link:" + nonce
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    cookieValue,
		Path:     "/",
		HttpOnly: true,
		Secure:   !h.Config.Dev,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   600,
	})

	state := nonce
	if r.URL.Query().Get("oauth") == "1" {
		q := r.URL.Query()
		q.Del("oauth")
		q.Del("action")
		state = nonce + "|" + q.Encode()
	}

	http.Redirect(w, r, provider.Config.AuthCodeURL(state), http.StatusFound)
}

func (h *Handler) OAuthCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	stateCookie, err := r.Cookie("oauth_state")
	if err != nil {
		http.Error(w, "invalid state", http.StatusBadRequest)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "oauth_state", MaxAge: -1, Path: "/"})

	cookieParts := strings.SplitN(stateCookie.Value, ":", 3)
	if len(cookieParts) < 2 {
		http.Error(w, "invalid state", http.StatusBadRequest)
		return
	}
	providerName := cookieParts[0]
	isLinkAction := len(cookieParts) == 3 && cookieParts[1] == "link"

	nonce := cookieParts[len(cookieParts)-1]
	if !strings.HasPrefix(state, nonce) {
		http.Error(w, "invalid state", http.StatusBadRequest)
		return
	}

	provider := h.getProvider(providerName)
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

if isLinkAction {
		h.handleOAuthLink(w, r, providerName, info)
		return
	}

user, isNew, err := h.findOrCreateUser(r, providerName, info)
	if err != nil {
		slog.Error("find or create user failed", "err", err)
		http.Error(w, "failed to save user", http.StatusInternalServerError)
		return
	}

	h.autoCreateTenant(r.Context(), user)

	if isNew {
		go func() {
			if err := h.Email.SendWelcome(context.WithoutCancel(r.Context()), user.Email, user.Name); err != nil {
				slog.Error("send welcome email", "err", err)
			}
		}()
	}

	h.createSessionAndRedirect(w, r, user.ID, state)
}


func (h *Handler) findOrCreateUser(r *http.Request, providerName string, info *auth.OAuthUserInfo) (*platform.User, bool, error) {
	ctx := r.Context()

	user, err := h.DB.FindUserByProvider(ctx, providerName, info.ID)
	if err == nil {
		return user, false, nil
	}

	if info.EmailVerified && info.Email != "" {
		user, err = h.DB.FindUserByVerifiedEmail(ctx, info.Email)
		if err == nil {
			_, linkErr := h.DB.LinkAccount(ctx, user.ID, providerName, info.ID, info.Email, true)
			if linkErr != nil {
				slog.Warn("auto-link failed", "err", linkErr, "provider", providerName, "user", user.ID)
			}
			return user, false, nil
		}
	}

	user, err = h.DB.CreateUser(ctx, info.Email, info.Name, info.AvatarURL)
	if err != nil {
		if info.Email != "" {
			user, err = h.DB.FindUserByVerifiedEmail(ctx, info.Email)
			if err == nil {
				h.DB.LinkAccount(ctx, user.ID, providerName, info.ID, info.Email, info.EmailVerified)
				return user, false, nil
			}
		}
		return nil, false, err
	}

	_, err = h.DB.LinkAccount(ctx, user.ID, providerName, info.ID, info.Email, info.EmailVerified)
	if err != nil {
		return nil, false, err
	}

	return user, true, nil
}


func (h *Handler) handleOAuthLink(w http.ResponseWriter, r *http.Request, providerName string, info *auth.OAuthUserInfo) {
	sessionCookie, err := r.Cookie("session")
	if err != nil {
		http.Redirect(w, r, "/auth/login", http.StatusSeeOther)
		return
	}

	sess, err := h.DB.GetSession(r.Context(), sessionCookie.Value)
	if err != nil {
		http.Redirect(w, r, "/auth/login", http.StatusSeeOther)
		return
	}

	existingUser, err := h.DB.FindUserByProvider(r.Context(), providerName, info.ID)
	if err == nil && existingUser.ID != sess.UserID {
		http.Redirect(w, r, "/settings?error=provider-taken", http.StatusSeeOther)
		return
	}

	if err == nil {
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
		return
	}

	_, err = h.DB.LinkAccount(r.Context(), sess.UserID, providerName, info.ID, info.Email, info.EmailVerified)
	if err != nil {
		slog.Error("link account failed", "err", err)
		http.Redirect(w, r, "/settings?error=link-failed", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}

func (h *Handler) createSessionAndRedirect(w http.ResponseWriter, r *http.Request, userID, state string) {
	sess, err := h.DB.CreateSession(r.Context(), userID)
	if err != nil {
		slog.Error("create session failed", "err", err)
		http.Error(w, "failed to create session", http.StatusInternalServerError)
		return
	}

	cookie := &http.Cookie{
		Name:     "session",
		Value:    sess.ID,
		Path:     "/",
		HttpOnly: true,
		Secure:   !h.Config.Dev,
		SameSite: http.SameSiteLaxMode,
		Expires:  sess.ExpiresAt,
	}
	if !h.Config.Dev {
		cookie.Domain = "." + h.Config.Host
	}
	http.SetCookie(w, cookie)

	loggedIn := &http.Cookie{
		Name:     "logged_in",
		Value:    "1",
		Path:     "/",
		Secure:   !h.Config.Dev,
		SameSite: http.SameSiteLaxMode,
		Expires:  sess.ExpiresAt,
	}
	if !h.Config.Dev {
		loggedIn.Domain = "." + h.Config.Host
	}
	http.SetCookie(w, loggedIn)

	if parts := strings.SplitN(state, "|", 2); len(parts) == 2 {
		http.Redirect(w, r, "/oauth/authorize?"+parts[1], http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, h.appURL(), http.StatusSeeOther)
}


func (h *Handler) MagicLinkRequest(w http.ResponseWriter, r *http.Request) {
	email := strings.TrimSpace(r.FormValue("email"))

	if _, err := mail.ParseAddress(email); err != nil {
		h.Engine.Render(w, "login.html", map[string]any{
			"GoogleEnabled":  h.Config.GoogleClientID != "",
			"GithubEnabled":  h.Config.GithubClientID != "",
			"DiscordEnabled": h.Config.DiscordClientID != "",
			"EmailError":     "Please enter a valid email address.",
			"Email":          email,
		})
		return
	}

	ml, err := h.DB.CreateMagicLink(r.Context(), email)
	if err != nil {
		slog.Error("create magic link failed", "err", err)
		http.Error(w, "something went wrong", http.StatusInternalServerError)
		return
	}

	var link string
	if h.Config.Dev {
		link = fmt.Sprintf("http://localhost:%d/auth/magic-link/verify?token=%s", h.Config.Port, ml.Token)
	} else {
		link = fmt.Sprintf("https://%s/auth/magic-link/verify?token=%s", h.Config.Host, ml.Token)
	}

	if oauthParams := r.FormValue("oauth_params"); oauthParams != "" {
		link += "&oauth_params=" + url.QueryEscape(oauthParams)
	}

	go func() {
		if err := h.Email.SendMagicLink(context.WithoutCancel(r.Context()), email, link); err != nil {
			slog.Error("send magic link email", "err", err)
		}
	}()

	h.Engine.Render(w, "magic_link_sent.html", map[string]any{
		"Email": email,
	})
}


func (h *Handler) MagicLinkVerify(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "missing token", http.StatusBadRequest)
		return
	}

	ml, err := h.DB.ConsumeMagicLink(r.Context(), token)
	if err != nil {
		slog.Warn("invalid magic link", "err", err)
		h.Engine.Render(w, "magic_link_sent.html", map[string]any{
			"Error": "This link is invalid or has expired. Please request a new one.",
		})
		return
	}

	user, err := h.DB.FindUserByVerifiedEmail(r.Context(), ml.Email)
	isNew := false
	if err != nil {
		user, err = h.DB.CreateUser(r.Context(), ml.Email, "", "")
		if err != nil {
			slog.Error("create user from magic link", "err", err)
			http.Error(w, "something went wrong", http.StatusInternalServerError)
			return
		}
		isNew = true
	}

	hasEmail, _ := h.DB.HasLinkedProvider(r.Context(), user.ID, "email")
	if !hasEmail {
		h.DB.LinkAccount(r.Context(), user.ID, "email", ml.Email, ml.Email, true)
	}

	h.autoCreateTenant(r.Context(), user)

	if isNew {
		go func() {
			if err := h.Email.SendWelcome(context.WithoutCancel(r.Context()), user.Email, user.Name); err != nil {
				slog.Error("send welcome email", "err", err)
			}
		}()
	}

	state := ""
	if oauthParams := r.URL.Query().Get("oauth_params"); oauthParams != "" {
		state = "x|" + oauthParams
	}

	h.createSessionAndRedirect(w, r, user.ID, state)
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session")
	if err == nil {
		h.DB.DeleteSession(r.Context(), cookie.Value)
	}

	logoutCookie := &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	}
	if !h.Config.Dev {
		logoutCookie.Domain = "." + h.Config.Host
	}
	http.SetCookie(w, logoutCookie)

	clearLoggedIn := &http.Cookie{
		Name:   "logged_in",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	}
	if !h.Config.Dev {
		clearLoggedIn.Domain = "." + h.Config.Host
	}
	http.SetCookie(w, clearLoggedIn)

	http.Redirect(w, r, "/auth/login", http.StatusSeeOther)
}

func (h *Handler) OAuthAuthorize(w http.ResponseWriter, r *http.Request) {
	authReq := auth.ParseAuthRequest(r)

	if err := h.MCPAuth.ValidateAuthRequest(r, authReq); err != nil {
		slog.Warn("oauth authorize failed", "err", err, "path", r.URL.Path)
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

	code, err := h.MCPAuth.IssueAuthCode(r, user.ID, authReq)
	if err != nil {
		slog.Error("issue auth code failed", "err", err)
		http.Error(w, "failed to issue code", http.StatusInternalServerError)
		return
	}
	redirectURL := auth.BuildRedirectURL(authReq.RedirectURI, code, authReq.State)
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

func (h *Handler) Home(w http.ResponseWriter, r *http.Request) {
	h.Engine.Render(w, "landing.html", nil)
}

func (h *Handler) appURL() string {
	if h.Config.Dev {
		return fmt.Sprintf("http://app.localhost:%d", h.Config.Port)
	}
	return "https://app." + h.Config.Host
}

func getOAuthState() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
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

var setupSlugRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,62}[a-z0-9]$`)

func slugifyForSubdomain(s string) string {
	s = strings.ToLower(s)
	s = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 64 {
		s = s[:64]
		s = strings.TrimRight(s, "-")
	}
	return s
}

func (h *Handler) getSessionUser(r *http.Request) (*platform.User, error) {
	cookie, err := r.Cookie("session")
	if err != nil {
		return nil, err
	}
	sess, err := h.DB.GetSession(r.Context(), cookie.Value)
	if err != nil {
		return nil, err
	}
	return h.DB.GetUser(r.Context(), sess.UserID)
}

func (h *Handler) AuthMe(w http.ResponseWriter, r *http.Request) {
	user, err := h.getSessionUser(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"name":       user.Name,
		"avatar_url": user.AvatarURL,
	})
}

var reservedSlugs = map[string]bool{"app": true, "www": true, "api": true, "admin": true}

func (h *Handler) autoCreateTenant(ctx context.Context, user *platform.User) {
	existing, _ := h.DB.GetTenantByUser(ctx, user.ID)
	if existing != nil {
		return
	}

	slug := slugifyForSubdomain(user.Name)
	if slug == "" {
		parts := strings.SplitN(user.Email, "@", 2)
		slug = slugifyForSubdomain(parts[0])
	}
	if slug == "" || len(slug) < 3 {
		slug = "site-" + slug
	}

	if reservedSlugs[slug] {
		slug = slug + "-blog"
	}

	// If slug is taken, append random suffix
	base := slug
	for i := 0; i < 5; i++ {
		if _, err := h.DB.GetTenant(ctx, slug); err != nil {
			break // not found, available
		}
		b := make([]byte, 3)
		rand.Read(b)
		slug = base + "-" + hex.EncodeToString(b)
	}

	name := user.Name
	if name == "" {
		name = slug
	}

	if err := h.DB.CreateTenant(ctx, &platform.Tenant{
		ID:     slug,
		UserID: user.ID,
		Name:   name,
	}); err != nil {
		slog.Error("auto-create tenant failed", "err", err, "slug", slug, "user", user.ID)
		return
	}

	if _, err := h.Pool.Get(slug); err != nil {
		slog.Error("init tenant db failed", "err", err)
	}

	slog.Info("auto-created tenant", "slug", slug, "user", user.ID)
}
