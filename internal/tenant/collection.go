package tenant

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type Collection struct {
	ID          string
	Title       string
	Slug        string
	Description string
	Visibility  string
	SortOrder   string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (db *DB) CreateCollection(ctx context.Context, c *Collection) error {
	if c.Visibility == "" {
		c.Visibility = "public"
	}
	_, err := db.DB.ExecContext(ctx, `
		INSERT INTO collections (id, title, slug, description, visibility, sort_order)
		VALUES (?, ?, ?, ?, ?, ?)
	`, c.ID, c.Title, c.Slug, c.Description, c.Visibility, c.SortOrder)
	if err != nil {
		return fmt.Errorf("create collection: %w", err)
	}
	return nil
}

func (db *DB) UpdateCollection(ctx context.Context, c *Collection) error {
	_, err := db.DB.ExecContext(ctx, `
		UPDATE collections SET title=?, slug=?, description=?, visibility=?, sort_order=?, updated_at=datetime('now')
		WHERE id=?
	`, c.Title, c.Slug, c.Description, c.Visibility, c.SortOrder, c.ID)
	if err != nil {
		return fmt.Errorf("update collection: %w", err)
	}
	return nil
}

func (db *DB) DeleteCollection(ctx context.Context, id string) error {
	_, err := db.DB.ExecContext(ctx, `DELETE FROM collections WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete collection: %w", err)
	}
	return nil
}

func (db *DB) GetCollection(ctx context.Context, id string) (*Collection, error) {
	row := db.DB.QueryRowContext(ctx, `
		SELECT id, title, slug, description, visibility, sort_order, created_at, updated_at
		FROM collections WHERE id = ?
	`, id)
	return scanCollection(row)
}

func (db *DB) GetCollectionBySlug(ctx context.Context, slug string) (*Collection, error) {
	row := db.DB.QueryRowContext(ctx, `
		SELECT id, title, slug, description, visibility, sort_order, created_at, updated_at
		FROM collections WHERE slug = ?
	`, slug)
	return scanCollection(row)
}

func (db *DB) ListCollections(ctx context.Context) ([]Collection, error) {
	rows, err := db.DB.QueryContext(ctx, `
		SELECT id, title, slug, description, visibility, sort_order, created_at, updated_at
		FROM collections ORDER BY title
	`)
	if err != nil {
		return nil, fmt.Errorf("list collections: %w", err)
	}
	defer rows.Close()

	var collections []Collection
	for rows.Next() {
		var c Collection
		if err := rows.Scan(&c.ID, &c.Title, &c.Slug, &c.Description, &c.Visibility, &c.SortOrder, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		collections = append(collections, c)
	}
	return collections, nil
}

func (db *DB) CountCollectionPages(ctx context.Context, collectionID string) (int, error) {
	var count int
	err := db.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM pages WHERE collection_id = ?`, collectionID).Scan(&count)
	return count, err
}

func (db *DB) IsSlugAvailable(ctx context.Context, slug string) (bool, error) {
	var count int
	err := db.DB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM (
			SELECT slug FROM collections WHERE slug = ?
			UNION ALL
			SELECT slug FROM pages WHERE slug = ? AND collection_id IS NULL
		)
	`, slug, slug).Scan(&count)
	if err != nil {
		return false, err
	}
	return count == 0, nil
}

func (db *DB) ReorderPages(ctx context.Context, collectionID string, pageIDs []string) error {
	tx, err := db.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	for i, id := range pageIDs {
		_, err := tx.ExecContext(ctx, `UPDATE pages SET position = ? WHERE id = ? AND collection_id = ?`, i, id, collectionID)
		if err != nil {
			return fmt.Errorf("reorder page %s: %w", id, err)
		}
	}

	return tx.Commit()
}

func scanCollection(row *sql.Row) (*Collection, error) {
	var c Collection
	err := row.Scan(&c.ID, &c.Title, &c.Slug, &c.Description, &c.Visibility, &c.SortOrder, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}
