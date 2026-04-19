package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"writekit/internal/auth"
	"writekit/internal/events"
	"writekit/internal/httplog"
	"writekit/internal/platform"
	"writekit/internal/tenant"
)

func (h *Handler) LocalRoutes(r chi.Router) {
	r.Use(auth.LocalAuth())
	r.Get("/api/me", h.MeLocal)
	r.Get("/api/site", h.GetSiteLocal)
	r.Put("/api/site/slug", h.UpdateSlugLocal)
	r.Put("/api/me", h.UpdateProfileLocal)
	r.Get("/api/team", h.ListTeamMembersLocal)
	r.Get("/api/graph", h.GraphLocal)
	r.Get("/api/local/info", h.LocalInfo)
	r.Get("/api/local/clients", h.LocalClients)
	r.Post("/api/local/clients/{id}/connect", h.LocalClientConnect)
	r.Post("/api/local/clients/{id}/disconnect", h.LocalClientDisconnect)
	r.Get("/api/local/settings", h.LocalGetSettings)
	r.Put("/api/local/settings", h.LocalPutSettings)
	r.Post("/api/local/pick-folder", h.LocalPickFolder)
	r.Get("/api/db/tables", h.DBTables)
	r.Get("/api/db/tables/{name}", h.DBTableRows)
	r.Post("/api/db/query", h.DBQuery)
	r.Get("/api/db/export", h.DBExport)
}

func (h *Handler) localSite() *platform.Tenant {
	tenants := auth.LocalTenants()
	if len(tenants) == 0 {
		return nil
	}
	t := tenants[0]
	return &t
}

func (h *Handler) MeLocal(w http.ResponseWriter, r *http.Request) {
	user := auth.LocalUser()
	site := h.localSite()
	writeJSON(w, http.StatusOK, map[string]any{
		"user": map[string]any{
			"id":         user.ID,
			"email":      user.Email,
			"name":       user.Name,
			"avatar_url": user.AvatarURL,
		},
		"site":         site,
		"subscription": nil,
		"role":         "owner",
	})
}

func (h *Handler) GetSiteLocal(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.localSite())
}

func (h *Handler) UpdateSlugLocal(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusBadRequest, map[string]string{
		"error": "renaming is not supported in desktop mode — your site is local-only",
	})
}

func (h *Handler) UpdateProfileLocal(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"name": body.Name})
}

func (h *Handler) ListTeamMembersLocal(w http.ResponseWriter, r *http.Request) {
	user := auth.LocalUser()
	writeJSON(w, http.StatusOK, []map[string]any{{
		"user_id":    user.ID,
		"email":      user.Email,
		"name":       user.Name,
		"avatar_url": user.AvatarURL,
		"role":       "owner",
		"created_at": time.Unix(0, 0).Format("2006-01-02T15:04:05Z"),
	}})
}

func (h *Handler) GraphLocal(w http.ResponseWriter, r *http.Request) {
	log := httplog.FromContext(r.Context())
	site := h.localSite()

	db, err := h.Pool.Get(site.ID)
	if err != nil {
		log.Error("graph: open tenant db", "tenant", site.ID, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to open tenant db"})
		return
	}

	pages, err := db.ListPages(r.Context(), tenant.PageFilter{Status: "published", Limit: graphMaxPages})
	if err != nil {
		log.Error("graph: list pages", "tenant", site.ID, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list pages"})
		return
	}

	siteBaseURL := h.Config.BaseURL + "/site"
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

	collections, _ := db.ListCollections(r.Context())
	graphCollections := make([]graphCollection, 0, len(collections))
	for _, c := range collections {
		graphCollections = append(graphCollections, graphCollection{ID: c.ID, Slug: c.Slug, Title: c.Title})
	}

	resp := graphResponse{
		Nodes:          nodes,
		Edges:          []graphEdge{},
		Collections:    graphCollections,
		TotalPageCount: len(nodes),
	}

	if h.Embedder != nil && h.Embedder.Enabled() {
		storageTag := h.Embedder.StorageTag()
		resp.Model = h.Embedder.Model()
		if storageTag != "" {
			embeddings, err := db.ListPageEmbeddings(r.Context(), storageTag)
			if err != nil {
				log.Warn("graph: list embeddings", "tenant", site.ID, "err", err)
			} else if len(embeddings) > 1 {
				normalize(embeddings)
				center(embeddings)
				normalize(embeddings)
				resp.Edges = computeEdges(embeddings)
			}
			resp.EmbeddedCount = len(embeddings)

			if resp.EmbeddedCount < resp.TotalPageCount && h.Bus != nil {
				go h.triggerBackfillLocal(site.ID, storageTag)
			}
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) triggerBackfillLocal(tenantID, model string) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("goroutine panic", "where", "graph backfill local", "panic", r)
		}
	}()
	db, err := h.Pool.Get(tenantID)
	if err != nil {
		return
	}
	ids, err := db.ListStalePageIDs(context.Background(), model)
	if err != nil {
		return
	}
	for _, id := range ids {
		h.Bus.Emit(events.Event{Type: events.PageUpdated, TenantID: tenantID, PageID: id})
	}
}
