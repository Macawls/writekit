package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"writekit/internal/config"
	"writekit/internal/email"
	"writekit/internal/platform"
	"writekit/internal/tenant"
)

type Handler struct {
	DB     *platform.DB
	Pool   *tenant.Pool
	Config *config.Config
	Email  *email.Sender
}

func (h *Handler) Routes(r chi.Router) {
	r.Post("/admin/api/auth/send", h.AuthSend)
	r.Get("/admin/api/auth/verify", h.AuthVerify)

	r.Group(func(r chi.Router) {
		r.Use(h.authMiddleware)
		r.Get("/admin/api/me", h.Me)
		r.Post("/admin/api/auth/logout", h.Logout)
		r.Get("/admin/api/stats", h.Stats)
		r.Get("/admin/api/users", h.ListUsers)
		r.Get("/admin/api/users/{id}", h.GetUser)
		r.Delete("/admin/api/users/{id}", h.DeleteUser)
	})
}

type contextKey string

const adminEmailKey contextKey = "admin_email"

func (h *Handler) isAdminEmail(email string) bool {
	for _, e := range h.Config.AdminEmails {
		if strings.EqualFold(e, email) {
			return true
		}
	}
	return false
}

func (h *Handler) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("admin_session")
		if err != nil {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		sess, err := h.DB.GetAdminSession(r.Context(), cookie.Value)
		if err != nil {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), adminEmailKey, sess.Email)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func (h *Handler) AuthSend(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}

	email := strings.TrimSpace(strings.ToLower(body.Email))
	if !h.isAdminEmail(email) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "sent"})
		return
	}

	ml, err := h.DB.CreateMagicLink(r.Context(), email)
	if err != nil {
		slog.Error("create admin magic link", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to send"})
		return
	}

	var link string
	if h.Config.Dev {
		link = fmt.Sprintf("http://admin.localhost:%d/admin/api/auth/verify?token=%s", h.Config.Port, ml.Token)
	} else {
		link = fmt.Sprintf("https://admin.%s/admin/api/auth/verify?token=%s", h.Config.Host, ml.Token)
	}

	go func() {
		if err := h.Email.SendMagicLink(context.WithoutCancel(r.Context()), email, link); err != nil {
			slog.Error("send admin magic link email", "err", err)
		}
	}()

	writeJSON(w, http.StatusOK, map[string]string{"status": "sent"})
}

func (h *Handler) AuthVerify(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "missing token", http.StatusBadRequest)
		return
	}

	ml, err := h.DB.ConsumeMagicLink(r.Context(), token)
	if err != nil {
		http.Redirect(w, r, "/?error=invalid", http.StatusSeeOther)
		return
	}

	if !h.isAdminEmail(ml.Email) {
		http.Redirect(w, r, "/?error=unauthorized", http.StatusSeeOther)
		return
	}

	sess, err := h.DB.CreateAdminSession(r.Context(), ml.Email)
	if err != nil {
		slog.Error("create admin session", "err", err)
		http.Error(w, "failed to create session", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "admin_session",
		Value:    sess.Token,
		Path:     "/",
		HttpOnly: true,
		Secure:   !h.Config.Dev,
		SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	email := r.Context().Value(adminEmailKey).(string)
	writeJSON(w, http.StatusOK, map[string]string{"email": email})
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie("admin_session"); err == nil {
		h.DB.DeleteAdminSession(r.Context(), cookie.Value)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "admin_session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   !h.Config.Dev,
		SameSite: http.SameSiteLaxMode,
	})

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) Stats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	totalUsers, _ := h.DB.CountUsers(ctx)
	totalTenants, _ := h.DB.CountTenants(ctx)
	activeSubs, _ := h.DB.CountActiveSubscriptions(ctx)
	recentUsers, _ := h.DB.ListUsers(ctx, 10, 0)

	writeJSON(w, http.StatusOK, map[string]any{
		"total_users":          totalUsers,
		"total_tenants":        totalTenants,
		"active_subscriptions": activeSubs,
		"recent_users":         recentUsers,
	})
}

func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	perPage := 20
	q := r.URL.Query().Get("q")

	var users []platform.User
	var total int
	var err error

	if q != "" {
		users, err = h.DB.SearchUsers(ctx, q)
		total = len(users)
	} else {
		users, err = h.DB.ListUsers(ctx, perPage, (page-1)*perPage)
		if err == nil {
			total, _ = h.DB.CountUsers(ctx)
		}
	}

	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list users"})
		return
	}

	if users == nil {
		users = []platform.User{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"users":    users,
		"total":    total,
		"page":     page,
		"per_page": perPage,
	})
}

func (h *Handler) GetUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")

	user, err := h.DB.GetUser(ctx, id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
		return
	}

	tenant, _ := h.DB.GetTenantByUser(ctx, user.ID)
	accounts, _ := h.DB.ListLinkedAccounts(ctx, user.ID)
	subscription, _ := h.DB.GetSubscription(ctx, user.ID)

	if accounts == nil {
		accounts = []platform.LinkedAccount{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"user":            user,
		"tenant":          tenant,
		"linked_accounts": accounts,
		"subscription":    subscription,
	})
}

func (h *Handler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")

	tenantIDs, err := h.DB.DeleteUser(ctx, id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete user"})
		return
	}

	for _, tid := range tenantIDs {
		if err := h.Pool.Delete(tid); err != nil {
			slog.Error("delete tenant db", "tenant", tid, "err", err)
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
