package site

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"writekit/internal/auth"
	"writekit/internal/events"
	"writekit/internal/tenant"
)

func (h *Handler) isTeamMember(r *http.Request, tenantID string) bool {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		return false
	}
	member, err := h.PlatformDB.GetTeamMember(r.Context(), tenantID, user.ID)
	return err == nil && member != nil
}

// canView checks if the current request can view content with the given visibility.
func (h *Handler) canView(r *http.Request, tenantID, visibility string) bool {
	switch visibility {
	case "public", "unlisted":
		return true
	case "private":
		return h.isTeamMember(r, tenantID)
	default:
		return true
	}
}

func (h *Handler) TenantRobotsTxt(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprint(w, "User-agent: *\nAllow: /\n")
}

func (h *Handler) TenantSitemap(w http.ResponseWriter, r *http.Request) {
	db, tenantID, err := h.getTenantDB(r)
	if err != nil {
		http.Error(w, "site not found", http.StatusNotFound)
		return
	}

	// Only public published pages in sitemap
	pages, _ := db.ListPages(r.Context(), tenant.PageFilter{Status: "published", Visibility: "public", Limit: 1000})
	baseURL := fmt.Sprintf("https://%s.%s", tenantID, h.Config.Host)

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>`+baseURL+`/</loc><priority>1.0</priority></url>
`)
	for _, p := range pages {
		loc := baseURL + "/" + p.Slug
		if p.CollectionID != nil && *p.CollectionID != "" {
			col, err := db.GetCollection(r.Context(), *p.CollectionID)
			if err == nil {
				// Skip pages in non-public collections
				if col.Visibility != "public" {
					continue
				}
				loc = baseURL + "/" + col.Slug + "/" + p.Slug
			}
		}
		lastmod := p.UpdatedAt.Format("2006-01-02")
		fmt.Fprintf(w, "  <url><loc>%s</loc><lastmod>%s</lastmod></url>\n", loc, lastmod)
	}
	fmt.Fprint(w, `</urlset>`)
}

func (h *Handler) getTenantDB(r *http.Request) (*tenant.DB, string, error) {
	host := r.Host
	if i := strings.LastIndex(host, ":"); i > 0 {
		host = host[:i]
	}

	tenantID := strings.TrimSuffix(host, "."+h.Config.Host)
	if tenantID == host || tenantID == "" {
		return nil, "", fmt.Errorf("invalid tenant host: %s", host)
	}

	if _, err := h.PlatformDB.GetTenant(r.Context(), tenantID); err != nil {
		return nil, "", fmt.Errorf("tenant not found: %s", tenantID)
	}

	db, err := h.Pool.Get(tenantID)
	if err != nil {
		return nil, "", err
	}
	return db, tenantID, nil
}

func (h *Handler) Index(w http.ResponseWriter, r *http.Request) {
	db, tenantID, err := h.getTenantDB(r)
	if err != nil {
		http.Error(w, "site not found", http.StatusNotFound)
		return
	}

	collections, err := db.ListCollections(r.Context())
	if err != nil {
		slog.Error("list collections", "tenant", tenantID, "err", err)
	}
	standalone := ""
	pages, err := db.ListPages(r.Context(), tenant.PageFilter{Status: "published", CollectionID: &standalone, Limit: 20})
	if err != nil {
		slog.Error("list pages", "tenant", tenantID, "err", err)
	}
	settings, _ := db.GetSettings(r.Context())

	isMember := h.isTeamMember(r, tenantID)

	// Filter collections: hide unlisted + private (show private only if team member)
	var visibleCollections []tenant.Collection
	for _, c := range collections {
		switch c.Visibility {
		case "public":
			visibleCollections = append(visibleCollections, c)
		case "private":
			if isMember {
				visibleCollections = append(visibleCollections, c)
			}
		// unlisted: hidden from index
		}
	}

	// Filter pages: hide unlisted + private (show private only if team member)
	var visiblePages []tenant.Page
	for _, p := range pages {
		switch p.Visibility {
		case "public":
			visiblePages = append(visiblePages, p)
		case "private":
			if isMember {
				visiblePages = append(visiblePages, p)
			}
		// unlisted: hidden from index
		}
	}

	collectionData := make([]map[string]any, len(visibleCollections))
	for i, c := range visibleCollections {
		count, _ := db.CountCollectionPages(r.Context(), c.ID)
		collectionData[i] = map[string]any{
			"Collection": c,
			"PageCount":  count,
		}
	}

	h.Engine.Render(w, "index.html", map[string]any{
		"Collections":     collectionData,
		"Pages":           visiblePages,
		"Settings":        settings,
		"TenantID":        tenantID,
		"Host":            h.Config.Host,
		"PageDescription": settings["description"],
		"IsMember":        isMember,
	})
}

func (h *Handler) PageOrCollection(w http.ResponseWriter, r *http.Request) {
	db, tenantID, err := h.getTenantDB(r)
	if err != nil {
		slog.Warn("page: tenant not found", "err", err, "host", r.Host)
		http.Error(w, "site not found", http.StatusNotFound)
		return
	}

	slug := chi.URLParam(r, "slug")
	slog.Info("page request", "tenant", tenantID, "slug", slug)
	settings, _ := db.GetSettings(r.Context())

	collection, err := db.GetCollectionBySlug(r.Context(), slug)
	if err == nil {
		// Check collection visibility
		if !h.canView(r, tenantID, collection.Visibility) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		pages, _ := db.ListCollectionPages(r.Context(), collection.ID, collection.SortOrder)

		// Filter pages by visibility within the collection
		isMember := h.isTeamMember(r, tenantID)
		var visiblePages []tenant.Page
		for _, p := range pages {
			switch p.Visibility {
			case "public", "unlisted":
				visiblePages = append(visiblePages, p)
			case "private":
				if isMember {
					visiblePages = append(visiblePages, p)
				}
			}
		}

		h.Engine.Render(w, "collection.html", map[string]any{
			"Collection": collection,
			"Pages":      visiblePages,
			"Settings":   settings,
			"TenantID":   tenantID,
			"Host":       h.Config.Host,
			"IsMember":   isMember,
		})
		return
	}

	page, err := db.GetStandalonePageBySlug(r.Context(), slug)
	if err != nil {
		slog.Warn("page not found", "tenant", tenantID, "slug", slug, "err", err)
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if page.Status != "published" {
		slog.Warn("page not published", "tenant", tenantID, "slug", slug, "status", page.Status)
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if !h.canView(r, tenantID, page.Visibility) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	h.Engine.Render(w, "page.html", map[string]any{
		"Page":            page,
		"PageTitle":       page.Title,
		"PageDescription": page.Excerpt,
		"Settings":        settings,
		"TenantID":        tenantID,
		"Host":            h.Config.Host,
	})
}

func (h *Handler) CollectionPage(w http.ResponseWriter, r *http.Request) {
	db, tenantID, err := h.getTenantDB(r)
	if err != nil {
		http.Error(w, "site not found", http.StatusNotFound)
		return
	}

	collectionSlug := chi.URLParam(r, "collection")
	pageSlug := chi.URLParam(r, "page")

	collection, err := db.GetCollectionBySlug(r.Context(), collectionSlug)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	// Private collection cascades — all pages are private
	if !h.canView(r, tenantID, collection.Visibility) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	page, err := db.GetPageInCollection(r.Context(), collection.ID, pageSlug)
	if err != nil || page.Status != "published" {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	// Check page-level visibility too
	if !h.canView(r, tenantID, page.Visibility) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	settings, _ := db.GetSettings(r.Context())

	h.Engine.Render(w, "page.html", map[string]any{
		"Page":            page,
		"PageTitle":       page.Title,
		"PageDescription": page.Excerpt,
		"Collection":      collection,
		"Settings":        settings,
		"TenantID":        tenantID,
		"Host":            h.Config.Host,
	})
}

func (h *Handler) RawMarkdown(w http.ResponseWriter, r *http.Request) {
	db, tenantID, err := h.getTenantDB(r)
	if err != nil {
		http.Error(w, "site not found", http.StatusNotFound)
		return
	}

	slug := strings.TrimSuffix(chi.URLParam(r, "slug"), ".md")
	page, err := db.GetStandalonePageBySlug(r.Context(), slug)
	if err != nil || page.Status != "published" {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if !h.canView(r, tenantID, page.Visibility) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Write([]byte(page.Content))
}

func (h *Handler) RawCollectionMarkdown(w http.ResponseWriter, r *http.Request) {
	db, tenantID, err := h.getTenantDB(r)
	if err != nil {
		http.Error(w, "site not found", http.StatusNotFound)
		return
	}

	collectionSlug := chi.URLParam(r, "collection")
	pageSlug := strings.TrimSuffix(chi.URLParam(r, "page"), ".md")

	collection, err := db.GetCollectionBySlug(r.Context(), collectionSlug)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if !h.canView(r, tenantID, collection.Visibility) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	page, err := db.GetPageInCollection(r.Context(), collection.ID, pageSlug)
	if err != nil || page.Status != "published" {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if !h.canView(r, tenantID, page.Visibility) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Write([]byte(page.Content))
}

func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	db, tenantID, err := h.getTenantDB(r)
	if err != nil {
		http.Error(w, "site not found", http.StatusNotFound)
		return
	}

	q := r.URL.Query().Get("q")
	var pages []tenant.Page
	if q != "" {
		results, _ := db.SearchPages(r.Context(), q)
		isMember := h.isTeamMember(r, tenantID)
		// Filter search results: never include unlisted, include private only for members
		for _, p := range results {
			switch p.Visibility {
			case "public":
				pages = append(pages, p)
			case "private":
				if isMember {
					pages = append(pages, p)
				}
			// unlisted: never in search results
			}
		}
	}

	settings, _ := db.GetSettings(r.Context())

	h.Engine.Render(w, "search.html", map[string]any{
		"Query":    q,
		"Pages":    pages,
		"Settings": settings,
		"TenantID": tenantID,
		"Host":     h.Config.Host,
	})
}

func (h *Handler) Preview(w http.ResponseWriter, r *http.Request) {
	db, tenantID, err := h.getTenantDB(r)
	if err != nil {
		http.Error(w, "site not found", http.StatusNotFound)
		return
	}

	token := chi.URLParam(r, "token")
	pt, err := db.GetPreviewToken(r.Context(), token)
	if err != nil {
		http.Error(w, "preview not found or expired", http.StatusNotFound)
		return
	}

	page, err := db.GetPage(r.Context(), pt.PageID)
	if err != nil {
		http.Error(w, "page not found", http.StatusNotFound)
		return
	}

	if vStr := r.URL.Query().Get("v"); vStr != "" {
		if v, err := strconv.Atoi(vStr); err == nil {
			pv, err := db.GetPageVersion(r.Context(), page.ID, v)
			if err == nil {
				page.Title = pv.Title
				page.Content = pv.Content
				page.ContentHTML = pv.ContentHTML
				page.Version = pv.Version
			}
		}
	}

	settings, _ := db.GetSettings(r.Context())

	h.Engine.Render(w, "page.html", map[string]any{
		"Page":         page,
		"Settings":     settings,
		"TenantID":     tenantID,
		"Host":         h.Config.Host,
		"Preview":      true,
		"PreviewToken": token,
	})
}

func (h *Handler) PreviewSSE(w http.ResponseWriter, r *http.Request) {
	db, _, err := h.getTenantDB(r)
	if err != nil {
		http.Error(w, "site not found", http.StatusNotFound)
		return
	}

	token := chi.URLParam(r, "token")
	pt, err := db.GetPreviewToken(r.Context(), token)
	if err != nil {
		http.Error(w, "preview not found or expired", http.StatusNotFound)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	pageID := pt.PageID
	type sseEvent struct {
		kind string // "saving" or "rendered"
	}
	ch := make(chan sseEvent, 2)

	contentSubID := h.Bus.On(events.PageContentSaved, func(e events.Event) {
		if e.PageID == pageID {
			select {
			case ch <- sseEvent{kind: "saving"}:
			default:
			}
		}
	})
	defer h.Bus.Off(events.PageContentSaved, contentSubID)

	updatedSubID := h.Bus.On(events.PageUpdated, func(e events.Event) {
		if e.PageID == pageID {
			select {
			case ch <- sseEvent{kind: "rendered"}:
			default:
			}
		}
	})
	defer h.Bus.Off(events.PageUpdated, updatedSubID)

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case evt := <-ch:
			fmt.Fprintf(w, "data: %s\n\n", evt.kind)
			flusher.Flush()
		}
	}
}
