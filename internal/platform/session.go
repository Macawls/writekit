package platform

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

type Session struct {
	ID        string
	UserID    string
	ExpiresAt time.Time
	CreatedAt time.Time
}

const sessionDuration = 30 * 24 * time.Hour

func (db *DB) CreateSession(ctx context.Context, userID string) (*Session, error) {
	id, err := generateToken(32)
	if err != nil {
		return nil, err
	}

	expiresAt := time.Now().Add(sessionDuration)
	_, err = db.Pool.Exec(ctx, `
		INSERT INTO sessions (id, user_id, expires_at) VALUES ($1, $2, $3)
	`, id, userID, expiresAt)
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	return &Session{ID: id, UserID: userID, ExpiresAt: expiresAt}, nil
}

func (db *DB) GetSession(ctx context.Context, id string) (*Session, error) {
	row := db.Pool.QueryRow(ctx, `
		SELECT id, user_id, expires_at, created_at FROM sessions
		WHERE id = $1 AND expires_at > NOW()
	`, id)

	var s Session
	err := row.Scan(&s.ID, &s.UserID, &s.ExpiresAt, &s.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}
	return &s, nil
}

func (db *DB) DeleteSession(ctx context.Context, id string) error {
	_, err := db.Pool.Exec(ctx, `DELETE FROM sessions WHERE id = $1`, id)
	return err
}

func (db *DB) CleanExpiredSessions(ctx context.Context) error {
	_, err := db.Pool.Exec(ctx, `DELETE FROM sessions WHERE expires_at < NOW()`)
	return err
}

func generateToken(bytes int) (string, error) {
	b := make([]byte, bytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
