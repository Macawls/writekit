package tenant

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

type PreviewToken struct {
	Token     string
	PageID    string
	ExpiresAt time.Time
	CreatedAt time.Time
}

func (db *DB) CreatePreviewToken(ctx context.Context, postID string, duration time.Duration) (*PreviewToken, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("generate preview token: %w", err)
	}
	token := hex.EncodeToString(b)
	expiresAt := time.Now().Add(duration)

	_, err := db.DB.ExecContext(ctx, `
		INSERT INTO preview_tokens (token, page_id, expires_at) VALUES (?, ?, ?)
	`, token, postID, expiresAt)
	if err != nil {
		return nil, fmt.Errorf("create preview token: %w", err)
	}

	return &PreviewToken{Token: token, PageID: postID, ExpiresAt: expiresAt}, nil
}

func (db *DB) GetActivePreviewTokenForPage(ctx context.Context, pageID string) (*PreviewToken, error) {
	row := db.DB.QueryRowContext(ctx, `
		SELECT token, page_id, expires_at, created_at FROM preview_tokens
		WHERE page_id = ? AND expires_at > datetime('now')
		ORDER BY created_at DESC
		LIMIT 1
	`, pageID)

	var pt PreviewToken
	err := row.Scan(&pt.Token, &pt.PageID, &pt.ExpiresAt, &pt.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get active preview token for page: %w", err)
	}
	return &pt, nil
}

func (db *DB) RefreshPreviewToken(ctx context.Context, token string, duration time.Duration) error {
	_, err := db.DB.ExecContext(ctx, `
		UPDATE preview_tokens SET expires_at = ? WHERE token = ?
	`, time.Now().Add(duration), token)
	if err != nil {
		return fmt.Errorf("refresh preview token: %w", err)
	}
	return nil
}

func (db *DB) GetPreviewToken(ctx context.Context, token string) (*PreviewToken, error) {
	row := db.DB.QueryRowContext(ctx, `
		SELECT token, page_id, expires_at, created_at FROM preview_tokens
		WHERE token = ? AND expires_at > datetime('now')
	`, token)

	var pt PreviewToken
	err := row.Scan(&pt.Token, &pt.PageID, &pt.ExpiresAt, &pt.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get preview token: %w", err)
	}
	return &pt, nil
}

func (db *DB) CleanExpiredTokens(ctx context.Context) error {
	if _, err := db.DB.ExecContext(ctx, `DELETE FROM preview_tokens WHERE expires_at < datetime('now')`); err != nil {
		return fmt.Errorf("clean expired preview tokens: %w", err)
	}
	return nil
}
