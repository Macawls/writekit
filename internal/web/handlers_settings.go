package web

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"writekit/internal/auth"
)

func (h *Handler) SettingsPage(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	accounts, err := h.DB.ListLinkedAccounts(r.Context(), user.ID)
	if err != nil {
		http.Error(w, "failed to load accounts", http.StatusInternalServerError)
		return
	}

	linked := map[string]bool{}
	for _, a := range accounts {
		linked[a.Provider] = true
	}

	h.Engine.Render(w, "settings.html", map[string]any{
		"User":           user,
		"Accounts":       accounts,
		"CanDisconnect":  len(accounts) > 1,
		"GoogleEnabled":  h.Config.GoogleClientID != "" && !linked["google"],
		"GithubEnabled":  h.Config.GithubClientID != "" && !linked["github"],
		"DiscordEnabled": h.Config.DiscordClientID != "" && !linked["discord"],
		"Error":          r.URL.Query().Get("error"),
	})
}

func (h *Handler) UnlinkAccount(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	accountID := chi.URLParam(r, "accountID")

	if err := h.DB.UnlinkAccount(r.Context(), user.ID, accountID); err != nil {
		http.Redirect(w, r, "/settings?error=cannot-unlink", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}

func (h *Handler) DeleteAccount(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())

	tenantIDs, err := h.DB.DeleteUser(r.Context(), user.ID)
	if err != nil {
		slog.Error("delete account failed", "err", err, "user", user.ID)
		http.Redirect(w, r, "/settings?error=delete-failed", http.StatusSeeOther)
		return
	}

	for _, tid := range tenantIDs {
		if err := h.Pool.Delete(tid); err != nil {
			slog.Error("delete tenant db failed", "err", err, "tenant", tid)
		}
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		Domain:   "." + h.Config.Host,
		HttpOnly: true,
		MaxAge:   -1,
	})

	http.Redirect(w, r, "/auth/login", http.StatusSeeOther)
}
