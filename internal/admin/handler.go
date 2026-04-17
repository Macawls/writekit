package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"writekit/internal/config"
	"writekit/internal/email"
	"writekit/internal/httplog"
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
		log := httplog.FromContext(r.Context())
		cookie, err := r.Cookie("admin_session")
		if err != nil {
			log.Debug("admin auth: missing cookie", "err", err)
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		sess, err := h.DB.GetAdminSession(r.Context(), cookie.Value)
		if err != nil {
			log.Warn("admin auth: invalid session", "err", err)
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		ctx := httplog.WithFields(r.Context(), "admin_email", sess.Email)
		ctx = context.WithValue(ctx, adminEmailKey, sess.Email)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("admin: write json response", "err", err)
	}
}

func (h *Handler) AuthSend(w http.ResponseWriter, r *http.Request) {
	log := httplog.FromContext(r.Context())
	var body struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		log.Warn("admin auth send: decode body", "err", err)
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}

	email := strings.TrimSpace(strings.ToLower(body.Email))
	if !h.isAdminEmail(email) {
		log.Warn("admin auth send: non-admin email attempted", "email", email)
		writeJSON(w, http.StatusOK, map[string]string{"status": "sent"})
		return
	}

	ml, err := h.DB.CreateMagicLink(r.Context(), email)
	if err != nil {
		log.Error("create admin magic link", "email", email, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to send"})
		return
	}

	var link string
	if h.Config.Dev {
		link = fmt.Sprintf("http://admin.localhost:%d/admin/api/auth/verify?token=%s", h.Config.Port, ml.Token)
	} else {
		link = fmt.Sprintf("https://admin.%s/admin/api/auth/verify?token=%s", h.Config.Host, ml.Token)
	}

	log.Info("admin magic link created", "email", email)
	go func() {
		if err := h.Email.SendMagicLink(context.WithoutCancel(r.Context()), email, link); err != nil {
			slog.Error("send admin magic link email", "email", email, "err", err)
		}
	}()

	writeJSON(w, http.StatusOK, map[string]string{"status": "sent"})
}

func (h *Handler) AuthVerify(w http.ResponseWriter, r *http.Request) {
	log := httplog.FromContext(r.Context())
	token := r.URL.Query().Get("token")
	if token == "" {
		log.Debug("admin verify: missing token")
		http.Error(w, "missing token", http.StatusBadRequest)
		return
	}

	ml, err := h.DB.ConsumeMagicLink(r.Context(), token)
	if err != nil {
		log.Warn("admin verify: invalid magic link", "err", err)
		http.Redirect(w, r, "/?error=invalid", http.StatusSeeOther)
		return
	}

	if !h.isAdminEmail(ml.Email) {
		log.Warn("admin verify: magic link for non-admin email", "email", ml.Email)
		http.Redirect(w, r, "/?error=unauthorized", http.StatusSeeOther)
		return
	}

	sess, err := h.DB.CreateAdminSession(r.Context(), ml.Email)
	if err != nil {
		log.Error("create admin session", "email", ml.Email, "err", err)
		http.Error(w, "failed to create session", http.StatusInternalServerError)
		return
	}
	log.Info("admin signed in", "email", ml.Email)

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
		if derr := h.DB.DeleteAdminSession(r.Context(), cookie.Value); derr != nil {
			httplog.FromContext(r.Context()).Warn("admin logout: delete session", "err", derr)
		}
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
	log := httplog.FromContext(ctx)

	totalUsers, err := h.DB.CountUsers(ctx)
	if err != nil {
		log.Warn("admin stats: count users", "err", err)
	}
	totalTenants, err := h.DB.CountTenants(ctx)
	if err != nil {
		log.Warn("admin stats: count tenants", "err", err)
	}
	activeSubs, err := h.DB.CountActiveSubscriptions(ctx)
	if err != nil {
		log.Warn("admin stats: count active subs", "err", err)
	}
	recentUsers, err := h.DB.ListUsers(ctx, 10, 0)
	if err != nil {
		log.Warn("admin stats: list recent users", "err", err)
	}

	totalStorage, tenantSizes := h.calcStorage(ctx)

	writeJSON(w, http.StatusOK, map[string]any{
		"total_users":          totalUsers,
		"total_tenants":        totalTenants,
		"active_subscriptions": activeSubs,
		"recent_users":         recentUsers,
		"total_storage_bytes":  totalStorage,
		"tenant_storage":       tenantSizes,
	})
}

type tenantStorage struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Bytes int64  `json:"bytes"`
}

func (h *Handler) calcStorage(ctx context.Context) (int64, []tenantStorage) {
	tenants, err := h.DB.ListAllTenants(ctx)
	if err != nil {
		httplog.FromContext(ctx).Warn("admin stats: list all tenants", "err", err)
		return 0, nil
	}

	dataDir := h.Pool.DataDir()
	var total int64
	var sizes []tenantStorage

	for _, t := range tenants {
		var size int64
		dbPath := filepath.Join(dataDir, t.ID+".db")
		if info, err := os.Stat(dbPath); err == nil {
			size += info.Size()
		}
		if info, err := os.Stat(dbPath + "-wal"); err == nil {
			size += info.Size()
		}
		if info, err := os.Stat(dbPath + "-shm"); err == nil {
			size += info.Size()
		}
		total += size
		sizes = append(sizes, tenantStorage{ID: t.ID, Name: t.Name, Bytes: size})
	}

	return total, sizes
}

func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := httplog.FromContext(ctx)
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
			total, err = h.DB.CountUsers(ctx)
			if err != nil {
				log.Warn("admin list users: count", "err", err)
			}
		}
	}

	if err != nil {
		log.Error("admin list users", "q", q, "page", page, "err", err)
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
	log := httplog.FromContext(ctx)
	id := chi.URLParam(r, "id")

	user, err := h.DB.GetUser(ctx, id)
	if err != nil {
		log.Info("admin get user: not found", "user_id", id, "err", err)
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
		return
	}

	tenant, err := h.DB.GetTenantByUser(ctx, user.ID)
	if err != nil {
		log.Debug("admin get user: no tenant", "user_id", user.ID, "err", err)
	}
	accounts, err := h.DB.ListLinkedAccounts(ctx, user.ID)
	if err != nil {
		log.Warn("admin get user: list linked accounts", "user_id", user.ID, "err", err)
	}
	subscription, err := h.DB.GetSubscription(ctx, user.ID)
	if err != nil {
		log.Debug("admin get user: no subscription", "user_id", user.ID, "err", err)
	}

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
	log := httplog.FromContext(ctx)
	id := chi.URLParam(r, "id")

	tenantIDs, err := h.DB.DeleteUser(ctx, id)
	if err != nil {
		log.Error("admin delete user", "user_id", id, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete user"})
		return
	}

	for _, tid := range tenantIDs {
		if err := h.Pool.Delete(tid); err != nil {
			log.Error("admin delete tenant db", "tenant", tid, "err", err)
		}
	}

	log.Info("admin deleted user", "user_id", id, "tenants", tenantIDs)
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
