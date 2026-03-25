package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"writekit/internal/auth"
	"writekit/internal/events"
	"writekit/internal/tenant"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/oklog/ulid/v2"
)

func (s *Server) registerCollectionTools(mcpServer *mcp.Server) {
	mcpServer.AddTool(&mcp.Tool{
		Name:        "create_collection",
		Description: "Create a new collection to group pages together. Collections can be ordered manually or by date.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"title":       map[string]any{"type": "string", "description": "Collection title"},
				"description": map[string]any{"type": "string", "description": "Short description of the collection"},
				"slug":        map[string]any{"type": "string", "description": "URL slug (auto-generated from title if omitted)"},
				"sort_order":  map[string]any{"type": "string", "enum": []string{"manual", "date"}, "description": "How pages are ordered: 'manual' (by position) or 'date' (by publish date). Default: manual"},
				"tenant_id":   map[string]any{"type": "string", "description": "Site ID (only needed if you have multiple sites)"},
			},
			"required": []string{"title"},
		},
	}, s.createCollection)

	mcpServer.AddTool(&mcp.Tool{
		Name:        "update_collection",
		Description: "Update a collection's title, description, slug, or sort order.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id":          map[string]any{"type": "string", "description": "Collection ID"},
				"title":       map[string]any{"type": "string", "description": "New title"},
				"description": map[string]any{"type": "string", "description": "New description"},
				"slug":        map[string]any{"type": "string", "description": "New URL slug"},
				"sort_order":  map[string]any{"type": "string", "enum": []string{"manual", "date"}, "description": "New sort order"},
				"tenant_id":   map[string]any{"type": "string", "description": "Site ID (only needed if you have multiple sites)"},
			},
			"required": []string{"id"},
		},
	}, s.updateCollection)

	mcpServer.AddTool(&mcp.Tool{
		Name:        "delete_collection",
		Description: "Delete a collection. Pages in the collection become standalone.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id":        map[string]any{"type": "string", "description": "Collection ID to delete"},
				"tenant_id": map[string]any{"type": "string", "description": "Site ID (only needed if you have multiple sites)"},
			},
			"required": []string{"id"},
		},
	}, s.deleteCollection)

	mcpServer.AddTool(&mcp.Tool{
		Name:        "list_collections",
		Description: "List all collections with page counts.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"tenant_id": map[string]any{"type": "string", "description": "Site ID (only needed if you have multiple sites)"},
			},
		},
	}, s.listCollections)

	mcpServer.AddTool(&mcp.Tool{
		Name:        "get_collection",
		Description: "Get a collection with its pages.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id":        map[string]any{"type": "string", "description": "Collection ID"},
				"slug":      map[string]any{"type": "string", "description": "Collection slug (alternative to ID)"},
				"tenant_id": map[string]any{"type": "string", "description": "Site ID (only needed if you have multiple sites)"},
			},
		},
	}, s.getCollection)

	mcpServer.AddTool(&mcp.Tool{
		Name:        "reorder_pages",
		Description: "Set the order of pages within a manually-ordered collection.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"collection_id": map[string]any{"type": "string", "description": "Collection ID"},
				"page_ids":      map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Page IDs in desired order"},
				"tenant_id":     map[string]any{"type": "string", "description": "Site ID (only needed if you have multiple sites)"},
			},
			"required": []string{"collection_id", "page_ids"},
		},
	}, s.reorderPages)
}

func (s *Server) createCollection(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	user := auth.UserFromContext(ctx)
	if user == nil {
		return toolError("not authenticated"), nil
	}

	var args struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Slug        string `json:"slug"`
		SortOrder   string `json:"sort_order"`
		TenantID    string `json:"tenant_id"`
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

	available, err := db.IsSlugAvailable(ctx, slug)
	if err != nil {
		return toolError(fmt.Sprintf("failed to check slug: %v", err)), nil
	}
	if !available {
		return toolError(fmt.Sprintf("slug '%s' is already in use", slug)), nil
	}

	sortOrder := args.SortOrder
	if sortOrder == "" {
		sortOrder = "manual"
	}

	collection := &tenant.Collection{
		ID:          ulid.Make().String(),
		Title:       args.Title,
		Slug:        slug,
		Description: args.Description,
		SortOrder:   sortOrder,
	}

	if err := db.CreateCollection(ctx, collection); err != nil {
		return toolError(fmt.Sprintf("failed to create collection: %v", err)), nil
	}

	s.Bus.Emit(events.Event{Type: events.CollectionCreated, TenantID: tenantID})

	return toolResult(fmt.Sprintf("Collection created.\n\n**Title:** %s\n**ID:** %s\n**Slug:** %s\n**Sort order:** %s",
		collection.Title, collection.ID, collection.Slug, collection.SortOrder)), nil
}

func (s *Server) updateCollection(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	user := auth.UserFromContext(ctx)
	if user == nil {
		return toolError("not authenticated"), nil
	}

	var args struct {
		ID          string  `json:"id"`
		Title       *string `json:"title"`
		Description *string `json:"description"`
		Slug        *string `json:"slug"`
		SortOrder   *string `json:"sort_order"`
		TenantID    string  `json:"tenant_id"`
	}
	raw, _ := json.Marshal(req.Params.Arguments)
	json.Unmarshal(raw, &args)

	db, tenantID, err := s.resolveTenant(user.ID, args.TenantID)
	if err != nil {
		return toolError(err.Error()), nil
	}

	collection, err := db.GetCollection(ctx, args.ID)
	if err != nil {
		return toolError("collection not found"), nil
	}

	if args.Title != nil {
		collection.Title = *args.Title
	}
	if args.Description != nil {
		collection.Description = *args.Description
	}
	if args.Slug != nil {
		collection.Slug = *args.Slug
	}
	if args.SortOrder != nil {
		collection.SortOrder = *args.SortOrder
	}

	if err := db.UpdateCollection(ctx, collection); err != nil {
		return toolError(fmt.Sprintf("failed to update: %v", err)), nil
	}

	s.Bus.Emit(events.Event{Type: events.CollectionUpdated, TenantID: tenantID})
	return toolResult(fmt.Sprintf("Collection updated.\n\n**Title:** %s\n**Slug:** %s", collection.Title, collection.Slug)), nil
}

func (s *Server) deleteCollection(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

	if err := db.DeleteCollection(ctx, args.ID); err != nil {
		return toolError(fmt.Sprintf("failed to delete: %v", err)), nil
	}

	s.Bus.Emit(events.Event{Type: events.CollectionDeleted, TenantID: tenantID})
	return toolResult("Collection deleted. Pages in this collection are now standalone."), nil
}

func (s *Server) listCollections(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	user := auth.UserFromContext(ctx)
	if user == nil {
		return toolError("not authenticated"), nil
	}

	var args struct {
		TenantID string `json:"tenant_id"`
	}
	raw, _ := json.Marshal(req.Params.Arguments)
	json.Unmarshal(raw, &args)

	db, _, err := s.resolveTenant(user.ID, args.TenantID)
	if err != nil {
		return toolError(err.Error()), nil
	}

	collections, err := db.ListCollections(ctx)
	if err != nil {
		return toolError(fmt.Sprintf("failed to list collections: %v", err)), nil
	}

	if len(collections) == 0 {
		return toolResult("No collections found."), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d collection(s):\n\n", len(collections)))
	for _, c := range collections {
		count, _ := db.CountCollectionPages(ctx, c.ID)
		sb.WriteString(fmt.Sprintf("- **%s** (ID: %s)\n  Slug: %s | Sort: %s | Pages: %d\n",
			c.Title, c.ID, c.Slug, c.SortOrder, count))
		if c.Description != "" {
			sb.WriteString(fmt.Sprintf("  %s\n", c.Description))
		}
	}
	return toolResult(sb.String()), nil
}

func (s *Server) getCollection(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

	var collection *tenant.Collection
	if args.ID != "" {
		collection, err = db.GetCollection(ctx, args.ID)
	} else if args.Slug != "" {
		collection, err = db.GetCollectionBySlug(ctx, args.Slug)
	} else {
		return toolError("provide either id or slug"), nil
	}
	if err != nil {
		return toolError("collection not found"), nil
	}

	pages, err := db.ListCollectionPages(ctx, collection.ID, collection.SortOrder)
	if err != nil {
		return toolError(fmt.Sprintf("failed to list pages: %v", err)), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("**%s**\nID: %s\nSlug: %s\nSort: %s\n", collection.Title, collection.ID, collection.Slug, collection.SortOrder))
	if collection.Description != "" {
		sb.WriteString(fmt.Sprintf("Description: %s\n", collection.Description))
	}
	sb.WriteString(fmt.Sprintf("\n%d page(s):\n\n", len(pages)))
	for i, p := range pages {
		sb.WriteString(fmt.Sprintf("%d. **%s** (ID: %s, Status: %s)\n", i+1, p.Title, p.ID, p.Status))
	}

	return toolResult(sb.String()), nil
}

func (s *Server) reorderPages(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	user := auth.UserFromContext(ctx)
	if user == nil {
		return toolError("not authenticated"), nil
	}

	var args struct {
		CollectionID string   `json:"collection_id"`
		PageIDs      []string `json:"page_ids"`
		TenantID     string   `json:"tenant_id"`
	}
	raw, _ := json.Marshal(req.Params.Arguments)
	json.Unmarshal(raw, &args)

	db, tenantID, err := s.resolveTenant(user.ID, args.TenantID)
	if err != nil {
		return toolError(err.Error()), nil
	}

	if err := db.ReorderPages(ctx, args.CollectionID, args.PageIDs); err != nil {
		return toolError(fmt.Sprintf("failed to reorder: %v", err)), nil
	}

	s.Bus.Emit(events.Event{Type: events.CollectionUpdated, TenantID: tenantID})
	return toolResult(fmt.Sprintf("Reordered %d pages.", len(args.PageIDs))), nil
}
