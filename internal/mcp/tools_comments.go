package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"writekit/internal/auth"
	"writekit/internal/events"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func (s *Server) registerCommentTools(mcpServer *mcpsdk.Server) {
	mcpServer.AddTool(&mcpsdk.Tool{
		Name:        "list_comments",
		Description: "List comments on a page.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"page_id":   map[string]any{"type": "string", "description": "Post ID to list comments for"},
				"tenant_id": map[string]any{"type": "string", "description": "Site ID (only needed if you have multiple sites)"},
			},
			"required": []string{"page_id"},
		},
	}, s.listComments)

	mcpServer.AddTool(&mcpsdk.Tool{
		Name:        "delete_comment",
		Description: "Delete a comment by ID.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id":        map[string]any{"type": "string", "description": "Comment ID to delete"},
				"tenant_id": map[string]any{"type": "string", "description": "Site ID (only needed if you have multiple sites)"},
			},
			"required": []string{"id"},
		},
	}, s.deleteComment)
}

func (s *Server) listComments(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
	user := auth.UserFromContext(ctx)
	if user == nil {
		return toolError("not authenticated"), nil
	}

	var args struct {
		PageID   string `json:"page_id"`
		TenantID string `json:"tenant_id"`
	}
	raw, _ := json.Marshal(req.Params.Arguments)
	json.Unmarshal(raw, &args)

	db, _, err := s.resolveTenant(user.ID, args.TenantID)
	if err != nil {
		return toolError(err.Error()), nil
	}

	comments, err := db.ListComments(ctx, args.PageID)
	if err != nil {
		return toolError(fmt.Sprintf("failed to list comments: %v", err)), nil
	}

	if len(comments) == 0 {
		return toolResult("No comments on this page."), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%d comment(s):\n\n", len(comments)))
	for _, c := range comments {
		sb.WriteString(fmt.Sprintf("- **%s** (ID: %s, %s)\n  %s\n",
			c.Author, c.ID, c.CreatedAt.Format("2006-01-02 15:04"), c.Content))
	}

	return toolResult(sb.String()), nil
}

func (s *Server) deleteComment(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
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

	if err := db.DeleteComment(ctx, args.ID); err != nil {
		return toolError(fmt.Sprintf("failed to delete comment: %v", err)), nil
	}

	s.Bus.Emit(events.Event{Type: events.CommentDeleted, TenantID: tenantID})
	return toolResult("Comment deleted."), nil
}
