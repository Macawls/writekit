package site

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"writekit/internal/httplog"
)

func (h *Handler) TagsIndex(w http.ResponseWriter, r *http.Request) {
	log := httplog.FromContext(r.Context())
	db, tenantID, err := h.getTenantDB(r)
	if err != nil {
		log.Info("tags index: tenant not found", "host", r.Host, "err", err)
		http.Error(w, "site not found", http.StatusNotFound)
		return
	}

	isMember := h.isTeamMember(r, tenantID)

	if !isMember {
		if html, err := db.GetTagIndexRender(r.Context()); err == nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write(html)
			return
		}
	}

	tags, err := db.ListTagCounts(r.Context(), isMember)
	if err != nil {
		log.Warn("tags index: list", "tenant", tenantID, "err", err)
	}

	settings, err := db.GetSettings(r.Context())
	if err != nil {
		log.Warn("tags index: get settings", "tenant", tenantID, "err", err)
	}

	h.Engine.Render(w, "tags-index.html", map[string]any{
		"Tags":            tags,
		"Settings":        settings,
		"TenantID":        tenantID,
		"Host":            h.Config.Host,
		"PageTitle":       "Tags",
		"PageDescription": "Browse pages by tag.",
	})
}

func (h *Handler) TagPage(w http.ResponseWriter, r *http.Request) {
	log := httplog.FromContext(r.Context())
	db, tenantID, err := h.getTenantDB(r)
	if err != nil {
		log.Info("tag page: tenant not found", "host", r.Host, "err", err)
		http.Error(w, "site not found", http.StatusNotFound)
		return
	}

	slug := chi.URLParam(r, "slug")
	if slug == "" {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	isMember := h.isTeamMember(r, tenantID)

	if !isMember {
		if html, _, err := db.GetTagRender(r.Context(), slug); err == nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write(html)
			return
		}
	}

	pages, displayName, err := db.PagesByTagSlug(r.Context(), slug, isMember)
	if err != nil {
		log.Warn("tag page: query", "tenant", tenantID, "slug", slug, "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if displayName == "" {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	settings, err := db.GetSettings(r.Context())
	if err != nil {
		log.Warn("tag page: get settings", "tenant", tenantID, "err", err)
	}

	h.Engine.Render(w, "tag.html", map[string]any{
		"Pages":           pages,
		"TagName":         displayName,
		"TagSlug":         slug,
		"Settings":        settings,
		"TenantID":        tenantID,
		"Host":            h.Config.Host,
		"PageTitle":       "#" + displayName,
		"PageDescription": "Pages tagged " + displayName + ".",
	})
}
