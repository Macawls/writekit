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
	return s.Send(ctx, to, "Welcome to WriteKit", welcomeHTML(name))
}

func (s *Sender) SendMagicLink(ctx context.Context, to, link string) error {
	return s.Send(ctx, to, "Sign in to WriteKit", magicLinkHTML(link))
}

