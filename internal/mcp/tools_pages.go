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

func (s *Server) registerPageTools(mcpServer *mcp.Server) {
	mcpServer.AddTool(&mcp.Tool{
		Name: "create_page",
		Description: `Create a new page. The page starts as a draft — use publish_page to make it live.

**Content format:** Write the body in rich Markdown. Supported syntax:
- Headings (# through ######), **bold**, *italic*, ~~strikethrough~~
- Links: [text](url), images: ![alt](url)
- Code blocks with language tags (` + "```go, ```python" + `) — renders with syntax highlighting, language icon, and copy button
- Callout blocks: > [!NOTE], > [!TIP], > [!WARNING], > [!DANGER] for styled alert boxes
- Media embeds: <embed src="url" /> for YouTube, Spotify, SoundCloud, Twitter/X, GitHub Gists
- D2 diagrams: ` + "```d2" + ` code blocks for architecture/flow diagrams
- Tables (GFM), ordered/unordered lists, task lists, horizontal rules, footnotes ([^1])
- Raw HTML for advanced layouts

Returns: The created page with a preview URL you can share.`,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"title":         map[string]any{"type": "string", "description": "Page title"},
				"content":       map[string]any{"type": "string", "description": "Page body in rich Markdown."},
				"excerpt":       map[string]any{"type": "string", "description": "Short excerpt for listings (auto-generated if omitted)"},
				"tags":          map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Tags for categorization"},
				"slug":          map[string]any{"type": "string", "description": "URL slug (auto-generated from title if omitted)"},
				"collection_id": map[string]any{"type": "string", "description": "Collection ID to add this page to (optional)"},
				"position":      map[string]any{"type": "integer", "description": "Position within collection (for manual ordering)"},
				"visibility":    map[string]any{"type": "string", "enum": []string{"public", "unlisted", "private"}, "description": "Page visibility: public (default, shown everywhere), unlisted (accessible via URL only), private (team members only)"},
				"tenant_id":     map[string]any{"type": "string", "description": "Site ID (only needed if you have multiple sites)"},
			},
			"required": []string{"title", "content"},
		},
	}, s.createPage)

	mcpServer.AddTool(&mcp.Tool{
		Name: "update_page",
		Description: `Update an existing page. Partial updates — only send the fields you want to change. To move a page to a collection or change title/tags/slug, you don't need to include content.

After updating, a new preview URL is generated so you can verify changes.`,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id":            map[string]any{"type": "string", "description": "Page ID"},
				"title":         map[string]any{"type": "string", "description": "New title"},
				"content":       map[string]any{"type": "string", "description": "New content in rich Markdown."},
				"excerpt":       map[string]any{"type": "string", "description": "New excerpt"},
				"tags":          map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "New tags"},
				"slug":          map[string]any{"type": "string", "description": "New URL slug"},
				"collection_id": map[string]any{"type": "string", "description": "Move to a collection (use empty string to make standalone)"},
				"position":      map[string]any{"type": "integer", "description": "New position within collection"},
				"visibility":    map[string]any{"type": "string", "enum": []string{"public", "unlisted", "private"}, "description": "Page visibility: public, unlisted, or private"},
				"tenant_id":     map[string]any{"type": "string", "description": "Site ID (only needed if you have multiple sites)"},
			},
			"required": []string{"id"},
		},
	}, s.updatePage)

	mcpServer.AddTool(&mcp.Tool{
		Name:        "delete_page",
		Description: "Permanently delete a page.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id":        map[string]any{"type": "string", "description": "Page ID to delete"},
				"tenant_id": map[string]any{"type": "string", "description": "Site ID (only needed if you have multiple sites)"},
			},
			"required": []string{"id"},
		},
	}, s.deletePage)

	mcpServer.AddTool(&mcp.Tool{
		Name:        "publish_page",
		Description: "Publish a draft page, making it live on your site. Returns the live URL.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id":        map[string]any{"type": "string", "description": "Page ID to publish"},
				"tenant_id": map[string]any{"type": "string", "description": "Site ID (only needed if you have multiple sites)"},
			},
			"required": []string{"id"},
		},
	}, s.publishPage)

	mcpServer.AddTool(&mcp.Tool{
		Name:        "unpublish_page",
		Description: "Revert a published page to draft status.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id":        map[string]any{"type": "string", "description": "Page ID to unpublish"},
				"tenant_id": map[string]any{"type": "string", "description": "Site ID (only needed if you have multiple sites)"},
			},
			"required": []string{"id"},
		},
	}, s.unpublishPage)

	mcpServer.AddTool(&mcp.Tool{
		Name:        "append_to_page",
		Description: "Append content to the end of an existing page. Use this instead of update_page when adding new sections — avoids re-sending the entire page content.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id":        map[string]any{"type": "string", "description": "Page ID to append to"},
				"content":   map[string]any{"type": "string", "description": "Markdown content to append at the end of the page"},
				"tenant_id": map[string]any{"type": "string", "description": "Site ID (only needed if you have multiple sites)"},
			},
			"required": []string{"id", "content"},
		},
	}, s.appendToPage)

	mcpServer.AddTool(&mcp.Tool{
		Name:        "list_pages",
		Description: "List pages. Filter by status, tag, or collection.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"status":        map[string]any{"type": "string", "enum": []string{"draft", "published"}, "description": "Filter by status"},
				"visibility":    map[string]any{"type": "string", "enum": []string{"public", "unlisted", "private"}, "description": "Filter by visibility"},
				"tag":           map[string]any{"type": "string", "description": "Filter by tag"},
				"collection_id": map[string]any{"type": "string", "description": "Filter by collection ID. Use 'standalone' for pages not in any collection."},
				"limit":         map[string]any{"type": "integer", "description": "Max results (default 50)"},
				"tenant_id":     map[string]any{"type": "string", "description": "Site ID (only needed if you have multiple sites)"},
			},
		},
	}, s.listPages)

	mcpServer.AddTool(&mcp.Tool{
		Name:        "get_page",
		Description: "Get a single page with full content.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id":        map[string]any{"type": "string", "description": "Page ID"},
				"slug":      map[string]any{"type": "string", "description": "Page slug (alternative to ID)"},
				"tenant_id": map[string]any{"type": "string", "description": "Site ID (only needed if you have multiple sites)"},
			},
		},
	}, s.getPage)

	mcpServer.AddTool(&mcp.Tool{
		Name:        "search_pages",
		Description: "Full-text search across page titles and content.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query":     map[string]any{"type": "string", "description": "Search query"},
				"tenant_id": map[string]any{"type": "string", "description": "Site ID (only needed if you have multiple sites)"},
			},
			"required": []string{"query"},
		},
	}, s.searchPages)
}

func (s *Server) createPage(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	user := auth.UserFromContext(ctx)
	if user == nil {
		return toolError("not authenticated — please sign in at the WriteKit website first"), nil
	}

	var args struct {
		Title        string   `json:"title"`
		Content      string   `json:"content"`
		Excerpt      string   `json:"excerpt"`
		Tags         []string `json:"tags"`
		Slug         string   `json:"slug"`
		CollectionID string   `json:"collection_id"`
		Position     int      `json:"position"`
		Visibility   string   `json:"visibility"`
		TenantID     string   `json:"tenant_id"`
	}
	raw, _ := json.Marshal(req.Params.Arguments)
	json.Unmarshal(raw, &args)

	db, tenantID, err := s.resolveTenantWithRole(ctx, user.ID, args.TenantID, "editor")
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

	var collectionID *string
	if args.CollectionID != "" {
		collectionID = &args.CollectionID
	}

	contentHTML, renderWarnings := renderContentWithErrors(args.Content)

	visibility := args.Visibility
	if visibility == "" {
		visibility = "public"
	}

	page := &tenant.Page{
		ID:           ulid.Make().String(),
		Title:        args.Title,
		Slug:         slug,
		Content:      args.Content,
		ContentHTML:  contentHTML,
		Excerpt:      excerpt,
		Status:       "draft",
		Visibility:   visibility,
		Tags:         string(tagsJSON),
		CollectionID: collectionID,
		Position:     args.Position,
	}

	page.Version = 1

	if err := db.CreatePage(ctx, page); err != nil {
		return toolError(fmt.Sprintf("failed to create page: %v", err)), nil
	}

	db.SavePageVersion(ctx, page)

	pt, err := db.CreatePreviewToken(ctx, page.ID, 24*time.Hour)
	if err != nil {
		return toolError("page created but failed to generate preview URL"), nil
	}

	s.Bus.Emit(events.Event{Type: events.PageCreated, TenantID: tenantID, PageID: page.ID})

	result := fmt.Sprintf("Page created as draft.\n\n**Title:** %s\n**ID:** %s\n**Slug:** %s\n**Preview:** %s\n\nUse publish_page to make it live.",
		page.Title, page.ID, page.Slug, s.buildPreviewURL(tenantID, pt.Token))

	if len(renderWarnings) > 0 {
		result += "\n\n**Warnings:**\n"
		for _, w := range renderWarnings {
			result += "- " + w + "\n"
		}
	}

	return toolResult(result), nil
}

func (s *Server) updatePage(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	user := auth.UserFromContext(ctx)
	if user == nil {
		return toolError("not authenticated — please sign in at the WriteKit website first"), nil
	}

	var args struct {
		ID           string   `json:"id"`
		Title        *string  `json:"title"`
		Content      *string  `json:"content"`
		Excerpt      *string  `json:"excerpt"`
		Tags         []string `json:"tags"`
		Slug         *string  `json:"slug"`
		CollectionID *string  `json:"collection_id"`
		Position     *int     `json:"position"`
		Visibility   *string  `json:"visibility"`
		TenantID     string   `json:"tenant_id"`
	}
	raw, _ := json.Marshal(req.Params.Arguments)
	json.Unmarshal(raw, &args)

	db, tenantID, err := s.resolveTenantWithRole(ctx, user.ID, args.TenantID, "editor")
	if err != nil {
		return toolError(err.Error()), nil
	}

	page, err := db.GetPage(ctx, args.ID)
	if err != nil {
		return toolError("page not found"), nil
	}

	if args.Title != nil {
		page.Title = *args.Title
	}
	var renderWarnings []string
	if args.Content != nil {
		page.Content = *args.Content
		page.ContentHTML, renderWarnings = renderContentWithErrors(*args.Content)
	}
	if args.Excerpt != nil {
		page.Excerpt = *args.Excerpt
	}
	if args.Tags != nil {
		tagsJSON, _ := json.Marshal(args.Tags)
		page.Tags = string(tagsJSON)
	}
	if args.Slug != nil {
		page.Slug = *args.Slug
	}
	if args.CollectionID != nil {
		if *args.CollectionID == "" {
			page.CollectionID = nil
		} else {
			page.CollectionID = args.CollectionID
		}
	}
	if args.Position != nil {
		page.Position = *args.Position
	}
	if args.Visibility != nil {
		page.Visibility = *args.Visibility
	}

	page.Version++

	if err := db.UpdatePage(ctx, page); err != nil {
		return toolError(fmt.Sprintf("failed to update: %v", err)), nil
	}

	db.SavePageVersion(ctx, page)

	s.Bus.Emit(events.Event{Type: events.PageUpdated, TenantID: tenantID, PageID: page.ID})

	pt, _ := db.CreatePreviewToken(ctx, page.ID, 24*time.Hour)
	previewURL := ""
	if pt != nil {
		previewURL = fmt.Sprintf("%s?v=%d", s.buildPreviewURL(tenantID, pt.Token), page.Version)
	}

	result := fmt.Sprintf("Page updated (v%d).\n\n**Title:** %s\n**Status:** %s", page.Version, page.Title, page.Status)
	if page.Status == "published" {
		result += fmt.Sprintf("\n**Live URL:** %s", s.buildPageURL(tenantID, page.CollectionID, page.Slug))
	}
	if previewURL != "" {
		result += fmt.Sprintf("\n**Preview URL:** %s", previewURL)
	}

	if len(renderWarnings) > 0 {
		result += "\n\n**Warnings:**\n"
		for _, w := range renderWarnings {
			result += "- " + w + "\n"
		}
	}

	return toolResult(result), nil
}

func (s *Server) deletePage(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	user := auth.UserFromContext(ctx)
	if user == nil {
		return toolError("not authenticated — please sign in at the WriteKit website first"), nil
	}

	var args struct {
		ID       string `json:"id"`
		TenantID string `json:"tenant_id"`
	}
	raw, _ := json.Marshal(req.Params.Arguments)
	json.Unmarshal(raw, &args)

	db, tenantID, err := s.resolveTenantWithRole(ctx, user.ID, args.TenantID, "editor")
	if err != nil {
		return toolError(err.Error()), nil
	}

	if err := db.DeletePage(ctx, args.ID); err != nil {
		return toolError(fmt.Sprintf("failed to delete: %v", err)), nil
	}

	s.Bus.Emit(events.Event{Type: events.PageDeleted, TenantID: tenantID, PageID: args.ID})
	return toolResult("Page deleted."), nil
}

func (s *Server) publishPage(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	user := auth.UserFromContext(ctx)
	if user == nil {
		return toolError("not authenticated — please sign in at the WriteKit website first"), nil
	}

	var args struct {
		ID       string `json:"id"`
		TenantID string `json:"tenant_id"`
	}
	raw, _ := json.Marshal(req.Params.Arguments)
	json.Unmarshal(raw, &args)

	db, tenantID, err := s.resolveTenantWithRole(ctx, user.ID, args.TenantID, "editor")
	if err != nil {
		return toolError(err.Error()), nil
	}

	page, err := db.GetPage(ctx, args.ID)
	if err != nil {
		return toolError("page not found"), nil
	}

	now := time.Now()
	page.Status = "published"
	page.PublishedAt = &now

	if err := db.UpdatePage(ctx, page); err != nil {
		return toolError(fmt.Sprintf("failed to publish: %v", err)), nil
	}

	s.Bus.Emit(events.Event{Type: events.PagePublished, TenantID: tenantID, PageID: page.ID})
	liveURL := s.buildPageURL(tenantID, page.CollectionID, page.Slug)
	return toolResult(fmt.Sprintf("Page published!\n\n**Live URL:** %s", liveURL)), nil
}

func (s *Server) unpublishPage(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	user := auth.UserFromContext(ctx)
	if user == nil {
		return toolError("not authenticated — please sign in at the WriteKit website first"), nil
	}

	var args struct {
		ID       string `json:"id"`
		TenantID string `json:"tenant_id"`
	}
	raw, _ := json.Marshal(req.Params.Arguments)
	json.Unmarshal(raw, &args)

	db, tenantID, err := s.resolveTenantWithRole(ctx, user.ID, args.TenantID, "editor")
	if err != nil {
		return toolError(err.Error()), nil
	}

	page, err := db.GetPage(ctx, args.ID)
	if err != nil {
		return toolError("page not found"), nil
	}

	page.Status = "draft"
	page.PublishedAt = nil

	if err := db.UpdatePage(ctx, page); err != nil {
		return toolError(fmt.Sprintf("failed to unpublish: %v", err)), nil
	}

	s.Bus.Emit(events.Event{Type: events.PageUpdated, TenantID: tenantID, PageID: page.ID})
	return toolResult("Page reverted to draft."), nil
}

func (s *Server) appendToPage(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	user := auth.UserFromContext(ctx)
	if user == nil {
		return toolError("not authenticated — please sign in at the WriteKit website first"), nil
	}

	var args struct {
		ID       string `json:"id"`
		Content  string `json:"content"`
		TenantID string `json:"tenant_id"`
	}
	raw, _ := json.Marshal(req.Params.Arguments)
	json.Unmarshal(raw, &args)

	db, tenantID, err := s.resolveTenantWithRole(ctx, user.ID, args.TenantID, "editor")
	if err != nil {
		return toolError(err.Error()), nil
	}

	page, err := db.GetPage(ctx, args.ID)
	if err != nil {
		return toolError("page not found"), nil
	}

	page.Content = page.Content + "\n\n" + args.Content
	page.ContentHTML, _ = renderContentWithErrors(page.Content)
	page.Version++

	if err := db.UpdatePage(ctx, page); err != nil {
		return toolError(fmt.Sprintf("failed to append: %v", err)), nil
	}

	db.SavePageVersion(ctx, page)

	s.Bus.Emit(events.Event{Type: events.PageUpdated, TenantID: tenantID, PageID: page.ID})

	pt, _ := db.CreatePreviewToken(ctx, page.ID, 24*time.Hour)
	previewURL := ""
	if pt != nil {
		previewURL = s.buildPreviewURL(tenantID, pt.Token)
	}

	result := fmt.Sprintf("Content appended (v%d).\n\n**Title:** %s\n**Status:** %s", page.Version, page.Title, page.Status)
	if previewURL != "" {
		result += fmt.Sprintf("\n**Preview URL:** %s", previewURL)
	}

	return toolResult(result), nil
}

func (s *Server) listPages(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	user := auth.UserFromContext(ctx)
	if user == nil {
		return toolError("not authenticated — please sign in at the WriteKit website first"), nil
	}

	var args struct {
		Status       string `json:"status"`
		Visibility   string `json:"visibility"`
		Tag          string `json:"tag"`
		CollectionID string `json:"collection_id"`
		Limit        int    `json:"limit"`
		TenantID     string `json:"tenant_id"`
	}
	raw, _ := json.Marshal(req.Params.Arguments)
	json.Unmarshal(raw, &args)

	db, _, err := s.resolveTenant(user.ID, args.TenantID)
	if err != nil {
		return toolError(err.Error()), nil
	}

	filter := tenant.PageFilter{Status: args.Status, Visibility: args.Visibility, Tag: args.Tag, Limit: args.Limit}
	if args.CollectionID == "standalone" {
		empty := ""
		filter.CollectionID = &empty
	} else if args.CollectionID != "" {
		filter.CollectionID = &args.CollectionID
	}

	pages, err := db.ListPages(ctx, filter)
	if err != nil {
		return toolError(fmt.Sprintf("failed to list pages: %v", err)), nil
	}

	if len(pages) == 0 {
		return toolResult("No pages found."), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d page(s):\n\n", len(pages)))
	for _, p := range pages {
		sb.WriteString(fmt.Sprintf("- **%s** (ID: %s)\n  Slug: %s | Status: %s | Visibility: %s | Tags: %s\n",
			p.Title, p.ID, p.Slug, p.Status, p.Visibility, p.Tags))
		if p.CollectionID != nil {
			sb.WriteString(fmt.Sprintf("  Collection: %s\n", *p.CollectionID))
		}
		if p.PublishedAt != nil {
			sb.WriteString(fmt.Sprintf("  Published: %s\n", p.PublishedAt.Format("2006-01-02")))
		}
	}
	return toolResult(sb.String()), nil
}

func (s *Server) getPage(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	user := auth.UserFromContext(ctx)
	if user == nil {
		return toolError("not authenticated — please sign in at the WriteKit website first"), nil
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

	var page *tenant.Page
	if args.ID != "" {
		page, err = db.GetPage(ctx, args.ID)
	} else if args.Slug != "" {
		page, err = db.GetPageBySlug(ctx, args.Slug)
	} else {
		return toolError("provide either id or slug"), nil
	}
	if err != nil {
		return toolError("page not found"), nil
	}

	result := fmt.Sprintf("**%s**\nID: %s\nSlug: %s\nStatus: %s\nVisibility: %s\nTags: %s\nCreated: %s\n\n---\n\n%s",
		page.Title, page.ID, page.Slug, page.Status, page.Visibility, page.Tags,
		page.CreatedAt.Format("2006-01-02 15:04"), page.Content)

	return toolResult(result), nil
}

func (s *Server) searchPages(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	user := auth.UserFromContext(ctx)
	if user == nil {
		return toolError("not authenticated — please sign in at the WriteKit website first"), nil
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

	pages, err := db.SearchPages(ctx, args.Query)
	if err != nil {
		return toolError(fmt.Sprintf("search failed: %v", err)), nil
	}

	if len(pages) == 0 {
		return toolResult("No pages matching your search."), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d result(s):\n\n", len(pages)))
	for _, p := range pages {
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

func renderContentWithErrors(content string) (string, []string) {
	return markdown.RenderWithErrors(content)
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
