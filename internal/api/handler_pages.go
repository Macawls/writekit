package api

import (
	"net/http"
	"strconv"

	"writekit/internal/httplog"
	"writekit/internal/tenant"
)

type pageListItem struct {
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	Slug         string   `json:"slug"`
	Status       string   `json:"status"`
	Visibility   string   `json:"visibility"`
	CollectionID *string  `json:"collection_id,omitempty"`
	Tags         []string `json:"tags"`
	UpdatedAt    string   `json:"updated_at"`
	PublishedAt  *string  `json:"published_at,omitempty"`
}

type pagesListResponse struct {
	Pages       []pageListItem    `json:"pages"`
	Collections []collectionLight `json:"collections"`
	Tags        []string          `json:"tags"`
	Total       int               `json:"total"`
	Limit       int               `json:"limit"`
	Offset      int               `json:"offset"`
}

type collectionLight struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Slug  string `json:"slug"`
}

func (h *Handler) ListPagesAPI(w http.ResponseWriter, r *http.Request) {
	log := httplog.FromContext(r.Context())
	tenantID, ok := h.resolveViewerTenantID(r)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no site"})
		return
	}
	db, err := h.Pool.Get(tenantID)
	if err != nil {
		log.Error("pages: open", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 200 {
		limit = 25
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}
	q := r.URL.Query()
	statusFilter := q.Get("status")
	visFilter := q.Get("visibility")
	search := q.Get("q")
	sortBy := q.Get("sort")

	var tags []string
	for _, t := range q["tag"] {
		if t != "" && t != "all" {
			tags = append(tags, t)
		}
	}
	var colIDs []string
	includeNone := false
	for _, c := range q["collection"] {
		switch c {
		case "", "all":
		case "none":
			includeNone = true
		default:
			colIDs = append(colIDs, c)
		}
	}

	filter := tenant.PageFilter{
		Limit:               limit,
		Offset:              offset,
		Status:              statusFilter,
		Visibility:          visFilter,
		Tags:                tags,
		Search:              search,
		Sort:                sortBy,
		CollectionIDs:       colIDs,
		IncludeNoCollection: includeNone,
	}

	pages, err := db.ListPages(r.Context(), filter)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	total, err := db.CountPagesFiltered(r.Context(), filter)
	if err != nil {
		log.Error("pages: count", "err", err)
		total = 0
	}

	cols, err := db.ListNonEmptyCollections(r.Context())
	if err != nil {
		cols = nil
	}

	allTags, err := db.ListAllTags(r.Context())
	if err != nil {
		allTags = nil
	}

	out := pagesListResponse{
		Pages:       make([]pageListItem, 0, len(pages)),
		Collections: make([]collectionLight, 0, len(cols)),
		Tags:        allTags,
		Total:       total,
		Limit:       limit,
		Offset:      offset,
	}
	for _, p := range pages {
		item := pageListItem{
			ID:           p.ID,
			Title:        p.Title,
			Slug:         p.Slug,
			Status:       p.Status,
			Visibility:   p.Visibility,
			CollectionID: p.CollectionID,
			Tags:         parseTags(p.Tags),
			UpdatedAt:    p.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		}
		if p.PublishedAt != nil {
			s := p.PublishedAt.Format("2006-01-02T15:04:05Z")
			item.PublishedAt = &s
		}
		out.Pages = append(out.Pages, item)
	}
	for _, c := range cols {
		out.Collections = append(out.Collections, collectionLight{ID: c.ID, Title: c.Title, Slug: c.Slug})
	}
	writeJSON(w, http.StatusOK, out)
}

