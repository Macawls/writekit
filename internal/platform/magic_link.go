package platform

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

const magicLinkDuration = 10 * time.Minute

type MagicLink struct {
	ID        string
	Email     string
	Token     string
	ExpiresAt time.Time
	Used      bool
	CreatedAt time.Time
}

func (db *DB) CreateMagicLink(ctx context.Context, email string) (*MagicLink, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}
	token := hex.EncodeToString(b)

	row := db.Pool.QueryRow(ctx, `
		INSERT INTO magic_links (email, token, expires_at)
		VALUES ($1, $2, NOW() + $3::interval)
		RETURNING id, email, token, expires_at, used, created_at
	`, email, token, fmt.Sprintf("%d seconds", int(magicLinkDuration.Seconds())))

	var ml MagicLink
	err := row.Scan(&ml.ID, &ml.Email, &ml.Token, &ml.ExpiresAt, &ml.Used, &ml.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create magic link: %w", err)
	}
	return &ml, nil
}

func (db *DB) ConsumeMagicLink(ctx context.Context, token string) (*MagicLink, error) {
	row := db.Pool.QueryRow(ctx, `
		UPDATE magic_links
		SET used = TRUE
		WHERE token = $1 AND used = FALSE AND expires_at > NOW()
		RETURNING id, email, token, expires_at, used, created_at
	`, token)

	var ml MagicLink
	err := row.Scan(&ml.ID, &ml.Email, &ml.Token, &ml.ExpiresAt, &ml.Used, &ml.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("consume magic link: %w", err)
	}
	return &ml, nil
}

func (db *DB) CleanExpiredMagicLinks(ctx context.Context) error {
	if _, err := db.Pool.Exec(ctx, `DELETE FROM magic_links WHERE expires_at < NOW()`); err != nil {
		return fmt.Errorf("clean expired magic links: %w", err)
	}
	return nil
}
