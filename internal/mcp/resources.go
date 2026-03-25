package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"writekit/internal/auth"
	"writekit/internal/tenant"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func (s *Server) registerResources(mcpServer *mcpsdk.Server) {
	mcpServer.AddResource(&mcpsdk.Resource{
		URI:         "writekit://site/stats",
		Name:        "Site Stats",
		Description: "Page counts, collection counts, comment counts",
		MIMEType:    "application/json",
	}, s.statsResource)

	mcpServer.AddResource(&mcpsdk.Resource{
		URI:         "writekit://site/settings",
		Name:        "Site Settings",
		Description: "Current site settings",
		MIMEType:    "application/json",
	}, s.settingsResource)

	mcpServer.AddResource(&mcpsdk.Resource{
		URI:         "writekit://site/recent-pages",
		Name:        "Recent Pages",
		Description: "Last 10 published pages",
		MIMEType:    "text/plain",
	}, s.recentPagesResource)

	mcpServer.AddResource(&mcpsdk.Resource{
		URI:         "writekit://site/drafts",
		Name:        "Drafts",
		Description: "All draft pages",
		MIMEType:    "text/plain",
	}, s.draftsResource)

	mcpServer.AddResource(&mcpsdk.Resource{
		URI:         "writekit://site/collections",
		Name:        "Collections",
		Description: "All collections with page counts",
		MIMEType:    "text/plain",
	}, s.collectionsResource)

	mcpServer.AddResource(&mcpsdk.Resource{
		URI:         "writekit://site/recent-comments",
		Name:        "Recent Comments",
		Description: "Recent comments across all pages",
		MIMEType:    "text/plain",
	}, s.recentCommentsResource)
}

func (s *Server) resolveResourceTenant(ctx context.Context) (*tenant.DB, string, error) {
	user := auth.UserFromContext(ctx)
	if user == nil {
		return nil, "", fmt.Errorf("not authenticated — please sign in at the WriteKit website first")
	}
	return s.resolveTenant(user.ID, "")
}

func (s *Server) statsResource(ctx context.Context, req *mcpsdk.ReadResourceRequest) (*mcpsdk.ReadResourceResult, error) {
	db, _, err := s.resolveResourceTenant(ctx)
	if err != nil {
		return nil, err
	}

	published, _ := db.CountPages(ctx, "published")
	drafts, _ := db.CountPages(ctx, "draft")
	comments, _ := db.CountComments(ctx)
	collections, _ := db.ListCollections(ctx)

	data, _ := json.Marshal(map[string]int{
		"published_pages":   published,
		"draft_pages":       drafts,
		"total_comments":    comments,
		"total_collections": len(collections),
	})

	return &mcpsdk.ReadResourceResult{
		Contents: []*mcpsdk.ResourceContents{
			{URI: "writekit://site/stats", MIMEType: "application/json", Text: string(data)},
		},
	}, nil
}

func (s *Server) settingsResource(ctx context.Context, req *mcpsdk.ReadResourceRequest) (*mcpsdk.ReadResourceResult, error) {
	db, _, err := s.resolveResourceTenant(ctx)
	if err != nil {
		return nil, err
	}

	settings, err := db.GetSettings(ctx)
	if err != nil {
		return nil, err
	}

	data, _ := json.Marshal(settings)
	return &mcpsdk.ReadResourceResult{
		Contents: []*mcpsdk.ResourceContents{
			{URI: "writekit://site/settings", MIMEType: "application/json", Text: string(data)},
		},
	}, nil
}

func (s *Server) recentPagesResource(ctx context.Context, req *mcpsdk.ReadResourceRequest) (*mcpsdk.ReadResourceResult, error) {
	db, _, err := s.resolveResourceTenant(ctx)
	if err != nil {
		return nil, err
	}

	pages, _ := db.ListPages(ctx, tenant.PageFilter{Status: "published", Limit: 10})

	var sb strings.Builder
	for _, p := range pages {
		sb.WriteString(fmt.Sprintf("- %s (slug: %s, published: %v)\n  %s\n",
			p.Title, p.Slug, p.PublishedAt, p.Excerpt))
	}

	return &mcpsdk.ReadResourceResult{
		Contents: []*mcpsdk.ResourceContents{
			{URI: "writekit://site/recent-pages", MIMEType: "text/plain", Text: sb.String()},
		},
	}, nil
}

func (s *Server) draftsResource(ctx context.Context, req *mcpsdk.ReadResourceRequest) (*mcpsdk.ReadResourceResult, error) {
	db, _, err := s.resolveResourceTenant(ctx)
	if err != nil {
		return nil, err
	}

	pages, _ := db.ListPages(ctx, tenant.PageFilter{Status: "draft", Limit: 50})

	var sb strings.Builder
	for _, p := range pages {
		sb.WriteString(fmt.Sprintf("- %s (ID: %s, created: %s)\n  %s\n",
			p.Title, p.ID, p.CreatedAt.Format("2006-01-02"), p.Excerpt))
	}
	if sb.Len() == 0 {
		sb.WriteString("No drafts.")
	}

	return &mcpsdk.ReadResourceResult{
		Contents: []*mcpsdk.ResourceContents{
			{URI: "writekit://site/drafts", MIMEType: "text/plain", Text: sb.String()},
		},
	}, nil
}

func (s *Server) collectionsResource(ctx context.Context, req *mcpsdk.ReadResourceRequest) (*mcpsdk.ReadResourceResult, error) {
	db, _, err := s.resolveResourceTenant(ctx)
	if err != nil {
		return nil, err
	}

	collections, _ := db.ListCollections(ctx)

	var sb strings.Builder
	for _, c := range collections {
		count, _ := db.CountCollectionPages(ctx, c.ID)
		sb.WriteString(fmt.Sprintf("- %s (slug: %s, sort: %s, pages: %d)\n",
			c.Title, c.Slug, c.SortOrder, count))
		if c.Description != "" {
			sb.WriteString(fmt.Sprintf("  %s\n", c.Description))
		}
	}
	if sb.Len() == 0 {
		sb.WriteString("No collections.")
	}

	return &mcpsdk.ReadResourceResult{
		Contents: []*mcpsdk.ResourceContents{
			{URI: "writekit://site/collections", MIMEType: "text/plain", Text: sb.String()},
		},
	}, nil
}

func (s *Server) recentCommentsResource(ctx context.Context, req *mcpsdk.ReadResourceRequest) (*mcpsdk.ReadResourceResult, error) {
	db, _, err := s.resolveResourceTenant(ctx)
	if err != nil {
		return nil, err
	}

	comments, _ := db.ListRecentComments(ctx, 20)

	var sb strings.Builder
	for _, c := range comments {
		sb.WriteString(fmt.Sprintf("- [%s] %s on page %s: %s\n",
			c.CreatedAt.Format("2006-01-02"), c.Author, c.PageID, c.Content))
	}
	if sb.Len() == 0 {
		sb.WriteString("No comments yet.")
	}

	return &mcpsdk.ReadResourceResult{
		Contents: []*mcpsdk.ResourceContents{
			{URI: "writekit://site/recent-comments", MIMEType: "text/plain", Text: sb.String()},
		},
	}, nil
}
