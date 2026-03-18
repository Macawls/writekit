package platform

import (
	"context"
	"fmt"
	"time"
)

type Subscription struct {
	ID                   string
	UserID               string
	StripeCustomerID     string
	StripeSubscriptionID string
	Status               string
	CurrentPeriodEnd     *time.Time
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

func (db *DB) GetSubscription(ctx context.Context, userID string) (*Subscription, error) {
	row := db.Pool.QueryRow(ctx, `
		SELECT id, user_id, stripe_customer_id, stripe_subscription_id, status, current_period_end, created_at, updated_at
		FROM subscriptions WHERE user_id = $1
	`, userID)

	var s Subscription
	err := row.Scan(&s.ID, &s.UserID, &s.StripeCustomerID, &s.StripeSubscriptionID,
		&s.Status, &s.CurrentPeriodEnd, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get subscription: %w", err)
	}
	return &s, nil
}

func (db *DB) UpsertSubscription(ctx context.Context, s *Subscription) error {
	_, err := db.Pool.Exec(ctx, `
		INSERT INTO subscriptions (user_id, stripe_customer_id, stripe_subscription_id, status, current_period_end)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id) DO UPDATE SET
			stripe_customer_id = EXCLUDED.stripe_customer_id,
			stripe_subscription_id = EXCLUDED.stripe_subscription_id,
			status = EXCLUDED.status,
			current_period_end = EXCLUDED.current_period_end,
			updated_at = NOW()
	`, s.UserID, s.StripeCustomerID, s.StripeSubscriptionID, s.Status, s.CurrentPeriodEnd)
	if err != nil {
		return fmt.Errorf("upsert subscription: %w", err)
	}
	return nil
}

func (db *DB) GetSubscriptionByCustomerID(ctx context.Context, customerID string) (*Subscription, error) {
	row := db.Pool.QueryRow(ctx, `
		SELECT id, user_id, stripe_customer_id, stripe_subscription_id, status, current_period_end, created_at, updated_at
		FROM subscriptions WHERE stripe_customer_id = $1
	`, customerID)

	var s Subscription
	err := row.Scan(&s.ID, &s.UserID, &s.StripeCustomerID, &s.StripeSubscriptionID,
		&s.Status, &s.CurrentPeriodEnd, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get subscription by customer: %w", err)
	}
	return &s, nil
}
