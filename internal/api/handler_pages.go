package api

import (
	"net/http"
	"strconv"
	"strings"

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
	statusFilter := r.URL.Query().Get("status")
	colFilter := r.URL.Query().Get("collection")
	visFilter := r.URL.Query().Get("visibility")
	tagFilter := r.URL.Query().Get("tag")
	search := r.URL.Query().Get("q")
	sortBy := r.URL.Query().Get("sort")

	filter := tenant.PageFilter{
		Limit:      limit,
		Offset:     offset,
		Status:     statusFilter,
		Visibility: visFilter,
		Tag:        tagFilter,
		Search:     search,
		Sort:       sortBy,
	}
	switch colFilter {
	case "":
	case "none":
		empty := ""
		filter.CollectionID = &empty
	default:
		v := colFilter
		filter.CollectionID = &v
	}

	pages, err := db.ListPages(r.Context(), filter)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	total := countPagesFiltered(r, db, filter)

	cols, err := db.ListCollections(r.Context())
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

func countPagesFiltered(r *http.Request, db *tenant.DB, f tenant.PageFilter) int {
	var where []string
	var args []any
	if f.Status != "" {
		where = append(where, "status = ?")
		args = append(args, f.Status)
	}
	if f.Visibility != "" {
		where = append(where, "visibility = ?")
		args = append(args, f.Visibility)
	}
	if f.CollectionID != nil {
		if *f.CollectionID == "" {
			where = append(where, "collection_id IS NULL")
		} else {
			where = append(where, "collection_id = ?")
			args = append(args, *f.CollectionID)
		}
	}
	if f.Tag != "" {
		where = append(where, "tags LIKE ?")
		args = append(args, "%\""+f.Tag+"\"%")
	}
	if s := strings.TrimSpace(f.Search); s != "" {
		like := "%" + strings.ToLower(s) + "%"
		where = append(where, "(LOWER(title) LIKE ? OR LOWER(slug) LIKE ? OR LOWER(search_text) LIKE ?)")
		args = append(args, like, like, like)
	}
	q := "SELECT COUNT(*) FROM pages"
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	var n int
	_ = db.DB.QueryRowContext(r.Context(), q, args...).Scan(&n)
	return n
}
