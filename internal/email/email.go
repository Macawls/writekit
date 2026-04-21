package email

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sesv2/types"
)

type Sender struct {
	client  *sesv2.Client
	from    string
	enabled bool
}

func NewSender(from, region string) *Sender {
	if from == "" || region == "" {
		slog.Info("email disabled: SES_FROM or SES_REGION not set")
		return &Sender{enabled: false}
	}

	cfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion(region),
	)
	if err != nil {
		slog.Error("failed to load AWS config", "err", err)
		return &Sender{enabled: false}
	}

	return &Sender{
		client:  sesv2.NewFromConfig(cfg),
		from:    from,
		enabled: true,
	}
}

func (s *Sender) Send(ctx context.Context, to, subject, html string) error {
	if !s.enabled {
		slog.Debug("email skipped (disabled)", "to", to, "subject", subject)
		return nil
	}

	_, err := s.client.SendEmail(ctx, &sesv2.SendEmailInput{
		FromEmailAddress: aws.String(s.from),
		Destination: &types.Destination{
			ToAddresses: []string{to},
		},
		Content: &types.EmailContent{
			Simple: &types.Message{
				Subject: &types.Content{Data: aws.String(subject)},
				Body: &types.Body{
					Html: &types.Content{Data: aws.String(html)},
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("send email to %s: %w", to, err)
	}

	slog.Info("email sent", "to", to, "subject", subject)
	return nil
}

func (s *Sender) SendWelcome(ctx context.Context, to, name string) error {
	greeting := "Hi"
	if name != "" {
		greeting = "Hi " + name
	}
	html, err := render("welcome", map[string]any{"Greeting": greeting})
	if err != nil {
		return err
	}
	return s.Send(ctx, to, "Welcome to WriteKit", html)
}

func (s *Sender) SendMagicLink(ctx context.Context, to, link string) error {
	html, err := render("magic_link", map[string]any{"Link": link})
	if err != nil {
		return err
	}
	return s.Send(ctx, to, "Sign in to WriteKit", html)
}

func (s *Sender) SendTeamInvitation(ctx context.Context, to, inviterName, tenantName, role, acceptLink string) error {
	html, err := render("team_invitation", map[string]any{
		"InviterName": inviterName,
		"TenantName":  tenantName,
		"Role":        role,
		"AcceptLink":  acceptLink,
	})
	if err != nil {
		return err
	}
	return s.Send(ctx, to, fmt.Sprintf("%s invited you to %s on WriteKit", inviterName, tenantName), html)
}

func (s *Sender) SendTeamMemberAdded(ctx context.Context, to, tenantName, tenantURL, role string) error {
	html, err := render("team_member_added", map[string]any{
		"TenantName": tenantName,
		"TenantURL":  tenantURL,
		"Role":       role,
	})
	if err != nil {
		return err
	}
	return s.Send(ctx, to, fmt.Sprintf("You joined %s on WriteKit", tenantName), html)
}

func (s *Sender) SendTeamInviteAccepted(ctx context.Context, to, inviteeDisplay, tenantName, teamURL string) error {
	html, err := render("team_invite_accepted", map[string]any{
		"InviteeDisplay": inviteeDisplay,
		"TenantName":     tenantName,
		"TeamURL":        teamURL,
	})
	if err != nil {
		return err
	}
	return s.Send(ctx, to, fmt.Sprintf("%s joined %s", inviteeDisplay, tenantName), html)
}
