package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"

	"writekit/internal/auth"
	"writekit/internal/events"
	"writekit/internal/markdown"
	"writekit/internal/tenant"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/oklog/ulid/v2"
)

func (s *Server) registerTools(mcpServer *mcp.Server) {
	mcpServer.AddTool(&mcp.Tool{
		Name: "create_post",
		Description: `Create a new blog post. The post starts as a draft — use publish_post to make it live.

**Content format:** Write the body in rich Markdown. Supported syntax:
- Headings (# through ######), **bold**, *italic*, ~~strikethrough~~
- Links: [text](url), images: ![alt](url)
- Code blocks with language tags (` + "```go, ```python" + `) — renders with syntax highlighting, language icon, and copy button
- Callout blocks: > [!NOTE], > [!TIP], > [!WARNING], > [!DANGER] for styled alert boxes
- Media embeds: <embed src="url" /> for YouTube, Spotify, SoundCloud, Twitter/X, GitHub Gists
- D2 diagrams: ` + "```d2" + ` code blocks for architecture/flow diagrams
- Tables (GFM), ordered/unordered lists, task lists, horizontal rules, footnotes ([^1])
- Raw HTML for advanced layouts

Returns: The created post with a preview URL you can share.`,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"title":     map[string]any{"type": "string", "description": "Post title"},
				"content":   map[string]any{"type": "string", "description": "Post body in rich Markdown. Use headings (##), bold, code blocks with language tags (```go), lists, links, blockquotes, callout blocks (> [!NOTE]), and embeds (<embed src=\"url\" />) to create professional, well-formatted content. Never write plain unformatted text."},
				"excerpt":   map[string]any{"type": "string", "description": "Short excerpt for listings (auto-generated if omitted)"},
				"tags":      map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Tags for categorization"},
				"slug":      map[string]any{"type": "string", "description": "URL slug (auto-generated from title if omitted)"},
				"tenant_id": map[string]any{"type": "string", "description": "Blog ID (only needed if you have multiple blogs)"},
			},
			"required": []string{"title", "content"},
		},
	}, s.createPost)

	mcpServer.AddTool(&mcp.Tool{
		Name: "update_post",
		Description: `Update an existing blog post. Only send the fields you want to change.

After updating, a new preview URL is generated so you can verify changes.`,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id":        map[string]any{"type": "string", "description": "Post ID"},
				"title":     map[string]any{"type": "string", "description": "New title"},
				"content":   map[string]any{"type": "string", "description": "New content in rich Markdown. Use headings, code blocks with language tags, callout blocks (> [!NOTE]), and embeds for professional formatting."},
				"excerpt":   map[string]any{"type": "string", "description": "New excerpt"},
				"tags":      map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "New tags"},
				"slug":      map[string]any{"type": "string", "description": "New URL slug"},
				"tenant_id": map[string]any{"type": "string", "description": "Blog ID (only needed if you have multiple blogs)"},
			},
			"required": []string{"id"},
		},
	}, s.updatePost)

	mcpServer.AddTool(&mcp.Tool{
		Name:        "delete_post",
		Description: "Permanently delete a post. This is irreversible — the post and all its comments will be removed.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id":        map[string]any{"type": "string", "description": "Post ID to delete"},
				"tenant_id": map[string]any{"type": "string", "description": "Blog ID (only needed if you have multiple blogs)"},
			},
			"required": []string{"id"},
		},
	}, s.deletePost)

	mcpServer.AddTool(&mcp.Tool{
		Name:        "publish_post",
		Description: "Publish a draft post, making it live on your blog. Returns the live URL.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id":        map[string]any{"type": "string", "description": "Post ID to publish"},
				"tenant_id": map[string]any{"type": "string", "description": "Blog ID (only needed if you have multiple blogs)"},
			},
			"required": []string{"id"},
		},
	}, s.publishPost)

	mcpServer.AddTool(&mcp.Tool{
		Name:        "unpublish_post",
		Description: "Revert a published post to draft status. It will no longer be visible on your blog.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id":        map[string]any{"type": "string", "description": "Post ID to unpublish"},
				"tenant_id": map[string]any{"type": "string", "description": "Blog ID (only needed if you have multiple blogs)"},
			},
			"required": []string{"id"},
		},
	}, s.unpublishPost)

	mcpServer.AddTool(&mcp.Tool{
		Name:        "list_posts",
		Description: "List blog posts. Filter by status (draft/published) or tag.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"status":    map[string]any{"type": "string", "enum": []string{"draft", "published"}, "description": "Filter by status"},
				"tag":       map[string]any{"type": "string", "description": "Filter by tag"},
				"limit":     map[string]any{"type": "integer", "description": "Max results (default 50)"},
				"tenant_id": map[string]any{"type": "string", "description": "Blog ID (only needed if you have multiple blogs)"},
			},
		},
	}, s.listPosts)

	mcpServer.AddTool(&mcp.Tool{
		Name:        "get_post",
		Description: "Get a single post with full content. Use this to read a post before editing.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id":        map[string]any{"type": "string", "description": "Post ID"},
				"slug":      map[string]any{"type": "string", "description": "Post slug (alternative to ID)"},
				"tenant_id": map[string]any{"type": "string", "description": "Blog ID (only needed if you have multiple blogs)"},
			},
		},
	}, s.getPost)

	mcpServer.AddTool(&mcp.Tool{
		Name:        "search_posts",
		Description: "Full-text search across post titles and content.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query":     map[string]any{"type": "string", "description": "Search query"},
				"tenant_id": map[string]any{"type": "string", "description": "Blog ID (only needed if you have multiple blogs)"},
			},
			"required": []string{"query"},
		},
	}, s.searchPosts)

}

func (s *Server) createPost(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	user := auth.UserFromContext(ctx)
	if user == nil {
		return toolError("not authenticated"), nil
	}

	var args struct {
		Title    string   `json:"title"`
		Content  string   `json:"content"`
		Excerpt  string   `json:"excerpt"`
		Tags     []string `json:"tags"`
		Slug     string   `json:"slug"`
		TenantID string   `json:"tenant_id"`
	}
	raw, _ := json.Marshal(req.Params.Arguments)
	json.Unmarshal(raw, &args)

	db, tenantID, err := s.resolveTenant(user.ID, args.TenantID)
	if err != nil {
		return toolError(err.Error()), nil
	}

	slug := args.Slug
	if slug == "" {
		slug = slugify(args.Title)
	}

	excerpt := args.Excerpt
	if excerpt == "" {
		excerpt = generateExcerpt(args.Content)
	}

	tagsJSON, _ := json.Marshal(args.Tags)
	if args.Tags == nil {
		tagsJSON = []byte("[]")
	}

	post := &tenant.Post{
		ID:          ulid.Make().String(),
		Title:       args.Title,
		Slug:        slug,
		Content:     args.Content,
		ContentHTML: renderContent(args.Content),
		Excerpt:     excerpt,
		Status:      "draft",
		Tags:        string(tagsJSON),
	}

	if err := db.CreatePost(ctx, post); err != nil {
		return toolError(fmt.Sprintf("failed to create post: %v", err)), nil
	}

	pt, err := db.CreatePreviewToken(ctx, post.ID, 24*time.Hour)
	if err != nil {
		return toolError("post created but failed to generate preview URL"), nil
	}

	s.Bus.Emit(events.Event{Type: events.PostCreated, TenantID: tenantID})

	result := fmt.Sprintf(`Post created as draft.

**Title:** %s
**ID:** %s
**Slug:** %s
**Preview:** %s

Tip: Open the preview URL to check formatting. Use update_post to refine before publishing.`,
		post.Title, post.ID, post.Slug, s.buildPreviewURL(tenantID, pt.Token))

	return toolResult(result), nil
}

func (s *Server) updatePost(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	user := auth.UserFromContext(ctx)
	if user == nil {
		return toolError("not authenticated"), nil
	}

	var args struct {
		ID       string   `json:"id"`
		Title    *string  `json:"title"`
		Content  *string  `json:"content"`
		Excerpt  *string  `json:"excerpt"`
		Tags     []string `json:"tags"`
		Slug     *string  `json:"slug"`
		TenantID string   `json:"tenant_id"`
	}
	raw, _ := json.Marshal(req.Params.Arguments)
	json.Unmarshal(raw, &args)

	db, tenantID, err := s.resolveTenant(user.ID, args.TenantID)
	if err != nil {
		return toolError(err.Error()), nil
	}

	post, err := db.GetPost(ctx, args.ID)
	if err != nil {
		return toolError("post not found"), nil
	}

	if args.Title != nil {
		post.Title = *args.Title
	}
	if args.Content != nil {
		post.Content = *args.Content
		post.ContentHTML = renderContent(*args.Content)
	}
	if args.Excerpt != nil {
		post.Excerpt = *args.Excerpt
	}
	if args.Tags != nil {
		tagsJSON, _ := json.Marshal(args.Tags)
		post.Tags = string(tagsJSON)
	}
	if args.Slug != nil {
		post.Slug = *args.Slug
	}

	if err := db.UpdatePost(ctx, post); err != nil {
		return toolError(fmt.Sprintf("failed to update: %v", err)), nil
	}

	s.Bus.Emit(events.Event{Type: events.PostUpdated, TenantID: tenantID})

	pt, _ := db.CreatePreviewToken(ctx, post.ID, 24*time.Hour)
	previewURL := ""
	if pt != nil {
		previewURL = s.buildPreviewURL(tenantID, pt.Token)
	}

	result := fmt.Sprintf("Post updated!\n\n**Title:** %s\n**Status:** %s", post.Title, post.Status)
	if post.Status == "published" {
		result += fmt.Sprintf("\n**Live URL:** %s", s.buildPostURL(tenantID, post.Slug))
	}
	if previewURL != "" {
		result += fmt.Sprintf("\n**Preview URL:** %s", previewURL)
	}

	return toolResult(result), nil
}

func (s *Server) deletePost(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	user := auth.UserFromContext(ctx)
	if user == nil {
		return toolError("not authenticated"), nil
	}

	var args struct {
		ID       string `json:"id"`
		TenantID string `json:"tenant_id"`
	}
	raw, _ := json.Marshal(req.Params.Arguments)
	json.Unmarshal(raw, &args)

	db, tenantID, err := s.resolveTenant(user.ID, args.TenantID)
	if err != nil {
		return toolError(err.Error()), nil
	}

	if err := db.DeletePost(ctx, args.ID); err != nil {
		return toolError(fmt.Sprintf("failed to delete: %v", err)), nil
	}

	s.Bus.Emit(events.Event{Type: events.PostDeleted, TenantID: tenantID})
	return toolResult("Post deleted permanently."), nil
}

func (s *Server) publishPost(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	user := auth.UserFromContext(ctx)
	if user == nil {
		return toolError("not authenticated"), nil
	}

	var args struct {
		ID       string `json:"id"`
		TenantID string `json:"tenant_id"`
	}
	raw, _ := json.Marshal(req.Params.Arguments)
	json.Unmarshal(raw, &args)

	db, tenantID, err := s.resolveTenant(user.ID, args.TenantID)
	if err != nil {
		return toolError(err.Error()), nil
	}

	post, err := db.GetPost(ctx, args.ID)
	if err != nil {
		return toolError("post not found"), nil
	}

	now := time.Now()
	post.Status = "published"
	post.PublishedAt = &now

	if err := db.UpdatePost(ctx, post); err != nil {
		return toolError(fmt.Sprintf("failed to publish: %v", err)), nil
	}

	s.Bus.Emit(events.Event{Type: events.PostPublished, TenantID: tenantID})
	liveURL := s.buildPostURL(tenantID, post.Slug)
	return toolResult(fmt.Sprintf("Post published!\n\n**Live URL:** %s", liveURL)), nil
}

func (s *Server) unpublishPost(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	user := auth.UserFromContext(ctx)
	if user == nil {
		return toolError("not authenticated"), nil
	}

	var args struct {
		ID       string `json:"id"`
		TenantID string `json:"tenant_id"`
	}
	raw, _ := json.Marshal(req.Params.Arguments)
	json.Unmarshal(raw, &args)

	db, tenantID, err := s.resolveTenant(user.ID, args.TenantID)
	if err != nil {
		return toolError(err.Error()), nil
	}

	post, err := db.GetPost(ctx, args.ID)
	if err != nil {
		return toolError("post not found"), nil
	}

	post.Status = "draft"
	post.PublishedAt = nil

	if err := db.UpdatePost(ctx, post); err != nil {
		return toolError(fmt.Sprintf("failed to unpublish: %v", err)), nil
	}

	s.Bus.Emit(events.Event{Type: events.PostUpdated, TenantID: tenantID})
	return toolResult("Post reverted to draft."), nil
}

func (s *Server) listPosts(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	user := auth.UserFromContext(ctx)
	if user == nil {
		return toolError("not authenticated"), nil
	}

	var args struct {
		Status   string `json:"status"`
		Tag      string `json:"tag"`
		Limit    int    `json:"limit"`
		TenantID string `json:"tenant_id"`
	}
	raw, _ := json.Marshal(req.Params.Arguments)
	json.Unmarshal(raw, &args)

	db, _, err := s.resolveTenant(user.ID, args.TenantID)
	if err != nil {
		return toolError(err.Error()), nil
	}

	posts, err := db.ListPosts(ctx, tenant.PostFilter{
		Status: args.Status, Tag: args.Tag, Limit: args.Limit,
	})
	if err != nil {
		return toolError(fmt.Sprintf("failed to list posts: %v", err)), nil
	}

	if len(posts) == 0 {
		return toolResult("No posts found."), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d post(s):\n\n", len(posts)))
	for _, p := range posts {
		sb.WriteString(fmt.Sprintf("- **%s** (ID: %s)\n  Slug: %s | Status: %s | Tags: %s\n",
			p.Title, p.ID, p.Slug, p.Status, p.Tags))
		if p.PublishedAt != nil {
			sb.WriteString(fmt.Sprintf("  Published: %s\n", p.PublishedAt.Format("2006-01-02")))
		}
	}
	return toolResult(sb.String()), nil
}

func (s *Server) getPost(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	user := auth.UserFromContext(ctx)
	if user == nil {
		return toolError("not authenticated"), nil
	}

	var args struct {
		ID       string `json:"id"`
		Slug     string `json:"slug"`
		TenantID string `json:"tenant_id"`
	}
	raw, _ := json.Marshal(req.Params.Arguments)
	json.Unmarshal(raw, &args)

	db, _, err := s.resolveTenant(user.ID, args.TenantID)
	if err != nil {
		return toolError(err.Error()), nil
	}

	var post *tenant.Post
	if args.ID != "" {
		post, err = db.GetPost(ctx, args.ID)
	} else if args.Slug != "" {
		post, err = db.GetPostBySlug(ctx, args.Slug)
	} else {
		return toolError("provide either id or slug"), nil
	}
	if err != nil {
		return toolError("post not found"), nil
	}

	result := fmt.Sprintf("**%s**\nID: %s\nSlug: %s\nStatus: %s\nTags: %s\nCreated: %s\n\n---\n\n%s",
		post.Title, post.ID, post.Slug, post.Status, post.Tags,
		post.CreatedAt.Format("2006-01-02 15:04"), post.Content)

	return toolResult(result), nil
}

func (s *Server) searchPosts(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	user := auth.UserFromContext(ctx)
	if user == nil {
		return toolError("not authenticated"), nil
	}

	var args struct {
		Query    string `json:"query"`
		TenantID string `json:"tenant_id"`
	}
	raw, _ := json.Marshal(req.Params.Arguments)
	json.Unmarshal(raw, &args)

	db, _, err := s.resolveTenant(user.ID, args.TenantID)
	if err != nil {
		return toolError(err.Error()), nil
	}

	posts, err := db.SearchPosts(ctx, args.Query)
	if err != nil {
		return toolError(fmt.Sprintf("search failed: %v", err)), nil
	}

	if len(posts) == 0 {
		return toolResult("No posts matching your search."), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d result(s):\n\n", len(posts)))
	for _, p := range posts {
		sb.WriteString(fmt.Sprintf("- **%s** (ID: %s, Status: %s)\n", p.Title, p.ID, p.Status))
	}
	return toolResult(sb.String()), nil
}

var nonAlphaNum = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(title string) string {
	s := strings.ToLower(title)
	s = nonAlphaNum.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 80 {
		s = s[:80]
		s = strings.TrimRight(s, "-")
	}
	return s
}

func generateExcerpt(content string) string {
	plain := stripMarkdown(content)
	if len(plain) > 200 {
		i := 200
		for i > 0 && !unicode.IsSpace(rune(plain[i])) {
			i--
		}
		if i > 0 {
			plain = plain[:i]
		}
		plain += "..."
	}
	return plain
}

func renderContent(content string) string {
	html, err := markdown.Render(content)
	if err != nil {
		return content
	}
	return html
}

func stripMarkdown(s string) string {
	s = regexp.MustCompile(`#+\s*`).ReplaceAllString(s, "")
	s = regexp.MustCompile(`\*+`).ReplaceAllString(s, "")
	s = regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`).ReplaceAllString(s, "$1")
	s = regexp.MustCompile("```[\\s\\S]*?```").ReplaceAllString(s, "")
	s = regexp.MustCompile("`[^`]+`").ReplaceAllString(s, "")
	s = strings.TrimSpace(s)
	return s
}
