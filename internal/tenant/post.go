package tenant

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type Page struct {
	ID           string
	Title        string
	Slug         string
	Content      string
	ContentHTML  string
	Excerpt      string
	Status       string
	Visibility   string
	Tags         string
	CollectionID *string
	Position     int
	Version      int
	PublishedAt  *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type PageFilter struct {
	Status       string
	Visibility   string
	Tag          string
	CollectionID *string
	Limit        int
	Offset       int
}

func (db *DB) CreatePage(ctx context.Context, p *Page) error {
	if p.Visibility == "" {
		p.Visibility = "public"
	}
	_, err := db.DB.ExecContext(ctx, `
		INSERT INTO pages (id, title, slug, content, content_html, excerpt, status, visibility, tags, collection_id, position, version, published_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, p.ID, p.Title, p.Slug, p.Content, p.ContentHTML, p.Excerpt, p.Status, p.Visibility, p.Tags, p.CollectionID, p.Position, p.Version, p.PublishedAt)
	if err != nil {
		return fmt.Errorf("create page: %w", err)
	}
	return nil
}

func (db *DB) UpdatePage(ctx context.Context, p *Page) error {
	_, err := db.DB.ExecContext(ctx, `
		UPDATE pages SET title=?, slug=?, content=?, content_html=?, excerpt=?, status=?, visibility=?, tags=?, collection_id=?, position=?, version=?, published_at=?, updated_at=datetime('now')
		WHERE id=?
	`, p.Title, p.Slug, p.Content, p.ContentHTML, p.Excerpt, p.Status, p.Visibility, p.Tags, p.CollectionID, p.Position, p.Version, p.PublishedAt, p.ID)
	if err != nil {
		return fmt.Errorf("update page: %w", err)
	}
	return nil
}

func (db *DB) UpdatePageContentHTML(ctx context.Context, id string, html string) error {
	_, err := db.DB.ExecContext(ctx, `UPDATE pages SET content_html=?, updated_at=datetime('now') WHERE id=?`, html, id)
	if err != nil {
		return fmt.Errorf("update page content_html: %w", err)
	}
	return nil
}

func (db *DB) DeletePage(ctx context.Context, id string) error {
	_, err := db.DB.ExecContext(ctx, `DELETE FROM pages WHERE id = ?`, id)
	return err
}

func (db *DB) GetPage(ctx context.Context, id string) (*Page, error) {
	return scanPage(db.DB.QueryRowContext(ctx,
		pageSelect+" WHERE id = ?", id))
}

func (db *DB) GetPageBySlug(ctx context.Context, slug string) (*Page, error) {
	return scanPage(db.DB.QueryRowContext(ctx,
		pageSelect+" WHERE slug = ?", slug))
}

func (db *DB) GetPageInCollection(ctx context.Context, collectionID, slug string) (*Page, error) {
	return scanPage(db.DB.QueryRowContext(ctx,
		pageSelect+" WHERE collection_id = ? AND slug = ?", collectionID, slug))
}

func (db *DB) GetStandalonePageBySlug(ctx context.Context, slug string) (*Page, error) {
	return scanPage(db.DB.QueryRowContext(ctx,
		pageSelect+" WHERE slug = ? AND collection_id IS NULL", slug))
}

func (db *DB) ListPages(ctx context.Context, f PageFilter) ([]Page, error) {
	var where []string
	var args []any

	if f.Status != "" {
		where = append(where, "status = ?")
		args = append(args, f.Status)
	}
	if f.Visibility != "" {
		where = append(where, "visibility = ?")
		args = append(args, f.Visibility)
	}
	if f.Tag != "" {
		where = append(where, "tags LIKE ?")
		args = append(args, "%\""+f.Tag+"\"%")
	}
	if f.CollectionID != nil {
		if *f.CollectionID == "" {
			where = append(where, "collection_id IS NULL")
		} else {
			where = append(where, "collection_id = ?")
			args = append(args, *f.CollectionID)
		}
	}

	query := pageSelect
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
		return nil, fmt.Errorf("list pages: %w", err)
	}
	defer rows.Close()

	var pages []Page
	for rows.Next() {
		p, err := scanPageRow(rows)
		if err != nil {
			return nil, err
		}
		pages = append(pages, *p)
	}
	return pages, nil
}

func (db *DB) ListCollectionPages(ctx context.Context, collectionID, sortOrder string, includeDrafts bool) ([]Page, error) {
	order := "position ASC"
	if sortOrder == "date" {
		order = "COALESCE(published_at, created_at) DESC"
	}

	statusFilter := " AND status = 'published'"
	if includeDrafts {
		statusFilter = ""
	}

	rows, err := db.DB.QueryContext(ctx,
		pageSelect+" WHERE collection_id = ?"+statusFilter+" ORDER BY "+order, collectionID)
	if err != nil {
		return nil, fmt.Errorf("list collection pages: %w", err)
	}
	defer rows.Close()

	var pages []Page
	for rows.Next() {
		p, err := scanPageRow(rows)
		if err != nil {
			return nil, err
		}
		pages = append(pages, *p)
	}
	return pages, nil
}

func (db *DB) SearchPages(ctx context.Context, query string) ([]Page, error) {
	rows, err := db.DB.QueryContext(ctx, `
		SELECT p.id, p.title, p.slug, p.content, p.content_html, p.excerpt, p.status, p.visibility, p.tags, p.collection_id, p.position, p.version, p.published_at, p.created_at, p.updated_at
		FROM pages p
		JOIN pages_fts ON posts_fts.rowid = p.rowid
		WHERE pages_fts MATCH ?
		ORDER BY rank
		LIMIT 20
	`, query)
	if err != nil {
		return nil, fmt.Errorf("search pages: %w", err)
	}
	defer rows.Close()

	var pages []Page
	for rows.Next() {
		p, err := scanPageRow(rows)
		if err != nil {
			return nil, err
		}
		pages = append(pages, *p)
	}
	return pages, nil
}

func (db *DB) CountPages(ctx context.Context, status string) (int, error) {
	var count int
	query := "SELECT COUNT(*) FROM pages"
	if status != "" {
		query += " WHERE status = ?"
		err := db.DB.QueryRowContext(ctx, query, status).Scan(&count)
		return count, err
	}
	err := db.DB.QueryRowContext(ctx, query).Scan(&count)
	return count, err
}

const pageSelect = `SELECT id, title, slug, content, content_html, excerpt, status, visibility, tags, collection_id, position, version, published_at, created_at, updated_at FROM pages`

type scanner interface {
	Scan(dest ...any) error
}

func scanPage(row *sql.Row) (*Page, error) {
	var p Page
	err := row.Scan(&p.ID, &p.Title, &p.Slug, &p.Content, &p.ContentHTML,
		&p.Excerpt, &p.Status, &p.Visibility, &p.Tags, &p.CollectionID, &p.Position, &p.Version, &p.PublishedAt, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func scanPageRow(rows *sql.Rows) (*Page, error) {
	var p Page
	err := rows.Scan(&p.ID, &p.Title, &p.Slug, &p.Content, &p.ContentHTML,
		&p.Excerpt, &p.Status, &p.Visibility, &p.Tags, &p.CollectionID, &p.Position, &p.Version, &p.PublishedAt, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

type PageVersion struct {
	ID          int
	PageID      string
	Version     int
	Title       string
	Content     string
	ContentHTML string
	CreatedAt   time.Time
}

const maxVersions = 20

func (db *DB) SavePageVersion(ctx context.Context, p *Page) error {
	_, err := db.DB.ExecContext(ctx, `
		INSERT INTO page_versions (page_id, version, title, content, content_html)
		VALUES (?, ?, ?, ?, ?)
	`, p.ID, p.Version, p.Title, p.Content, p.ContentHTML)
	if err != nil {
		return fmt.Errorf("save page version: %w", err)
	}

	_, _ = db.DB.ExecContext(ctx, `
		DELETE FROM page_versions WHERE page_id = ? AND version <= (? - ?)
	`, p.ID, p.Version, maxVersions)

	return nil
}

func (db *DB) GetPageVersion(ctx context.Context, pageID string, version int) (*PageVersion, error) {
	var v PageVersion
	err := db.DB.QueryRowContext(ctx, `
		SELECT id, page_id, version, title, content, content_html, created_at
		FROM page_versions WHERE page_id = ? AND version = ?
	`, pageID, version).Scan(&v.ID, &v.PageID, &v.Version, &v.Title, &v.Content, &v.ContentHTML, &v.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get page version: %w", err)
	}
	return &v, nil
}
