package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"writekit/internal/auth"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func (s *Server) registerSettingsTools(mcpServer *mcpsdk.Server) {
	mcpServer.AddTool(&mcpsdk.Tool{
		Name: "get_settings",
		Description: `Get your blog settings. Available settings:
- **title**: Your blog's title (shown in header and RSS feed)
- **description**: A short description of your blog`,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"tenant_id": map[string]any{"type": "string", "description": "Blog ID (only needed if you have multiple blogs)"},
			},
		},
	}, s.getSettings)

	mcpServer.AddTool(&mcpsdk.Tool{
		Name:        "update_settings",
		Description: "Update blog settings. Only send the settings you want to change.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"title":       map[string]any{"type": "string", "description": "Blog title"},
				"description": map[string]any{"type": "string", "description": "Blog description"},
				"tenant_id":   map[string]any{"type": "string", "description": "Blog ID (only needed if you have multiple blogs)"},
			},
		},
	}, s.updateSettings)
}

func (s *Server) getSettings(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
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

	settings, err := db.GetSettings(ctx)
	if err != nil {
		return toolError(fmt.Sprintf("failed to get settings: %v", err)), nil
	}

	var sb strings.Builder
	sb.WriteString("Current blog settings:\n\n")
	for k, v := range settings {
		sb.WriteString(fmt.Sprintf("- **%s**: %s\n", k, v))
	}

	return toolResult(sb.String()), nil
}

func (s *Server) updateSettings(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
	user := auth.UserFromContext(ctx)
	if user == nil {
		return toolError("not authenticated"), nil
	}

	var args map[string]any
	raw, _ := json.Marshal(req.Params.Arguments)
	json.Unmarshal(raw, &args)

	tenantID, _ := args["tenant_id"].(string)
	db, _, err := s.resolveTenant(user.ID, tenantID)
	if err != nil {
		return toolError(err.Error()), nil
	}

	updates := make(map[string]string)
	for _, key := range []string{"title", "description"} {
		if v, ok := args[key].(string); ok {
			updates[key] = v
		}
	}

	if len(updates) == 0 {
		return toolError("no settings to update"), nil
	}

	if err := db.UpdateSettings(ctx, updates); err != nil {
		return toolError(fmt.Sprintf("failed to update settings: %v", err)), nil
	}

	return toolResult("Settings updated."), nil
}
