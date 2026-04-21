package web

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"writekit/internal/events"
	"writekit/internal/httplog"
	"writekit/internal/platform"
)

func (h *Handler) AcceptInvitation(w http.ResponseWriter, r *http.Request) {
	log := httplog.FromContext(r.Context())
	token := r.URL.Query().Get("token")
	if token == "" {
		h.renderInvitationError(w, "This invitation link is missing its token.")
		return
	}

	inv, err := h.DB.GetInvitationByToken(r.Context(), token)
	if errors.Is(err, platform.ErrInvitationNotFound) {
		h.renderInvitationError(w, "This invitation is invalid or has been revoked.")
		return
	}
	if err != nil {
		log.Error("get invitation", "err", err)
		http.Error(w, "something went wrong", http.StatusInternalServerError)
		return
	}

	if inv.AcceptedAt != nil {
		h.renderInvitationError(w, "This invitation has already been accepted. Sign in to continue.")
		return
	}
	if inv.RevokedAt != nil {
		h.renderInvitationError(w, "This invitation has been revoked. Ask "+inv.InviterName+" for a new one.")
		return
	}
	if inv.IsExpired() {
		h.renderInvitationError(w, "This invitation has expired. Ask "+inv.InviterName+" for a new one.")
		return
	}

	sessionUser, _ := h.getSessionUser(r)

	if sessionUser != nil && !strings.EqualFold(sessionUser.Email, inv.Email) {
		h.Engine.Render(w, "accept_invitation.html", map[string]any{
			"WrongAccount":    true,
			"InvitationEmail": inv.Email,
			"TenantName":      inv.TenantName,
			"AcceptURL":       "/invitations/accept?token=" + token,
		})
		return
	}

	user := sessionUser
	if user == nil {
		existing, err := h.DB.FindUserByVerifiedEmail(r.Context(), inv.Email)
		if err == nil {
			user = existing
		} else {
			created, cerr := h.DB.CreateUser(r.Context(), inv.Email, deriveNameFromEmail(inv.Email), "")
			if cerr != nil {
				log.Error("create user from invitation", "email", inv.Email, "err", cerr)
				http.Error(w, "failed to create account", http.StatusInternalServerError)
				return
			}
			user = created
			if _, lerr := h.DB.LinkAccount(r.Context(), user.ID, "email", user.Email, user.Email, true); lerr != nil {
				log.Warn("link email provider for invitation signup", "user_id", user.ID, "err", lerr)
			}
		}
	}

	accepted, err := h.DB.AcceptInvitation(r.Context(), token, user.ID)
	if err != nil {
		log.Error("accept invitation", "err", err, "user_id", user.ID)
		h.renderInvitationError(w, "This invitation could not be accepted. It may have expired or been revoked.")
		return
	}

	inviteeDisplay := user.Name
	if inviteeDisplay == "" {
		inviteeDisplay = user.Email
	}
	h.Bus.Emit(events.Event{
		Type:     events.TeamInvitationAccepted,
		TenantID: accepted.TenantID,
		Payload: events.TeamInvitationAcceptedPayload{
			InvitationID:   accepted.ID,
			Email:          user.Email,
			Role:           accepted.Role,
			TenantName:     inv.TenantName,
			InviteeDisplay: inviteeDisplay,
			InviterEmail:   inv.InviterEmail,
		},
	})

	log.Info("invitation accepted", "user_id", user.ID, "tenant", accepted.TenantID, "role", accepted.Role)

	if sessionUser == nil {
		if !h.setSessionCookies(w, r, user.ID) {
			return
		}
	}

	scheme := "https"
	if h.Config.Dev {
		scheme = "http"
	}
	http.Redirect(w, r, fmt.Sprintf("%s://%s.%s", scheme, accepted.TenantID, h.Config.Host), http.StatusSeeOther)
}

func (h *Handler) renderInvitationError(w http.ResponseWriter, msg string) {
	h.Engine.Render(w, "accept_invitation.html", map[string]any{
		"Error": msg,
	})
}

func deriveNameFromEmail(email string) string {
	parts := strings.SplitN(email, "@", 2)
	if len(parts) == 0 || parts[0] == "" {
		return ""
	}
	local := parts[0]
	local = strings.ReplaceAll(local, ".", " ")
	local = strings.ReplaceAll(local, "_", " ")
	local = strings.ReplaceAll(local, "-", " ")
	words := strings.Fields(local)
	for i, w := range words {
		if w == "" {
			continue
		}
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}
