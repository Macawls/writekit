package api

import (
	"encoding/json"
	"net/http"

	"writekit/internal/httplog"
	"writekit/internal/tenant"
)

const graphMaxPages = 10000

type graphNode struct {
	ID           string   `json:"id"`
	Slug         string   `json:"slug"`
	Title        string   `json:"title"`
	Tags         []string `json:"tags"`
	CollectionID *string  `json:"collection_id,omitempty"`
	URL          string   `json:"url"`
	Visibility   string   `json:"visibility"`
}

type graphCollection struct {
	ID    string `json:"id"`
	Slug  string `json:"slug"`
	Title string `json:"title"`
}

type graphResponse struct {
	Nodes       []graphNode       `json:"nodes"`
	Collections []graphCollection `json:"collections"`
}

func (h *Handler) Graph(w http.ResponseWriter, r *http.Request) {
	log := httplog.FromContext(r.Context())
	user := userFromContext(r.Context())
	site, err := h.DB.GetTenantByUser(r.Context(), user.ID)
	if err != nil {
		log.Warn("graph: tenant lookup failed", "user_id", user.ID, "err", err)
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no site found"})
		return
	}

	db, err := h.Pool.Get(site.ID)
	if err != nil {
		log.Error("graph: open tenant db", "tenant", site.ID, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to open tenant db"})
		return
	}

	siteBaseURL := "https://" + site.ID + "." + h.Config.Host
	respondGraph(w, r, db, siteBaseURL)
}

func respondGraph(w http.ResponseWriter, r *http.Request, db *tenant.DB, siteBaseURL string) {
	log := httplog.FromContext(r.Context())

	pages, err := db.ListPages(r.Context(), tenant.PageFilter{Status: "published", Limit: graphMaxPages})
	if err != nil {
		log.Error("graph: list pages", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list pages"})
		return
	}

	nodes := make([]graphNode, 0, len(pages))
	for _, p := range pages {
		nodes = append(nodes, graphNode{
			ID:           p.ID,
			Slug:         p.Slug,
			Title:        p.Title,
			Tags:         parseTags(p.Tags),
			CollectionID: p.CollectionID,
			URL:          buildPageURL(siteBaseURL, db, r, p),
			Visibility:   p.Visibility,
		})
	}

	collections, err := db.ListCollections(r.Context())
	if err != nil {
		log.Warn("graph: list collections", "err", err)
	}
	graphCollections := make([]graphCollection, 0, len(collections))
	for _, c := range collections {
		graphCollections = append(graphCollections, graphCollection{ID: c.ID, Slug: c.Slug, Title: c.Title})
	}

	writeJSON(w, http.StatusOK, graphResponse{
		Nodes:       nodes,
		Collections: graphCollections,
	})
}

func parseTags(raw string) []string {
	var tags []string
	if raw == "" {
		return tags
	}
	if err := json.Unmarshal([]byte(raw), &tags); err != nil {
		return []string{}
	}
	return tags
}

func buildPageURL(base string, db *tenant.DB, r *http.Request, p tenant.Page) string {
	if p.CollectionID != nil && *p.CollectionID != "" {
		if col, err := db.GetCollection(r.Context(), *p.CollectionID); err == nil {
			return base + "/" + col.Slug + "/" + p.Slug
		}
	}
	return base + "/" + p.Slug
}
