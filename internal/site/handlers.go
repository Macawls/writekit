package site

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"writekit/internal/tenant"
	"github.com/oklog/ulid/v2"
)

func (h *Handler) getTenantDB(r *http.Request) (*tenant.DB, string, error) {
	host := r.Host
	if i := strings.LastIndex(host, ":"); i > 0 {
		host = host[:i]
	}

	tenantID := strings.TrimSuffix(host, "."+h.Config.Host)
	if tenantID == host || tenantID == "" {
		return nil, "", fmt.Errorf("invalid tenant host: %s", host)
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

	collectionData := make([]map[string]any, len(collections))
	for i, c := range collections {
		count, _ := db.CountCollectionPages(r.Context(), c.ID)
		collectionData[i] = map[string]any{
			"Collection": c,
			"PageCount":  count,
		}
	}

	h.Engine.Render(w, "index.html", map[string]any{
		"Collections": collectionData,
		"Pages":       pages,
		"Settings":    settings,
		"TenantID":    tenantID,
		"Host":        h.Config.Host,
	})
}

func (h *Handler) PageOrCollection(w http.ResponseWriter, r *http.Request) {
	db, tenantID, err := h.getTenantDB(r)
	if err != nil {
		http.Error(w, "site not found", http.StatusNotFound)
		return
	}

	slug := chi.URLParam(r, "slug")
	settings, _ := db.GetSettings(r.Context())

	collection, err := db.GetCollectionBySlug(r.Context(), slug)
	if err == nil {
		pages, _ := db.ListCollectionPages(r.Context(), collection.ID, collection.SortOrder)
		h.Engine.Render(w, "collection.html", map[string]any{
			"Collection": collection,
			"Pages":      pages,
			"Settings":   settings,
			"TenantID":   tenantID,
			"Host":       h.Config.Host,
		})
		return
	}

	page, err := db.GetStandalonePageBySlug(r.Context(), slug)
	if err != nil || page.Status != "published" {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	comments, _ := db.ListComments(r.Context(), page.ID)

	h.Engine.Render(w, "page.html", map[string]any{
		"Page":     page,
		"Comments": comments,
		"Settings": settings,
		"TenantID": tenantID,
		"Host":     h.Config.Host,
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

	page, err := db.GetPageInCollection(r.Context(), collection.ID, pageSlug)
	if err != nil || page.Status != "published" {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	comments, _ := db.ListComments(r.Context(), page.ID)
	settings, _ := db.GetSettings(r.Context())

	h.Engine.Render(w, "page.html", map[string]any{
		"Page":       page,
		"Collection": collection,
		"Comments":   comments,
		"Settings":   settings,
		"TenantID":   tenantID,
		"Host":       h.Config.Host,
	})
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
		pages, _ = db.SearchPages(r.Context(), q)
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

	settings, _ := db.GetSettings(r.Context())

	h.Engine.Render(w, "page.html", map[string]any{
		"Page":     page,
		"Settings": settings,
		"TenantID": tenantID,
		"Host":     h.Config.Host,
		"Preview":  true,
	})
}

func (h *Handler) submitComment(w http.ResponseWriter, r *http.Request, db *tenant.DB, tenantID string, page *tenant.Page, redirectPath string) {
	author := strings.TrimSpace(r.FormValue("author"))
	authorEmail := strings.TrimSpace(r.FormValue("email"))
	content := strings.TrimSpace(r.FormValue("content"))

	if author == "" || content == "" {
		http.Error(w, "author and content are required", http.StatusBadRequest)
		return
	}

	comment := &tenant.Comment{
		ID:      ulid.Make().String(),
		PageID:  page.ID,
		Author:  author,
		Email:   authorEmail,
		Content: content,
	}

	parentID := r.FormValue("parent_id")
	if parentID != "" {
		comment.ParentID = &parentID
	}

	if err := db.CreateComment(r.Context(), comment); err != nil {
		slog.Error("create comment", "err", err)
		http.Error(w, "failed to post comment", http.StatusInternalServerError)
		return
	}

	ctx := context.WithoutCancel(r.Context())
	go func() {
		t, err := h.PlatformDB.GetTenant(ctx, tenantID)
		if err != nil {
			return
		}
		owner, err := h.PlatformDB.GetUser(ctx, t.UserID)
		if err != nil {
			return
		}
		settings, _ := db.GetSettings(ctx)
		siteName := settings["title"]
		if siteName == "" {
			siteName = tenantID
		}
		pageURL := fmt.Sprintf("https://%s.%s%s", tenantID, h.Config.Host, redirectPath)
		if err := h.Email.SendCommentNotification(ctx, owner.Email, owner.Name, siteName, page.Title, author, content, pageURL); err != nil {
			slog.Error("send comment notification", "err", err)
		}
	}()

	http.Redirect(w, r, fmt.Sprintf("%s#comment-%s", redirectPath, comment.ID), http.StatusSeeOther)
}

func (h *Handler) SubmitComment(w http.ResponseWriter, r *http.Request) {
	db, tenantID, err := h.getTenantDB(r)
	if err != nil {
		http.Error(w, "site not found", http.StatusNotFound)
		return
	}

	slug := chi.URLParam(r, "slug")
	page, err := db.GetStandalonePageBySlug(r.Context(), slug)
	if err != nil || page.Status != "published" {
		http.Error(w, "page not found", http.StatusNotFound)
		return
	}

	h.submitComment(w, r, db, tenantID, page, "/"+slug)
}

func (h *Handler) SubmitCollectionComment(w http.ResponseWriter, r *http.Request) {
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

	page, err := db.GetPageInCollection(r.Context(), collection.ID, pageSlug)
	if err != nil || page.Status != "published" {
		http.Error(w, "page not found", http.StatusNotFound)
		return
	}

	h.submitComment(w, r, db, tenantID, page, "/"+collectionSlug+"/"+pageSlug)
}
