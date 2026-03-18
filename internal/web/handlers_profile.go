package web

import (
	"net/http"

	"writekit/internal/auth"
)

func (h *Handler) ProfilePage(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	h.Engine.Render(w, "profile.html", map[string]any{
		"User": user,
	})
}

func (h *Handler) ProfileUpdate(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	name := r.FormValue("name")

	if name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	if err := h.DB.UpdateUser(r.Context(), user.ID, name); err != nil {
		http.Error(w, "failed to update profile", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/profile", http.StatusSeeOther)
}
