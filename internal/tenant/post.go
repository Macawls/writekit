package tenant

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type Post struct {
	ID          string
	Title       string
	Slug        string
	Content     string
	ContentHTML string
	Excerpt     string
	Status      string
	Tags        string
	PublishedAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type PostFilter struct {
	Status string
	Tag    string
	Limit  int
	Offset int
}

func (db *DB) CreatePost(ctx context.Context, p *Post) error {
	_, err := db.DB.ExecContext(ctx, `
		INSERT INTO posts (id, title, slug, content, content_html, excerpt, status, tags, published_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, p.ID, p.Title, p.Slug, p.Content, p.ContentHTML, p.Excerpt, p.Status, p.Tags, p.PublishedAt)
	if err != nil {
		return fmt.Errorf("create post: %w", err)
	}
	return nil
}

func (db *DB) UpdatePost(ctx context.Context, p *Post) error {
	_, err := db.DB.ExecContext(ctx, `
		UPDATE posts SET title=?, slug=?, content=?, content_html=?, excerpt=?, status=?, tags=?, published_at=?, updated_at=datetime('now')
		WHERE id=?
	`, p.Title, p.Slug, p.Content, p.ContentHTML, p.Excerpt, p.Status, p.Tags, p.PublishedAt, p.ID)
	if err != nil {
		return fmt.Errorf("update post: %w", err)
	}
	return nil
}

func (db *DB) DeletePost(ctx context.Context, id string) error {
	_, err := db.DB.ExecContext(ctx, `DELETE FROM posts WHERE id = ?`, id)
	return err
}

func (db *DB) GetPost(ctx context.Context, id string) (*Post, error) {
	return db.scanPost(db.DB.QueryRowContext(ctx, `
		SELECT id, title, slug, content, content_html, excerpt, status, tags, published_at, created_at, updated_at
		FROM posts WHERE id = ?
	`, id))
}

func (db *DB) GetPostBySlug(ctx context.Context, slug string) (*Post, error) {
	return db.scanPost(db.DB.QueryRowContext(ctx, `
		SELECT id, title, slug, content, content_html, excerpt, status, tags, published_at, created_at, updated_at
		FROM posts WHERE slug = ?
	`, slug))
}

func (db *DB) ListPosts(ctx context.Context, f PostFilter) ([]Post, error) {
	var where []string
	var args []any

	if f.Status != "" {
		where = append(where, "status = ?")
		args = append(args, f.Status)
	}
	if f.Tag != "" {
		where = append(where, "tags LIKE ?")
		args = append(args, "%\""+f.Tag+"\"%")
	}

	query := "SELECT id, title, slug, content, content_html, excerpt, status, tags, published_at, created_at, updated_at FROM posts"
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY COALESCE(published_at, created_at) DESC"

	limit := f.Limit
	if limit <= 0 {
		limit = 50
	}
	query += fmt.Sprintf(" LIMIT %d OFFSET %d", limit, f.Offset)

	rows, err := db.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list posts: %w", err)
	}
	defer rows.Close()

	var posts []Post
	for rows.Next() {
		p, err := scanPostRow(rows)
		if err != nil {
			return nil, err
		}
		posts = append(posts, *p)
	}
	return posts, nil
}

func (db *DB) SearchPosts(ctx context.Context, query string) ([]Post, error) {
	rows, err := db.DB.QueryContext(ctx, `
		SELECT p.id, p.title, p.slug, p.content, p.content_html, p.excerpt, p.status, p.tags, p.published_at, p.created_at, p.updated_at
		FROM posts p
		JOIN posts_fts ON posts_fts.rowid = p.rowid
		WHERE posts_fts MATCH ?
		ORDER BY rank
		LIMIT 20
	`, query)
	if err != nil {
		return nil, fmt.Errorf("search posts: %w", err)
	}
	defer rows.Close()

	var posts []Post
	for rows.Next() {
		p, err := scanPostRow(rows)
		if err != nil {
			return nil, err
		}
		posts = append(posts, *p)
	}
	return posts, nil
}

func (db *DB) CountPosts(ctx context.Context, status string) (int, error) {
	var count int
	query := "SELECT COUNT(*) FROM posts"
	if status != "" {
		query += " WHERE status = ?"
		err := db.DB.QueryRowContext(ctx, query, status).Scan(&count)
		return count, err
	}
	err := db.DB.QueryRowContext(ctx, query).Scan(&count)
	return count, err
}

type scanner interface {
	Scan(dest ...any) error
}

func (db *DB) scanPost(row *sql.Row) (*Post, error) {
	var p Post
	err := row.Scan(&p.ID, &p.Title, &p.Slug, &p.Content, &p.ContentHTML,
		&p.Excerpt, &p.Status, &p.Tags, &p.PublishedAt, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func scanPostRow(rows *sql.Rows) (*Post, error) {
	var p Post
	err := rows.Scan(&p.ID, &p.Title, &p.Slug, &p.Content, &p.ContentHTML,
		&p.Excerpt, &p.Status, &p.Tags, &p.PublishedAt, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}
