package blog

import (
	"encoding/xml"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"writekit/internal/tenant"
	"github.com/oklog/ulid/v2"
)

func (h *Handler) getTenantDB(r *http.Request) (*tenant.DB, string, error) {
	host := r.Host

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
		http.Error(w, "blog not found", http.StatusNotFound)
		return
	}

	posts, err := db.ListPosts(r.Context(), tenant.PostFilter{Status: "published", Limit: 20})
	if err != nil {
		slog.Error("list posts", "tenant", tenantID, "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	settings, _ := db.GetSettings(r.Context())

	h.Engine.Render(w, "index.html", map[string]any{
		"Posts":    posts,
		"Settings": settings,
		"TenantID": tenantID,
		"Host":     h.Config.Host,
	})
}

func (h *Handler) Post(w http.ResponseWriter, r *http.Request) {
	db, tenantID, err := h.getTenantDB(r)
	if err != nil {
		http.Error(w, "blog not found", http.StatusNotFound)
		return
	}

	slug := chi.URLParam(r, "slug")
	post, err := db.GetPostBySlug(r.Context(), slug)
	if err != nil || post.Status != "published" {
		http.Error(w, "post not found", http.StatusNotFound)
		return
	}

	comments, _ := db.ListComments(r.Context(), post.ID)
	settings, _ := db.GetSettings(r.Context())

	h.Engine.Render(w, "post.html", map[string]any{
		"Post":     post,
		"Comments": comments,
		"Settings": settings,
		"TenantID": tenantID,
		"Host":     h.Config.Host,
	})
}

func (h *Handler) Tag(w http.ResponseWriter, r *http.Request) {
	db, tenantID, err := h.getTenantDB(r)
	if err != nil {
		http.Error(w, "blog not found", http.StatusNotFound)
		return
	}

	tag := chi.URLParam(r, "tag")
	posts, err := db.ListPosts(r.Context(), tenant.PostFilter{Status: "published", Tag: tag, Limit: 50})
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	settings, _ := db.GetSettings(r.Context())

	h.Engine.Render(w, "tag.html", map[string]any{
		"Tag":      tag,
		"Posts":    posts,
		"Settings": settings,
		"TenantID": tenantID,
		"Host":     h.Config.Host,
	})
}

func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	db, tenantID, err := h.getTenantDB(r)
	if err != nil {
		http.Error(w, "blog not found", http.StatusNotFound)
		return
	}

	q := r.URL.Query().Get("q")
	var posts []tenant.Post
	if q != "" {
		posts, _ = db.SearchPosts(r.Context(), q)
	}

	settings, _ := db.GetSettings(r.Context())

	h.Engine.Render(w, "search.html", map[string]any{
		"Query":    q,
		"Posts":    posts,
		"Settings": settings,
		"TenantID": tenantID,
		"Host":     h.Config.Host,
	})
}

func (h *Handler) Preview(w http.ResponseWriter, r *http.Request) {
	db, tenantID, err := h.getTenantDB(r)
	if err != nil {
		http.Error(w, "blog not found", http.StatusNotFound)
		return
	}

	token := chi.URLParam(r, "token")
	pt, err := db.GetPreviewToken(r.Context(), token)
	if err != nil {
		http.Error(w, "preview not found or expired", http.StatusNotFound)
		return
	}

	post, err := db.GetPost(r.Context(), pt.PostID)
	if err != nil {
		http.Error(w, "post not found", http.StatusNotFound)
		return
	}

	settings, _ := db.GetSettings(r.Context())

	h.Engine.Render(w, "post.html", map[string]any{
		"Post":     post,
		"Settings": settings,
		"TenantID": tenantID,
		"Host":     h.Config.Host,
		"Preview":  true,
	})
}

func (h *Handler) SubmitComment(w http.ResponseWriter, r *http.Request) {
	db, tenantID, err := h.getTenantDB(r)
	if err != nil {
		http.Error(w, "blog not found", http.StatusNotFound)
		return
	}

	slug := chi.URLParam(r, "slug")
	post, err := db.GetPostBySlug(r.Context(), slug)
	if err != nil || post.Status != "published" {
		http.Error(w, "post not found", http.StatusNotFound)
		return
	}

	author := strings.TrimSpace(r.FormValue("author"))
	authorEmail := strings.TrimSpace(r.FormValue("email"))
	content := strings.TrimSpace(r.FormValue("content"))

	if author == "" || content == "" {
		http.Error(w, "author and content are required", http.StatusBadRequest)
		return
	}

	comment := &tenant.Comment{
		ID:      ulid.Make().String(),
		PostID:  post.ID,
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

	go func() {
		t, err := h.PlatformDB.GetTenant(r.Context(), tenantID)
		if err != nil {
			return
		}
		owner, err := h.PlatformDB.GetUser(r.Context(), t.UserID)
		if err != nil {
			return
		}
		settings, _ := db.GetSettings(r.Context())
		blogName := settings["title"]
		if blogName == "" {
			blogName = tenantID
		}
		postURL := fmt.Sprintf("https://%s.%s/posts/%s", tenantID, h.Config.Host, slug)
		if err := h.Email.SendCommentNotification(r.Context(), owner.Email, blogName, post.Title, author, content, postURL); err != nil {
			slog.Error("send comment notification", "err", err)
		}
	}()

	http.Redirect(w, r, fmt.Sprintf("/posts/%s#comment-%s", slug, comment.ID), http.StatusSeeOther)
}

type rssChannel struct {
	XMLName     xml.Name  `xml:"rss"`
	Version     string    `xml:"version,attr"`
	Channel     rssFeed   `xml:"channel"`
}

type rssFeed struct {
	Title       string    `xml:"title"`
	Link        string    `xml:"link"`
	Description string    `xml:"description"`
	Items       []rssItem `xml:"item"`
}

type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
	GUID        string `xml:"guid"`
}

func (h *Handler) RSS(w http.ResponseWriter, r *http.Request) {
	db, tenantID, err := h.getTenantDB(r)
	if err != nil {
		http.Error(w, "blog not found", http.StatusNotFound)
		return
	}

	posts, _ := db.ListPosts(r.Context(), tenant.PostFilter{Status: "published", Limit: 20})
	settings, _ := db.GetSettings(r.Context())

	blogURL := fmt.Sprintf("https://%s.%s", tenantID, h.Config.Host)

	items := make([]rssItem, len(posts))
	for i, p := range posts {
		pubDate := p.CreatedAt.Format(time.RFC1123Z)
		if p.PublishedAt != nil {
			pubDate = p.PublishedAt.Format(time.RFC1123Z)
		}
		items[i] = rssItem{
			Title:       p.Title,
			Link:        fmt.Sprintf("%s/posts/%s", blogURL, p.Slug),
			Description: p.Excerpt,
			PubDate:     pubDate,
			GUID:        p.ID,
		}
	}

	feed := rssChannel{
		Version: "2.0",
		Channel: rssFeed{
			Title:       settings["title"],
			Link:        blogURL,
			Description: settings["description"],
			Items:       items,
		},
	}

	w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
	xml.NewEncoder(w).Encode(feed)
}
