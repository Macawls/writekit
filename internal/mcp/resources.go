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
		URI:         "writekit://blog/stats",
		Name:        "Blog Stats",
		Description: "Post counts, comment counts",
		MIMEType:    "application/json",
	}, s.statsResource)

	mcpServer.AddResource(&mcpsdk.Resource{
		URI:         "writekit://blog/settings",
		Name:        "Blog Settings",
		Description: "Current blog settings",
		MIMEType:    "application/json",
	}, s.settingsResource)

	mcpServer.AddResource(&mcpsdk.Resource{
		URI:         "writekit://blog/recent-posts",
		Name:        "Recent Posts",
		Description: "Last 10 published posts",
		MIMEType:    "text/plain",
	}, s.recentPostsResource)

	mcpServer.AddResource(&mcpsdk.Resource{
		URI:         "writekit://blog/drafts",
		Name:        "Drafts",
		Description: "All draft posts",
		MIMEType:    "text/plain",
	}, s.draftsResource)

	mcpServer.AddResource(&mcpsdk.Resource{
		URI:         "writekit://blog/recent-comments",
		Name:        "Recent Comments",
		Description: "Recent comments across all posts",
		MIMEType:    "text/plain",
	}, s.recentCommentsResource)
}

func (s *Server) resolveResourceTenant(ctx context.Context) (*tenant.DB, string, error) {
	user := auth.UserFromContext(ctx)
	if user == nil {
		return nil, "", fmt.Errorf("not authenticated")
	}
	return s.resolveTenant(user.ID, "")
}

func (s *Server) statsResource(ctx context.Context, req *mcpsdk.ReadResourceRequest) (*mcpsdk.ReadResourceResult, error) {
	db, _, err := s.resolveResourceTenant(ctx)
	if err != nil {
		return nil, err
	}

	published, _ := db.CountPosts(ctx, "published")
	drafts, _ := db.CountPosts(ctx, "draft")
	comments, _ := db.CountComments(ctx)

	data, _ := json.Marshal(map[string]int{
		"published_posts": published,
		"draft_posts":     drafts,
		"total_comments":  comments,
	})

	return &mcpsdk.ReadResourceResult{
		Contents: []*mcpsdk.ResourceContents{
			{URI: "writekit://blog/stats", MIMEType: "application/json", Text: string(data)},
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
			{URI: "writekit://blog/settings", MIMEType: "application/json", Text: string(data)},
		},
	}, nil
}

func (s *Server) recentPostsResource(ctx context.Context, req *mcpsdk.ReadResourceRequest) (*mcpsdk.ReadResourceResult, error) {
	db, _, err := s.resolveResourceTenant(ctx)
	if err != nil {
		return nil, err
	}

	posts, _ := db.ListPosts(ctx, tenant.PostFilter{Status: "published", Limit: 10})

	var sb strings.Builder
	for _, p := range posts {
		sb.WriteString(fmt.Sprintf("- %s (slug: %s, published: %v)\n  %s\n",
			p.Title, p.Slug, p.PublishedAt, p.Excerpt))
	}

	return &mcpsdk.ReadResourceResult{
		Contents: []*mcpsdk.ResourceContents{
			{URI: "writekit://blog/recent-posts", MIMEType: "text/plain", Text: sb.String()},
		},
	}, nil
}

func (s *Server) draftsResource(ctx context.Context, req *mcpsdk.ReadResourceRequest) (*mcpsdk.ReadResourceResult, error) {
	db, _, err := s.resolveResourceTenant(ctx)
	if err != nil {
		return nil, err
	}

	posts, _ := db.ListPosts(ctx, tenant.PostFilter{Status: "draft", Limit: 50})

	var sb strings.Builder
	for _, p := range posts {
		sb.WriteString(fmt.Sprintf("- %s (ID: %s, created: %s)\n  %s\n",
			p.Title, p.ID, p.CreatedAt.Format("2006-01-02"), p.Excerpt))
	}
	if sb.Len() == 0 {
		sb.WriteString("No drafts.")
	}

	return &mcpsdk.ReadResourceResult{
		Contents: []*mcpsdk.ResourceContents{
			{URI: "writekit://blog/drafts", MIMEType: "text/plain", Text: sb.String()},
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
		sb.WriteString(fmt.Sprintf("- [%s] %s on post %s: %s\n",
			c.CreatedAt.Format("2006-01-02"), c.Author, c.PostID, c.Content))
	}
	if sb.Len() == 0 {
		sb.WriteString("No comments yet.")
	}

	return &mcpsdk.ReadResourceResult{
		Contents: []*mcpsdk.ResourceContents{
			{URI: "writekit://blog/recent-comments", MIMEType: "text/plain", Text: sb.String()},
		},
	}, nil
}
