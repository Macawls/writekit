package app

import (
	"context"
	"log/slog"

	"writekit/internal/config"
	"writekit/internal/email"
	"writekit/internal/events"
)

func subscribeEmailHandlers(bus *events.Bus, sender *email.Sender, cfg *config.Config) {
	bus.On(events.TeamInvitationCreated, func(e events.Event) {
		p, ok := e.Payload.(events.TeamInvitationCreatedPayload)
		if !ok {
			return
		}
		inviter := p.InviterName
		if inviter == "" {
			inviter = p.TenantName
		}
		acceptLink := cfg.BaseURL + "/invitations/accept?token=" + p.Token
		if err := sender.SendTeamInvitation(context.Background(), p.Email, inviter, p.TenantName, p.Role, acceptLink); err != nil {
			slog.Error("send team invitation email", "err", err, "email", p.Email)
		}
	})

	bus.On(events.TeamInvitationAccepted, func(e events.Event) {
		p, ok := e.Payload.(events.TeamInvitationAcceptedPayload)
		if !ok {
			return
		}
		scheme := "https"
		if cfg.Dev {
			scheme = "http"
		}
		tenantURL := scheme + "://" + e.TenantID + "." + cfg.Host
		if err := sender.SendTeamMemberAdded(context.Background(), p.Email, p.TenantName, tenantURL, p.Role); err != nil {
			slog.Error("send team member added email", "err", err, "email", p.Email)
		}
		teamURL := scheme + "://app." + cfg.Host + "/team"
		if err := sender.SendTeamInviteAccepted(context.Background(), p.InviterEmail, p.InviteeDisplay, p.TenantName, teamURL); err != nil {
			slog.Error("send invite accepted email", "err", err, "email", p.InviterEmail)
		}
	})
}
