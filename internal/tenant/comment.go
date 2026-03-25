package tenant

import (
	"context"
	"fmt"
	"time"
)

type Comment struct {
	ID        string
	PageID    string
	ParentID  *string
	Author    string
	Email     string
	Content   string
	CreatedAt time.Time
}

func (db *DB) CreateComment(ctx context.Context, c *Comment) error {
	_, err := db.DB.ExecContext(ctx, `
		INSERT INTO comments (id, page_id, parent_id, author, email, content)
		VALUES (?, ?, ?, ?, ?, ?)
	`, c.ID, c.PageID, c.ParentID, c.Author, c.Email, c.Content)
	if err != nil {
		return fmt.Errorf("create comment: %w", err)
	}
	return nil
}

func (db *DB) ListComments(ctx context.Context, postID string) ([]Comment, error) {
	rows, err := db.DB.QueryContext(ctx, `
		SELECT id, page_id, parent_id, author, email, content, created_at
		FROM comments WHERE page_id = ? ORDER BY created_at ASC
	`, postID)
	if err != nil {
		return nil, fmt.Errorf("list comments: %w", err)
	}
	defer rows.Close()

	var comments []Comment
	for rows.Next() {
		var c Comment
		if err := rows.Scan(&c.ID, &c.PageID, &c.ParentID, &c.Author, &c.Email, &c.Content, &c.CreatedAt); err != nil {
			return nil, err
		}
		comments = append(comments, c)
	}
	return comments, nil
}

func (db *DB) ListRecentComments(ctx context.Context, limit int) ([]Comment, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := db.DB.QueryContext(ctx, `
		SELECT id, page_id, parent_id, author, email, content, created_at
		FROM comments ORDER BY created_at DESC LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("list recent comments: %w", err)
	}
	defer rows.Close()

	var comments []Comment
	for rows.Next() {
		var c Comment
		if err := rows.Scan(&c.ID, &c.PageID, &c.ParentID, &c.Author, &c.Email, &c.Content, &c.CreatedAt); err != nil {
			return nil, err
		}
		comments = append(comments, c)
	}
	return comments, nil
}

func (db *DB) DeleteComment(ctx context.Context, id string) error {
	_, err := db.DB.ExecContext(ctx, `DELETE FROM comments WHERE id = ?`, id)
	return err
}

func (db *DB) CountComments(ctx context.Context) (int, error) {
	var count int
	err := db.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM comments").Scan(&count)
	return count, err
}
