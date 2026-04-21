package site

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"writekit/internal/auth"
	"writekit/internal/events"
	"writekit/internal/httplog"
	"writekit/internal/tenant"
)

const indexPageSize = 5

func (h *Handler) isTeamMember(r *http.Request, tenantID string) bool {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		return false
	}
	if h.Config.Local {
		return user.ID == auth.LocalUserID && tenantID == auth.LocalTenantID
	}
	member, err := h.PlatformDB.GetTeamMember(r.Context(), tenantID, user.ID)
	if err != nil {
		httplog.FromContext(r.Context()).Debug("site: team member lookup failed", "tenant", tenantID, "user_id", user.ID, "err", err)
		return false
	}
	return member != nil
}

func visibilityRank(v string) int {
	switch v {
	case "private":
		return 2
	case "unlisted":
		return 1
	default:
		return 0
	}
}

func effectiveVisibility(collection, page string) string {
	if visibilityRank(collection) >= visibilityRank(page) {
		if collection == "" {
			return "public"
		}
		return collection
	}
	return page
}

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
	log := httplog.FromContext(r.Context())
	db, tenantID, err := h.getTenantDB(r)
	if err != nil {
		log.Warn("sitemap: tenant not found", "host", r.Host, "err", err)
		http.Error(w, "site not found", http.StatusNotFound)
		return
	}

	pages, err := db.ListPages(r.Context(), tenant.PageFilter{Status: "published", Visibility: "public", Limit: 1000})
	if err != nil {
		log.Error("sitemap: list pages", "tenant", tenantID, "err", err)
	}
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
	if h.Config.Local {
		db, err := h.Pool.Get(auth.LocalTenantID)
		if err != nil {
			return nil, "", fmt.Errorf("open local tenant db: %w", err)
		}
		return db, auth.LocalTenantID, nil
	}

	host := r.Host
	if i := strings.LastIndex(host, ":"); i > 0 {
		host = host[:i]
	}

	tenantID := strings.TrimSuffix(host, "."+h.Config.Host)
	if tenantID == host || tenantID == "" {
		return nil, "", fmt.Errorf("invalid tenant host: %s", host)
	}

	if _, err := h.PlatformDB.GetTenant(r.Context(), tenantID); err != nil {
		return nil, "", fmt.Errorf("tenant %s not found: %w", tenantID, err)
	}

	db, err := h.Pool.Get(tenantID)
	if err != nil {
		return nil, "", fmt.Errorf("open tenant db %s: %w", tenantID, err)
	}
	return db, tenantID, nil
}

func (h *Handler) Index(w http.ResponseWriter, r *http.Request) {
	log := httplog.FromContext(r.Context())
	db, tenantID, err := h.getTenantDB(r)
	if err != nil {
		log.Info("site index: tenant not found", "host", r.Host, "err", err)
		http.Error(w, "site not found", http.StatusNotFound)
		return
	}

	collections, err := db.ListCollections(r.Context())
	if err != nil {
		log.Error("site index: list collections", "tenant", tenantID, "err", err)
	}

	pageNum, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if pageNum < 1 {
		pageNum = 1
	}
	totalStandalone, err := db.CountStandalonePages(r.Context(), "published")
	if err != nil {
		log.Warn("site index: count standalone pages", "tenant", tenantID, "err", err)
	}
	totalPages := (totalStandalone + indexPageSize - 1) / indexPageSize
	if totalPages < 1 {
		totalPages = 1
	}
	if pageNum > totalPages {
		pageNum = totalPages
	}

	standalone := ""
	pages, err := db.ListPages(r.Context(), tenant.PageFilter{
		Status:       "published",
		CollectionID: &standalone,
		Limit:        indexPageSize,
		Offset:       (pageNum - 1) * indexPageSize,
	})
	if err != nil {
		log.Error("site index: list pages", "tenant", tenantID, "err", err)
	}
	settings, err := db.GetSettings(r.Context())
	if err != nil {
		log.Warn("site index: get settings", "tenant", tenantID, "err", err)
	}

	isMember := h.isTeamMember(r, tenantID)

	// Include draft pages for team members, only on page 1
	if isMember && pageNum == 1 {
		drafts, err := db.ListPages(r.Context(), tenant.PageFilter{Status: "draft", CollectionID: &standalone, Limit: indexPageSize})
		if err != nil {
			log.Error("site index: list draft pages", "tenant", tenantID, "err", err)
		}
		pages = append(drafts, pages...)
	}

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
		count, err := db.CountCollectionPages(r.Context(), c.ID)
		if err != nil {
			log.Warn("site index: count collection pages", "tenant", tenantID, "collection", c.ID, "err", err)
		}
		collectionData[i] = map[string]any{
			"Collection": c,
			"PageCount":  count,
		}
	}

	prevPage := 0
	if pageNum > 1 {
		prevPage = pageNum - 1
	}
	nextPage := 0
	if pageNum < totalPages {
		nextPage = pageNum + 1
	}

	h.Engine.Render(w, "index.html", map[string]any{
		"Collections":     collectionData,
		"Pages":           visiblePages,
		"Settings":        settings,
		"TenantID":        tenantID,
		"Host":            h.Config.Host,
		"PageDescription": settings["description"],
		"IsMember":        isMember,
		"Page":            pageNum,
		"TotalPages":      totalPages,
		"PrevPage":        prevPage,
		"NextPage":        nextPage,
	})
}

func (h *Handler) PageOrCollection(w http.ResponseWriter, r *http.Request) {
	db, tenantID, err := h.getTenantDB(r)
	if err != nil {
		httplog.FromContext(r.Context()).Warn("page: tenant not found", "err", err, "host", r.Host)
		http.Error(w, "site not found", http.StatusNotFound)
		return
	}

	slug := chi.URLParam(r, "slug")
	httplog.FromContext(r.Context()).Debug("page request", "tenant", tenantID, "slug", slug)
	settings, err := db.GetSettings(r.Context())
	if err != nil {
		httplog.FromContext(r.Context()).Warn("page: get settings", "tenant", tenantID, "err", err)
	}

	collection, err := db.GetCollectionBySlug(r.Context(), slug)
	if err == nil {
		// Check collection visibility
		if !h.canView(r, tenantID, collection.Visibility) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		isMember := h.isTeamMember(r, tenantID)
		pages, err := db.ListCollectionPages(r.Context(), collection.ID, collection.SortOrder, isMember)
		if err != nil {
			httplog.FromContext(r.Context()).Error("list collection pages", "tenant", tenantID, "collection", collection.ID, "err", err)
		}

		var visiblePages []tenant.Page
		for _, p := range pages {
			switch p.Visibility {
			case "public", "unlisted":
				p.Visibility = effectiveVisibility(collection.Visibility, p.Visibility)
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
		httplog.FromContext(r.Context()).Info("page not found", "tenant", tenantID, "slug", slug, "err", err)
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if page.Status != "published" {
		if !h.isTeamMember(r, tenantID) {
			httplog.FromContext(r.Context()).Info("page not published", "tenant", tenantID, "slug", slug, "status", page.Status)
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
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
		"OGImageURL":      h.OGImageURL(tenantID, "", page.Slug),
	})
}

func (h *Handler) CollectionPage(w http.ResponseWriter, r *http.Request) {
	log := httplog.FromContext(r.Context())
	db, tenantID, err := h.getTenantDB(r)
	if err != nil {
		log.Info("collection page: tenant not found", "host", r.Host, "err", err)
		http.Error(w, "site not found", http.StatusNotFound)
		return
	}

	collectionSlug := chi.URLParam(r, "collection")
	pageSlug := chi.URLParam(r, "page")

	collection, err := db.GetCollectionBySlug(r.Context(), collectionSlug)
	if err != nil {
		log.Info("collection page: collection not found", "tenant", tenantID, "slug", collectionSlug, "err", err)
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	// Private collection cascades — all pages are private
	if !h.canView(r, tenantID, collection.Visibility) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	page, err := db.GetPageInCollection(r.Context(), collection.ID, pageSlug)
	if err != nil {
		log.Info("collection page: page not found", "tenant", tenantID, "collection", collection.ID, "slug", pageSlug, "err", err)
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if page.Status != "published" && !h.isTeamMember(r, tenantID) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	// Check page-level visibility too
	if !h.canView(r, tenantID, page.Visibility) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	settings, err := db.GetSettings(r.Context())
	if err != nil {
		log.Warn("collection page: get settings", "tenant", tenantID, "err", err)
	}

	h.Engine.Render(w, "page.html", map[string]any{
		"Page":            page,
		"PageTitle":       page.Title,
		"PageDescription": page.Excerpt,
		"Collection":      collection,
		"Settings":        settings,
		"TenantID":        tenantID,
		"Host":            h.Config.Host,
		"OGImageURL":      h.OGImageURL(tenantID, collection.Slug, page.Slug),
	})
}

func (h *Handler) RawMarkdown(w http.ResponseWriter, r *http.Request) {
	log := httplog.FromContext(r.Context())
	db, tenantID, err := h.getTenantDB(r)
	if err != nil {
		log.Info("raw md: tenant not found", "host", r.Host, "err", err)
		http.Error(w, "site not found", http.StatusNotFound)
		return
	}

	slug := strings.TrimSuffix(chi.URLParam(r, "slug"), ".md")
	page, err := db.GetStandalonePageBySlug(r.Context(), slug)
	if err != nil || page.Status != "published" {
		if err != nil {
			log.Info("raw md: page not found", "tenant", tenantID, "slug", slug, "err", err)
		}
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if !h.canView(r, tenantID, page.Visibility) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	if _, err := w.Write([]byte(page.Content)); err != nil {
		log.Warn("raw md: write body", "tenant", tenantID, "slug", slug, "err", err)
	}
}

func (h *Handler) RawCollectionMarkdown(w http.ResponseWriter, r *http.Request) {
	log := httplog.FromContext(r.Context())
	db, tenantID, err := h.getTenantDB(r)
	if err != nil {
		log.Info("raw col md: tenant not found", "host", r.Host, "err", err)
		http.Error(w, "site not found", http.StatusNotFound)
		return
	}

	collectionSlug := chi.URLParam(r, "collection")
	pageSlug := strings.TrimSuffix(chi.URLParam(r, "page"), ".md")

	collection, err := db.GetCollectionBySlug(r.Context(), collectionSlug)
	if err != nil {
		log.Info("raw col md: collection not found", "tenant", tenantID, "slug", collectionSlug, "err", err)
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if !h.canView(r, tenantID, collection.Visibility) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	page, err := db.GetPageInCollection(r.Context(), collection.ID, pageSlug)
	if err != nil || page.Status != "published" {
		if err != nil {
			log.Info("raw col md: page not found", "tenant", tenantID, "collection", collection.ID, "slug", pageSlug, "err", err)
		}
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if !h.canView(r, tenantID, page.Visibility) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	if _, err := w.Write([]byte(page.Content)); err != nil {
		log.Warn("raw col md: write body", "tenant", tenantID, "slug", pageSlug, "err", err)
	}
}

type searchResult struct {
	Title       string `json:"title"`
	TitleHTML   string `json:"titleHtml"`
	Slug        string `json:"slug"`
	Collection  string `json:"collection,omitempty"`
	SnippetHTML string `json:"snippetHtml,omitempty"`
	URL         string `json:"url"`
}

func (h *Handler) SearchJSON(w http.ResponseWriter, r *http.Request) {
	log := httplog.FromContext(r.Context())
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	db, tenantID, err := h.getTenantDB(r)
	if err != nil {
		log.Info("search: tenant not found", "host", r.Host, "err", err)
		http.Error(w, `{"error":"site not found"}`, http.StatusNotFound)
		return
	}

	q := strings.TrimSpace(r.URL.Query().Get("q"))
	results := []searchResult{}
	start := time.Now()
	if q != "" {
		found, err := db.SearchPages(r.Context(), q)
		if err != nil {
			log.Warn("search: query failed", "tenant", tenantID, "q", q, "err", err)
		}
		isMember := h.isTeamMember(r, tenantID)
		for _, hit := range found {
			switch hit.Visibility {
			case "public":
			case "private":
				if !isMember {
					continue
				}
			default:
				continue
			}
			url := "/" + hit.Slug
			colSlug := ""
			if hit.CollectionID != nil && *hit.CollectionID != "" {
				col, err := db.GetCollection(r.Context(), *hit.CollectionID)
				if err == nil {
					if col.Visibility != "public" && !(col.Visibility == "private" && isMember) {
						continue
					}
					colSlug = col.Slug
					url = "/" + col.Slug + "/" + hit.Slug
				}
			}
			results = append(results, searchResult{
				Title:       hit.Title,
				TitleHTML:   hit.TitleHTML,
				Slug:        hit.Slug,
				Collection:  colSlug,
				SnippetHTML: hit.SnippetHTML,
				URL:         url,
			})
		}
	}

	elapsedMs := float64(time.Since(start).Microseconds()) / 1000.0
	if err := json.NewEncoder(w).Encode(map[string]any{
		"results":    results,
		"durationMs": elapsedMs,
	}); err != nil {
		log.Warn("search: encode json", "tenant", tenantID, "err", err)
	}
}

func (h *Handler) Preview(w http.ResponseWriter, r *http.Request) {
	log := httplog.FromContext(r.Context())
	db, tenantID, err := h.getTenantDB(r)
	if err != nil {
		log.Info("preview: tenant not found", "host", r.Host, "err", err)
		http.Error(w, "site not found", http.StatusNotFound)
		return
	}

	token := chi.URLParam(r, "token")
	pt, err := db.GetPreviewToken(r.Context(), token)
	if err != nil {
		log.Info("preview: invalid or expired token", "tenant", tenantID, "err", err)
		http.Error(w, "preview not found or expired", http.StatusNotFound)
		return
	}

	page, err := db.GetPage(r.Context(), pt.PageID)
	if err != nil {
		log.Warn("preview: page lookup failed", "tenant", tenantID, "page_id", pt.PageID, "err", err)
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
			} else {
				log.Warn("preview: get page version", "tenant", tenantID, "page_id", page.ID, "version", v, "err", err)
			}
		} else {
			log.Debug("preview: invalid version param", "raw", vStr, "err", err)
		}
	}

	settings, err := db.GetSettings(r.Context())
	if err != nil {
		log.Warn("preview: get settings", "tenant", tenantID, "err", err)
	}

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
	log := httplog.FromContext(r.Context())
	db, tenantID, err := h.getTenantDB(r)
	if err != nil {
		log.Info("preview sse: tenant not found", "host", r.Host, "err", err)
		http.Error(w, "site not found", http.StatusNotFound)
		return
	}

	token := chi.URLParam(r, "token")
	pt, err := db.GetPreviewToken(r.Context(), token)
	if err != nil {
		log.Info("preview sse: invalid or expired token", "tenant", tenantID, "err", err)
		http.Error(w, "preview not found or expired", http.StatusNotFound)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		log.Error("preview sse: responsewriter does not support flushing", "tenant", tenantID)
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
