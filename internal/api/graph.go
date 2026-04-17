package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"math"
	"net/http"
	"sort"
	"sync"
	"time"

	"writekit/internal/events"
	"writekit/internal/httplog"
	"writekit/internal/tenant"
)

const (
	graphMaxPages            = 10000
	graphTopKNeighbors       = 4
	graphMinSimilarity       = 0.55
	graphBackfillMinInterval = 10 * time.Second
)

var (
	backfillMu       sync.Mutex
	backfillLastFire = map[string]time.Time{}
)

type graphNode struct {
	ID           string   `json:"id"`
	Slug         string   `json:"slug"`
	Title        string   `json:"title"`
	Tags         []string `json:"tags"`
	CollectionID *string  `json:"collection_id,omitempty"`
	URL          string   `json:"url"`
	Visibility   string   `json:"visibility"`
}

type graphEdge struct {
	Source string  `json:"source"`
	Target string  `json:"target"`
	Weight float32 `json:"weight"`
}

type graphResponse struct {
	Nodes           []graphNode `json:"nodes"`
	Edges           []graphEdge `json:"edges"`
	Model           string      `json:"model"`
	EmbeddedCount   int         `json:"embedded_count"`
	TotalPageCount  int         `json:"total_page_count"`
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

	pages, err := db.ListPages(r.Context(), tenant.PageFilter{Status: "published", Limit: graphMaxPages})
	if err != nil {
		log.Error("graph: list pages", "tenant", site.ID, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list pages"})
		return
	}

	nodes := make([]graphNode, 0, len(pages))
	siteBaseURL := "https://" + site.ID + "." + h.Config.Host
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

	resp := graphResponse{
		Nodes:          nodes,
		Edges:          []graphEdge{},
		TotalPageCount: len(nodes),
	}

	model := h.Embedder.Model()
	if h.Embedder.Enabled() && model != "" {
		resp.Model = model
		embeddings, err := db.ListPageEmbeddings(r.Context(), model)
		if err != nil {
			log.Warn("graph: list embeddings", "tenant", site.ID, "model", model, "err", err)
		} else if len(embeddings) > 1 {
			normalize(embeddings)
			resp.Edges = computeEdges(embeddings)
		}
		resp.EmbeddedCount = len(embeddings)

		if resp.EmbeddedCount < resp.TotalPageCount {
			go h.triggerBackfill(site.ID, model)
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) triggerBackfill(tenantID, model string) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("goroutine panic", "where", "graph backfill", "panic", r)
		}
	}()

	if h.Bus == nil {
		return
	}

	backfillMu.Lock()
	if last, ok := backfillLastFire[tenantID]; ok && time.Since(last) < graphBackfillMinInterval {
		backfillMu.Unlock()
		return
	}
	backfillLastFire[tenantID] = time.Now()
	backfillMu.Unlock()

	db, err := h.Pool.Get(tenantID)
	if err != nil {
		slog.Warn("graph backfill: open tenant db", "tenant", tenantID, "err", err)
		return
	}
	ids, err := db.ListStalePageIDs(context.Background(), model)
	if err != nil {
		slog.Warn("graph backfill: list stale page ids", "tenant", tenantID, "err", err)
		return
	}
	for _, id := range ids {
		h.Bus.Emit(events.Event{Type: events.PageUpdated, TenantID: tenantID, PageID: id})
	}
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

func normalize(embeddings []tenant.PageEmbedding) {
	for i := range embeddings {
		var sum float64
		for _, v := range embeddings[i].Vec {
			sum += float64(v) * float64(v)
		}
		if sum == 0 {
			continue
		}
		inv := float32(1.0 / math.Sqrt(sum))
		for j := range embeddings[i].Vec {
			embeddings[i].Vec[j] *= inv
		}
	}
}

type graphNeighbor struct {
	idx    int
	weight float32
}

func computeEdges(embeddings []tenant.PageEmbedding) []graphEdge {
	n := len(embeddings)

	edges := make([]graphEdge, 0, n*graphTopKNeighbors)
	seen := make(map[string]bool, n*graphTopKNeighbors)

	for i := range n {
		top := make([]graphNeighbor, 0, graphTopKNeighbors)
		for j := range n {
			if i == j {
				continue
			}
			sim := dot(embeddings[i].Vec, embeddings[j].Vec)
			if sim < graphMinSimilarity {
				continue
			}
			top = insertTop(top, graphNeighbor{idx: j, weight: sim})
		}
		for _, nb := range top {
			a, b := embeddings[i].PageID, embeddings[nb.idx].PageID
			key := edgeKey(a, b)
			if seen[key] {
				continue
			}
			seen[key] = true
			edges = append(edges, graphEdge{Source: a, Target: b, Weight: nb.weight})
		}
	}
	return edges
}

func insertTop(top []graphNeighbor, cand graphNeighbor) []graphNeighbor {
	top = append(top, cand)
	sort.Slice(top, func(i, j int) bool {
		return top[i].weight > top[j].weight
	})
	if len(top) > graphTopKNeighbors {
		top = top[:graphTopKNeighbors]
	}
	return top
}

func dot(a, b []float32) float32 {
	n := min(len(a), len(b))
	var sum float32
	for i := range n {
		sum += a[i] * b[i]
	}
	return sum
}

func edgeKey(a, b string) string {
	if a < b {
		return a + "\x00" + b
	}
	return b + "\x00" + a
}
