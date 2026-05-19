package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/oklog/ulid/v2"
	"writekit/internal/auth"
	"writekit/internal/events"
	"writekit/internal/markdown"
	"writekit/internal/tenant"
)

func (s *Server) registerPageTools(mcpServer *mcp.Server) {
	mcpServer.AddTool(&mcp.Tool{
		Name: "create_page",
		Description: `Create a new page as an empty draft. Use ` + "`append_to_page`" + ` to add body content section by section, or ` + "`update_page`" + ` to set/replace the full content.

**Content format (for the content you'll write via append_to_page / update_page):** Rich Markdown — headings, **bold**, *italic*, ~~strikethrough~~, links, images, GFM tables, lists, task lists, footnotes ([^1]), and raw HTML.

**Beautiful by default — use these freely:**
- Code blocks with language tags (` + "```go, ```python" + `) render with syntax highlighting, a language icon, and a copy button.
- Callout blocks: ` + "`> [!NOTE]`" + `, ` + "`> [!TIP]`" + `, ` + "`> [!WARNING]`" + `, ` + "`> [!DANGER]`" + ` produce styled alert boxes.
- Media embeds: ` + "`<embed src=\"url\" />`" + ` works for YouTube, Spotify, SoundCloud, Twitter/X, GitHub Gists.
- Images: ` + "`![alt](url)`" + ` — for new images, call ` + "`upload_image`" + ` first to get a ` + "`/img/{id}.webp`" + ` URL; never inline base64.
- D2 diagrams in ` + "```d2" + ` code blocks (see reference below) — rendered server-side to SVG with NeutralGrey theme, dagre auto-layout, and zoom/pan/fullscreen.

**D2 diagram reference**

Key syntax:
- Containers (nesting): server: { api: API; db: Database }
- Shapes: shape: cylinder (DB), queue, cloud, package, diamond, person, hexagon
- Data shapes: shape: sql_table, class (with typed fields/methods)
- Connections: a -> b: label, a <-> b (bidirectional), a -- b (undirected)
- Classes (reusable styles): classes: { svc: { style.fill: "#3498DB"; style.stroke: "#2E5C8A" } } then node.class: svc
- Styles: style.fill, style.stroke, style.stroke-width, style.border-radius, style.shadow, style.3d, style.opacity, style.font-size, style.text-transform, style.stroke-dash
- Icons: icon: https://icons.terrastruct.com/... or any SVG URL
- Tooltips: tooltip: "hover text"
- Direction: direction: right (or down, left, up)
- Grid layout: grid-rows: N, grid-columns: N, grid-gap: N for structured layouts
- Sequence diagrams: shape: sequence_diagram with actor -> actor: message syntax

Best practices for professional diagrams:
- Always group related components into containers — flat diagrams look amateur
- Label every connection with the protocol, data, or relationship it represents
- Use classes for consistent styling across similar elements (services, databases, etc.)
- Use appropriate shapes: cylinder for databases, cloud for external services, package for modules
- Keep it focused — one diagram per concept, don't try to show everything

Returns: the page id, slug, and a preview URL.`,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"title":         map[string]any{"type": "string", "description": "Page title"},
				"tags":          map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Tags for categorization"},
				"slug":          map[string]any{"type": "string", "description": "URL slug (auto-generated from title if omitted)"},
				"collection_id": map[string]any{"type": "string", "description": "Collection ID to add this page to (optional)"},
				"position":      map[string]any{"type": "integer", "description": "Position within collection (for manual ordering)"},
				"visibility":    map[string]any{"type": "string", "enum": []string{"public", "unlisted", "private"}, "description": "Visibility *when published*: public (listed in index/sitemap/search), unlisted (accessible by URL only), private (team members only). This does NOT publish the page — pages remain drafts until publish_page is called explicitly by the user."},
				"tenant_id":     map[string]any{"type": "string", "description": "Site ID (only needed if you have multiple sites)"},
			},
			"required": []string{"title"},
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id":          map[string]any{"type": "string"},
				"slug":        map[string]any{"type": "string"},
				"title":       map[string]any{"type": "string"},
				"preview_url": map[string]any{"type": "string"},
				"version":     map[string]any{"type": "integer"},
			},
			"required": []string{"id", "slug", "title", "preview_url", "version"},
		},
	}, s.createPage)

	mcpServer.AddTool(&mcp.Tool{
		Name:        "update_page",
		Description: "Update an existing page. Partial updates — only send the fields you want to change. Sending `content` REPLACES the page body atomically — the preview updates in a single hop. Ideal for revisions and edits; for fresh long-form drafts where you want the user to see the page build up section by section, use `append_to_page` repeatedly instead.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id":            map[string]any{"type": "string", "description": "Page ID"},
				"title":         map[string]any{"type": "string", "description": "New title"},
				"content":       map[string]any{"type": "string", "description": "New content in rich Markdown (replaces existing body)."},
				"excerpt":       map[string]any{"type": "string", "description": "New excerpt"},
				"tags":          map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "New tags"},
				"slug":          map[string]any{"type": "string", "description": "New URL slug"},
				"collection_id": map[string]any{"type": "string", "description": "Move to a collection (use empty string to make standalone)"},
				"position":      map[string]any{"type": "integer", "description": "New position within collection"},
				"visibility":    map[string]any{"type": "string", "enum": []string{"public", "unlisted", "private"}, "description": "Visibility *when published*: public (listed in index/sitemap/search), unlisted (accessible by URL only), private (team members only). This does NOT publish the page — pages remain drafts until publish_page is called explicitly by the user."},
				"tenant_id":     map[string]any{"type": "string", "description": "Site ID (only needed if you have multiple sites)"},
			},
			"required": []string{"id"},
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id":          map[string]any{"type": "string"},
				"version":     map[string]any{"type": "integer"},
				"status":      map[string]any{"type": "string"},
				"preview_url": map[string]any{"type": "string"},
				"live_url":    map[string]any{"type": "string"},
			},
			"required": []string{"id", "version", "status", "preview_url"},
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
		Description: "Publish a draft page, making it live on the site's public index, sitemap, and (where configured) search. **Call this only when the user has explicitly asked to publish.** Setting `visibility: \"public\"` on `create_page` or `update_page` does *not* mean the page should be published — it only controls whether the page would be listed if/when published. Pages remain in draft state until publish_page is called.",
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
		Description: "Append markdown to the end of an existing page. Strictly additive — existing content is unchanged. Calling this repeatedly streams content into the page in real time, so the user can watch sections appear in the preview tab as you compose them.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id":        map[string]any{"type": "string", "description": "Page ID to append to"},
				"content":   map[string]any{"type": "string", "description": "Markdown content to append at the end of the page"},
				"tenant_id": map[string]any{"type": "string", "description": "Site ID (only needed if you have multiple sites)"},
			},
			"required": []string{"id", "content"},
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id":          map[string]any{"type": "string"},
				"version":     map[string]any{"type": "integer"},
				"preview_url": map[string]any{"type": "string"},
			},
			"required": []string{"id", "version", "preview_url"},
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
	s.trackPresence(req, tenantID)

	slug := args.Slug
	if slug == "" {
		slug = slugify(args.Title)
	}

	tagsJSON, _ := json.Marshal(args.Tags)
	if args.Tags == nil {
		tagsJSON = []byte("[]")
	}

	var collectionID *string
	if args.CollectionID != "" {
		collectionID = &args.CollectionID
	}

	visibility := args.Visibility
	if visibility == "" {
		visibility = "public"
	}

	page := &tenant.Page{
		ID:           ulid.Make().String(),
		Title:        args.Title,
		Slug:         slug,
		Content:      "",
		ContentHTML:  "",
		Excerpt:      "",
		Status:       "draft",
		Visibility:   visibility,
		Tags:         string(tagsJSON),
		CollectionID: collectionID,
		Position:     args.Position,
		Version:      0,
	}

	if err := db.CreatePage(ctx, page); err != nil {
		return toolError(fmt.Sprintf("failed to create page: %v", err)), nil
	}

	pt, err := s.ensurePreviewToken(ctx, db, page.ID)
	if err != nil {
		return toolError("page created but failed to generate preview URL"), nil
	}

	s.Bus.Emit(events.Event{Type: events.PageCreated, TenantID: tenantID, PageID: page.ID})

	previewURL := s.buildPreviewURL(tenantID, pt.Token)

	result := fmt.Sprintf("Page created as draft.\n\n**Title:** %s\n**ID:** %s\n**Slug:** %s\n**Preview:** %s\n\nAdd content with append_to_page or update_page, then publish_page to make it live.",
		page.Title, page.ID, page.Slug, previewURL)

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: result}},
		StructuredContent: map[string]any{
			"id":          page.ID,
			"slug":        page.Slug,
			"title":       page.Title,
			"preview_url": previewURL,
			"version":     page.Version,
		},
	}, nil
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
	s.trackPresence(req, tenantID)

	page, err := db.GetPage(ctx, args.ID)
	if err != nil {
		return toolError("page not found"), nil
	}

	if args.Title != nil {
		page.Title = *args.Title
	}
	contentChanged := false
	if args.Content != nil && *args.Content != page.Content {
		contentChanged = true
		page.Content = *args.Content
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

	// If content changed, save with stale HTML now — render asynchronously
	if err := db.UpdatePage(ctx, page); err != nil {
		return toolError(fmt.Sprintf("failed to update: %v", err)), nil
	}

	// Emit immediate "content saved" event so preview shows loading state
	if contentChanged {
		s.Bus.Emit(events.Event{Type: events.PageContentSaved, TenantID: tenantID, PageID: page.ID})
	}

	// Background: render HTML, update DB, save version, emit "updated"
	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		if contentChanged {
			html, warnings := renderContentWithErrors(page.Content)
			page.ContentHTML = html
			if err := db.UpdatePageContentHTML(bgCtx, page.ID, html); err != nil {
				slog.Error("mcp: background render failed", "page_id", page.ID, "err", err)
			}
			if len(warnings) > 0 {
				slog.Warn("mcp: render warnings", "page_id", page.ID, "warnings", warnings)
			}
		}

		db.SavePageVersion(bgCtx, page)
		s.Bus.Emit(events.Event{Type: events.PageUpdated, TenantID: tenantID, PageID: page.ID})
	}()

	pt, _ := s.ensurePreviewToken(ctx, db, page.ID)
	previewURL := ""
	if pt != nil {
		previewURL = fmt.Sprintf("%s?v=%d", s.buildPreviewURL(tenantID, pt.Token), page.Version)
	}

	liveURL := ""
	result := fmt.Sprintf("Page updated (v%d).\n\n**Title:** %s\n**Status:** %s", page.Version, page.Title, page.Status)
	if page.Status == "published" {
		liveURL = s.buildPageURL(tenantID, page.CollectionID, page.Slug)
		result += fmt.Sprintf("\n**Live URL:** %s", liveURL)
	}
	if previewURL != "" {
		result += fmt.Sprintf("\n**Preview URL:** %s", previewURL)
	}

	structured := map[string]any{
		"id":          page.ID,
		"version":     page.Version,
		"status":      page.Status,
		"preview_url": previewURL,
	}
	if liveURL != "" {
		structured["live_url"] = liveURL
	}

	return &mcp.CallToolResult{
		Content:           []mcp.Content{&mcp.TextContent{Text: result}},
		StructuredContent: structured,
	}, nil
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
	s.trackPresence(req, tenantID)

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
	s.trackPresence(req, tenantID)

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
	s.trackPresence(req, tenantID)

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
	s.trackPresence(req, tenantID)

	page, err := db.GetPage(ctx, args.ID)
	if err != nil {
		return toolError("page not found"), nil
	}

	if page.Version == 0 || page.Content == "" {
		page.Content = args.Content
	} else {
		page.Content = page.Content + "\n\n" + args.Content
	}
	page.Version++

	if err := db.UpdatePage(ctx, page); err != nil {
		return toolError(fmt.Sprintf("failed to append: %v", err)), nil
	}

	s.Bus.Emit(events.Event{Type: events.PageContentSaved, TenantID: tenantID, PageID: page.ID})

	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		html, warnings := renderContentWithErrors(page.Content)
		page.ContentHTML = html
		if err := db.UpdatePageContentHTML(bgCtx, page.ID, html); err != nil {
			slog.Error("mcp: background render failed", "page_id", page.ID, "err", err)
		}
		if len(warnings) > 0 {
			slog.Warn("mcp: render warnings", "page_id", page.ID, "warnings", warnings)
		}
		db.SavePageVersion(bgCtx, page)
		s.Bus.Emit(events.Event{Type: events.PageUpdated, TenantID: tenantID, PageID: page.ID})
	}()

	pt, _ := s.ensurePreviewToken(ctx, db, page.ID)
	previewURL := ""
	if pt != nil {
		previewURL = fmt.Sprintf("%s?v=%d", s.buildPreviewURL(tenantID, pt.Token), page.Version)
	}

	result := fmt.Sprintf("Content appended (v%d).\n\n**Title:** %s\n**Status:** %s", page.Version, page.Title, page.Status)
	if previewURL != "" {
		result += fmt.Sprintf("\n**Preview URL:** %s", previewURL)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: result}},
		StructuredContent: map[string]any{
			"id":          page.ID,
			"version":     page.Version,
			"preview_url": previewURL,
		},
	}, nil
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
