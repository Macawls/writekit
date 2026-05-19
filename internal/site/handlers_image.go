package site

import (
	"bytes"
	"errors"
	"net/http"

	"database/sql"

	"github.com/go-chi/chi/v5"
)

func (h *Handler) Image(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if len(id) != 64 {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	db, _, err := h.getTenantDB(r)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	img, err := db.GetImage(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/webp")
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("ETag", `"`+id+`"`)
	http.ServeContent(w, r, id+".webp", img.CreatedAt, bytes.NewReader(img.Bytes))
}
