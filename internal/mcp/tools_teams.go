package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"writekit/internal/auth"
	"writekit/internal/events"
	"writekit/internal/platform"
)

func (s *Server) registerTeamTools(mcpServer *mcpsdk.Server) {
	mcpServer.AddTool(&mcpsdk.Tool{
		Name:        "list_members",
		Description: "List all team members for your site.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"tenant_id": map[string]any{"type": "string", "description": "Site ID (only needed if you have multiple sites)"},
			},
		},
	}, s.listMembers)

	mcpServer.AddTool(&mcpsdk.Tool{
		Name:        "invite_member",
		Description: "Invite someone to your site's team by email. Sends them an email with a link to accept. They'll create a WriteKit account on the spot if they don't have one. Owners only.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"email":     map[string]any{"type": "string", "description": "Email address to invite"},
				"role":      map[string]any{"type": "string", "enum": []string{"editor", "viewer"}, "description": "Role: editor (can manage content) or viewer (can view private content)"},
				"tenant_id": map[string]any{"type": "string", "description": "Site ID (only needed if you have multiple sites)"},
			},
			"required": []string{"email", "role"},
		},
	}, s.inviteMember)

	mcpServer.AddTool(&mcpsdk.Tool{
		Name:        "list_invitations",
		Description: "List pending team invitations that haven't been accepted or revoked.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"tenant_id": map[string]any{"type": "string", "description": "Site ID (only needed if you have multiple sites)"},
			},
		},
	}, s.listInvitations)

	mcpServer.AddTool(&mcpsdk.Tool{
		Name:        "revoke_invitation",
		Description: "Revoke a pending team invitation by email. The invitation link stops working. Owners only.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"email":     map[string]any{"type": "string", "description": "Email of the pending invitation to revoke"},
				"tenant_id": map[string]any{"type": "string", "description": "Site ID (only needed if you have multiple sites)"},
			},
			"required": []string{"email"},
		},
	}, s.revokeInvitation)

	mcpServer.AddTool(&mcpsdk.Tool{
		Name:        "resend_invitation",
		Description: "Resend a pending team invitation, generating a new link and extending expiry. Owners only.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"email":     map[string]any{"type": "string", "description": "Email of the pending invitation to resend"},
				"tenant_id": map[string]any{"type": "string", "description": "Site ID (only needed if you have multiple sites)"},
			},
			"required": []string{"email"},
		},
	}, s.resendInvitation)

	mcpServer.AddTool(&mcpsdk.Tool{
		Name:        "remove_member",
		Description: "Remove a team member from your site. Cannot remove the last owner.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"email":     map[string]any{"type": "string", "description": "Email of the member to remove"},
				"tenant_id": map[string]any{"type": "string", "description": "Site ID (only needed if you have multiple sites)"},
			},
			"required": []string{"email"},
		},
	}, s.removeMember)

	mcpServer.AddTool(&mcpsdk.Tool{
		Name:        "update_member_role",
		Description: "Change a team member's role. Cannot demote the last owner.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"email":     map[string]any{"type": "string", "description": "Email of the member"},
				"role":      map[string]any{"type": "string", "enum": []string{"owner", "editor", "viewer"}, "description": "New role"},
				"tenant_id": map[string]any{"type": "string", "description": "Site ID (only needed if you have multiple sites)"},
			},
			"required": []string{"email", "role"},
		},
	}, s.updateMemberRole)
}

func (s *Server) listMembers(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
	user := auth.UserFromContext(ctx)
	if user == nil {
		return toolError("not authenticated — please sign in at the WriteKit website first"), nil
	}

	var args struct {
		TenantID string `json:"tenant_id"`
	}
	raw, _ := json.Marshal(req.Params.Arguments)
	json.Unmarshal(raw, &args)

	_, tenantID, err := s.resolveTenant(user.ID, args.TenantID)
	if err != nil {
		return toolError(err.Error()), nil
	}

	members, err := s.PlatformDB.ListTeamMembers(ctx, tenantID)
	if err != nil {
		return toolError(fmt.Sprintf("failed to list members: %v", err)), nil
	}

	if len(members) == 0 {
		return toolResult("No team members found."), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Team (%d member(s)):\n\n", len(members)))
	for _, m := range members {
		name := m.Name
		if name == "" {
			name = m.Email
		}
		sb.WriteString(fmt.Sprintf("- **%s** <%s> — %s\n", name, m.Email, m.Role))
	}
	return toolResult(sb.String()), nil
}

func (s *Server) inviteMember(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
	user := auth.UserFromContext(ctx)
	if user == nil {
		return toolError("not authenticated — please sign in at the WriteKit website first"), nil
	}

	var args struct {
		Email    string `json:"email"`
		Role     string `json:"role"`
		TenantID string `json:"tenant_id"`
	}
	raw, _ := json.Marshal(req.Params.Arguments)
	json.Unmarshal(raw, &args)

	_, tenantID, err := s.resolveTenantWithRole(ctx, user.ID, args.TenantID, "owner")
	if err != nil {
		return toolError(err.Error()), nil
	}

	if args.Role != "editor" && args.Role != "viewer" {
		return toolError("role must be 'editor' or 'viewer'"), nil
	}

	inv, err := s.PlatformDB.CreateInvitation(ctx, tenantID, args.Email, args.Role, user.ID)
	if err != nil {
		switch {
		case errors.Is(err, platform.ErrAlreadyTeamMember):
			return toolError(fmt.Sprintf("%s is already a team member", args.Email)), nil
		case errors.Is(err, platform.ErrInvitationExists):
			return toolError(fmt.Sprintf("a pending invitation for %s already exists — use resend_invitation to send a new link", args.Email)), nil
		case errors.Is(err, platform.ErrInvitationRateLimited):
			return toolError("too many invitations sent recently for this site — try again later"), nil
		default:
			return toolError(fmt.Sprintf("failed to create invitation: %v", err)), nil
		}
	}

	tenant, err := s.PlatformDB.GetTenant(ctx, tenantID)
	tenantName := tenantID
	if err == nil {
		tenantName = tenant.Name
	}

	s.Bus.Emit(events.Event{
		Type:     events.TeamInvitationCreated,
		TenantID: tenantID,
		Payload: events.TeamInvitationCreatedPayload{
			InvitationID: inv.ID,
			Email:        inv.Email,
			Role:         inv.Role,
			Token:        inv.Token,
			TenantName:   tenantName,
			InviterName:  user.Name,
		},
	})

	return toolResult(fmt.Sprintf("Invitation sent to **%s** as **%s**. Expires in 14 days.", inv.Email, inv.Role)), nil
}

func (s *Server) listInvitations(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
	user := auth.UserFromContext(ctx)
	if user == nil {
		return toolError("not authenticated — please sign in at the WriteKit website first"), nil
	}

	var args struct {
		TenantID string `json:"tenant_id"`
	}
	raw, _ := json.Marshal(req.Params.Arguments)
	json.Unmarshal(raw, &args)

	_, tenantID, err := s.resolveTenant(user.ID, args.TenantID)
	if err != nil {
		return toolError(err.Error()), nil
	}

	invs, err := s.PlatformDB.ListPendingInvitations(ctx, tenantID)
	if err != nil {
		return toolError(fmt.Sprintf("failed to list invitations: %v", err)), nil
	}

	if len(invs) == 0 {
		return toolResult("No pending invitations."), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Pending invitations (%d):\n\n", len(invs)))
	for _, inv := range invs {
		sb.WriteString(fmt.Sprintf("- **%s** — %s (invited by %s, expires %s)\n",
			inv.Email, inv.Role, inv.InviterEmail, inv.ExpiresAt.Format("2006-01-02")))
	}
	return toolResult(sb.String()), nil
}

func (s *Server) revokeInvitation(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
	user := auth.UserFromContext(ctx)
	if user == nil {
		return toolError("not authenticated — please sign in at the WriteKit website first"), nil
	}

	var args struct {
		Email    string `json:"email"`
		TenantID string `json:"tenant_id"`
	}
	raw, _ := json.Marshal(req.Params.Arguments)
	json.Unmarshal(raw, &args)

	_, tenantID, err := s.resolveTenantWithRole(ctx, user.ID, args.TenantID, "owner")
	if err != nil {
		return toolError(err.Error()), nil
	}

	inv, err := s.findPendingInvitation(ctx, tenantID, args.Email)
	if err != nil {
		return toolError(err.Error()), nil
	}

	if err := s.PlatformDB.RevokeInvitation(ctx, tenantID, inv.ID); err != nil {
		return toolError(fmt.Sprintf("failed to revoke: %v", err)), nil
	}

	return toolResult(fmt.Sprintf("Invitation for **%s** revoked.", inv.Email)), nil
}

func (s *Server) resendInvitation(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
	user := auth.UserFromContext(ctx)
	if user == nil {
		return toolError("not authenticated — please sign in at the WriteKit website first"), nil
	}

	var args struct {
		Email    string `json:"email"`
		TenantID string `json:"tenant_id"`
	}
	raw, _ := json.Marshal(req.Params.Arguments)
	json.Unmarshal(raw, &args)

	_, tenantID, err := s.resolveTenantWithRole(ctx, user.ID, args.TenantID, "owner")
	if err != nil {
		return toolError(err.Error()), nil
	}

	existing, err := s.findPendingInvitation(ctx, tenantID, args.Email)
	if err != nil {
		return toolError(err.Error()), nil
	}

	inv, err := s.PlatformDB.ResendInvitation(ctx, tenantID, existing.ID)
	if err != nil {
		return toolError(fmt.Sprintf("failed to resend: %v", err)), nil
	}

	tenant, terr := s.PlatformDB.GetTenant(ctx, tenantID)
	tenantName := tenantID
	if terr == nil {
		tenantName = tenant.Name
	}

	s.Bus.Emit(events.Event{
		Type:     events.TeamInvitationCreated,
		TenantID: tenantID,
		Payload: events.TeamInvitationCreatedPayload{
			InvitationID: inv.ID,
			Email:        inv.Email,
			Role:         inv.Role,
			Token:        inv.Token,
			TenantName:   tenantName,
			InviterName:  user.Name,
		},
	})

	return toolResult(fmt.Sprintf("Invitation resent to **%s**. New link is valid for 14 days.", inv.Email)), nil
}

func (s *Server) findPendingInvitation(ctx context.Context, tenantID, email string) (*platform.TeamInvitation, error) {
	invs, err := s.PlatformDB.ListPendingInvitations(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list invitations: %w", err)
	}
	email = strings.ToLower(strings.TrimSpace(email))
	for i := range invs {
		if invs[i].Email == email {
			return &invs[i], nil
		}
	}
	return nil, fmt.Errorf("no pending invitation for %s", email)
}

func (s *Server) removeMember(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
	user := auth.UserFromContext(ctx)
	if user == nil {
		return toolError("not authenticated — please sign in at the WriteKit website first"), nil
	}

	var args struct {
		Email    string `json:"email"`
		TenantID string `json:"tenant_id"`
	}
	raw, _ := json.Marshal(req.Params.Arguments)
	json.Unmarshal(raw, &args)

	_, tenantID, err := s.resolveTenantWithRole(ctx, user.ID, args.TenantID, "owner")
	if err != nil {
		return toolError(err.Error()), nil
	}

	target, err := s.PlatformDB.GetUserByEmail(ctx, args.Email)
	if err != nil {
		return toolError("user not found"), nil
	}

	if err := s.PlatformDB.RemoveTeamMember(ctx, tenantID, target.ID); err != nil {
		return toolError(err.Error()), nil
	}

	tenantName := tenantID
	if t, terr := s.PlatformDB.GetTenant(ctx, tenantID); terr == nil {
		tenantName = t.Name
	}
	removerName := user.Name
	if removerName == "" {
		removerName = user.Email
	}
	s.Bus.Emit(events.Event{
		Type:     events.TeamMemberRemoved,
		TenantID: tenantID,
		Payload: events.TeamMemberRemovedPayload{
			Email:       target.Email,
			TenantName:  tenantName,
			RemoverName: removerName,
		},
	})

	return toolResult(fmt.Sprintf("**%s** removed from team.", args.Email)), nil
}

func (s *Server) updateMemberRole(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
	user := auth.UserFromContext(ctx)
	if user == nil {
		return toolError("not authenticated — please sign in at the WriteKit website first"), nil
	}

	var args struct {
		Email    string `json:"email"`
		Role     string `json:"role"`
		TenantID string `json:"tenant_id"`
	}
	raw, _ := json.Marshal(req.Params.Arguments)
	json.Unmarshal(raw, &args)

	_, tenantID, err := s.resolveTenantWithRole(ctx, user.ID, args.TenantID, "owner")
	if err != nil {
		return toolError(err.Error()), nil
	}

	target, err := s.PlatformDB.GetUserByEmail(ctx, args.Email)
	if err != nil {
		return toolError("user not found"), nil
	}

	if err := s.PlatformDB.UpdateTeamMemberRole(ctx, tenantID, target.ID, args.Role); err != nil {
		return toolError(err.Error()), nil
	}

	return toolResult(fmt.Sprintf("**%s** role changed to **%s**.", args.Email, args.Role)), nil
}
