package api

import (
	"net/http"

	"writekit/internal/httplog"
	"writekit/internal/markdown"
	"writekit/internal/tenant"
)

const embeddingSourceMaxChars = 16000

type embeddingSourceItem struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

func (h *Handler) EmbeddingSource(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r.Context())
	site, err := h.DB.GetTenantByUser(r.Context(), user.ID)
	if err != nil {
		httplog.FromContext(r.Context()).Warn("embedding source: tenant lookup", "user_id", user.ID, "err", err)
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no site found"})
		return
	}
	h.respondEmbeddingSource(w, r, site.ID)
}

func (h *Handler) EmbeddingSourceLocal(w http.ResponseWriter, r *http.Request) {
	site := h.localSite()
	h.respondEmbeddingSource(w, r, site.ID)
}

func (h *Handler) respondEmbeddingSource(w http.ResponseWriter, r *http.Request, tenantID string) {
	log := httplog.FromContext(r.Context())
	db, err := h.Pool.Get(tenantID)
	if err != nil {
		log.Error("embedding source: open tenant db", "tenant", tenantID, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to open tenant db"})
		return
	}
	pages, err := db.ListPages(r.Context(), tenant.PageFilter{Status: "published", Limit: graphMaxPages})
	if err != nil {
		log.Error("embedding source: list pages", "tenant", tenantID, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list pages"})
		return
	}
	out := make([]embeddingSourceItem, 0, len(pages))
	for _, p := range pages {
		text := markdown.PlainText(p.Content)
		if text == "" {
			text = p.Title
		}
		if len(text) > embeddingSourceMaxChars {
			text = text[:embeddingSourceMaxChars]
		}
		out = append(out, embeddingSourceItem{ID: p.ID, Text: text})
	}
	writeJSON(w, http.StatusOK, out)
}
