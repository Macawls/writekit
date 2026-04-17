package site

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"writekit/internal/httplog"
	"writekit/internal/og"
	"writekit/internal/tenant"
)

const ogWordsPerMinute = 200

func (h *Handler) PageOG(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	h.renderOG(w, r, slug, "")
}

func (h *Handler) CollectionPageOG(w http.ResponseWriter, r *http.Request) {
	collectionSlug := chi.URLParam(r, "collection")
	pageSlug := chi.URLParam(r, "page")
	h.renderOG(w, r, pageSlug, collectionSlug)
}

func (h *Handler) renderOG(w http.ResponseWriter, r *http.Request, pageSlug, collectionSlug string) {
	log := httplog.FromContext(r.Context())

	if h.OG == nil {
		http.Error(w, "og renderer unavailable", http.StatusServiceUnavailable)
		return
	}

	db, tenantID, err := h.getTenantDB(r)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	var (
		page       *tenant.Page
		collection *tenant.Collection
	)

	if collectionSlug != "" {
		col, err := db.GetCollectionBySlug(r.Context(), collectionSlug)
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if !h.canView(r, tenantID, col.Visibility) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		p, err := db.GetPageInCollection(r.Context(), col.ID, pageSlug)
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		page = p
		collection = col
	} else {
		p, err := db.GetStandalonePageBySlug(r.Context(), pageSlug)
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		page = p
	}

	if page.Status != "published" && !h.isTeamMember(r, tenantID) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if !h.canView(r, tenantID, page.Visibility) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	etag := fmt.Sprintf(`"og-%s-v%d"`, page.ID, page.Version)
	if match := r.Header.Get("If-None-Match"); match == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	data := og.Data{
		Subdomain: tenantID + "." + h.Config.Host,
		Title:     page.Title,
		Subtitle:  page.Excerpt,
		Tags:      parseOGTags(page.Tags),
		DateText:  ogDateText(page),
		SlugPath:  ogSlugPath(collection, page),
	}

	img, err := h.OG.Render(data)
	if err != nil {
		log.Error("og render", "tenant", tenantID, "slug", pageSlug, "err", err)
		http.Error(w, "render failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Header().Set("ETag", etag)
	_, _ = w.Write(img)
}

func parseOGTags(raw string) []string {
	if raw == "" {
		return nil
	}
	var tags []string
	if err := json.Unmarshal([]byte(raw), &tags); err != nil {
		return nil
	}
	return tags
}

func ogDateText(p *tenant.Page) string {
	var dateStr string
	if p.PublishedAt != nil {
		dateStr = p.PublishedAt.Format("January 2, 2006")
	} else {
		dateStr = p.CreatedAt.Format("January 2, 2006")
	}
	readMins := ogReadTime(p.Content)
	if readMins > 0 {
		return fmt.Sprintf("%s · %d min read", dateStr, readMins)
	}
	return dateStr
}

func ogReadTime(content string) int {
	words := len(strings.Fields(content))
	if words == 0 {
		return 0
	}
	mins := words / ogWordsPerMinute
	if mins < 1 {
		return 1
	}
	return mins
}

func ogSlugPath(collection *tenant.Collection, p *tenant.Page) string {
	if collection != nil {
		return "/" + collection.Slug + "/" + p.Slug
	}
	return "/" + p.Slug
}

// OGImageURL builds the absolute URL for a page's OG image.
func (h *Handler) OGImageURL(tenantID string, collectionSlug, pageSlug string) string {
	var path string
	if collectionSlug != "" {
		path = "/og/" + collectionSlug + "/" + pageSlug + ".png"
	} else {
		path = "/og/" + pageSlug + ".png"
	}
	return "https://" + tenantID + "." + h.Config.Host + path
}

