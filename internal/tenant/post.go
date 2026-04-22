package tenant

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"html"
	"sort"
	"strings"
	"time"
	"unicode"

	"writekit/internal/markdown"
)

type Page struct {
	ID           string
	Title        string
	Slug         string
	Content      string
	ContentHTML  string
	SearchText   string
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
	Search       string
	CollectionID *string
	Sort         string
	Limit        int
	Offset       int
}

func (db *DB) CreatePage(ctx context.Context, p *Page) error {
	if p.Visibility == "" {
		p.Visibility = "public"
	}
	if p.SearchText == "" && p.Content != "" {
		p.SearchText = markdown.Plain(p.Content)
	}
	_, err := db.DB.ExecContext(ctx, `
		INSERT INTO pages (id, title, slug, content, content_html, search_text, excerpt, status, visibility, tags, collection_id, position, version, published_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, p.ID, p.Title, p.Slug, p.Content, p.ContentHTML, p.SearchText, p.Excerpt, p.Status, p.Visibility, p.Tags, p.CollectionID, p.Position, p.Version, p.PublishedAt)
	if err != nil {
		return fmt.Errorf("create page: %w", err)
	}
	return nil
}

func (db *DB) UpdatePage(ctx context.Context, p *Page) error {
	if p.SearchText == "" && p.Content != "" {
		p.SearchText = markdown.Plain(p.Content)
	}
	_, err := db.DB.ExecContext(ctx, `
		UPDATE pages SET title=?, slug=?, content=?, content_html=?, search_text=?, excerpt=?, status=?, visibility=?, tags=?, collection_id=?, position=?, version=?, published_at=?, updated_at=datetime('now')
		WHERE id=?
	`, p.Title, p.Slug, p.Content, p.ContentHTML, p.SearchText, p.Excerpt, p.Status, p.Visibility, p.Tags, p.CollectionID, p.Position, p.Version, p.PublishedAt, p.ID)
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
	if _, err := db.DB.ExecContext(ctx, `DELETE FROM pages WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete page %s: %w", id, err)
	}
	return nil
}

func (db *DB) GetPageRender(ctx context.Context, pageID string) ([]byte, error) {
	var html []byte
	err := db.DB.QueryRowContext(ctx, `SELECT html FROM page_renders WHERE page_id = ?`, pageID).Scan(&html)
	if err != nil {
		return nil, err
	}
	return html, nil
}

func (db *DB) SetPageRender(ctx context.Context, pageID string, html []byte) error {
	_, err := db.DB.ExecContext(ctx, `
		INSERT INTO page_renders (page_id, html) VALUES (?, ?)
		ON CONFLICT(page_id) DO UPDATE SET html = excluded.html, updated_at = datetime('now')
	`, pageID, html)
	if err != nil {
		return fmt.Errorf("set page render %s: %w", pageID, err)
	}
	return nil
}

func (db *DB) ClearPageRenders(ctx context.Context) error {
	_, err := db.DB.ExecContext(ctx, `DELETE FROM page_renders`)
	if err != nil {
		return fmt.Errorf("clear page renders: %w", err)
	}
	return nil
}

func (db *DB) GetTagRender(ctx context.Context, slug string) ([]byte, string, error) {
	var html []byte
	var name string
	err := db.DB.QueryRowContext(ctx, `SELECT html, name FROM tag_renders WHERE slug = ?`, slug).Scan(&html, &name)
	if err != nil {
		return nil, "", err
	}
	return html, name, nil
}

func (db *DB) SetTagRender(ctx context.Context, slug, name string, html []byte) error {
	_, err := db.DB.ExecContext(ctx, `
		INSERT INTO tag_renders (slug, name, html) VALUES (?, ?, ?)
		ON CONFLICT(slug) DO UPDATE SET name = excluded.name, html = excluded.html, updated_at = datetime('now')
	`, slug, name, html)
	if err != nil {
		return fmt.Errorf("set tag render %s: %w", slug, err)
	}
	return nil
}

func (db *DB) ClearTagRenders(ctx context.Context) error {
	_, err := db.DB.ExecContext(ctx, `DELETE FROM tag_renders`)
	if err != nil {
		return fmt.Errorf("clear tag renders: %w", err)
	}
	return nil
}

func (db *DB) GetTagIndexRender(ctx context.Context) ([]byte, error) {
	var html []byte
	err := db.DB.QueryRowContext(ctx, `SELECT html FROM tag_index_render WHERE id = 1`).Scan(&html)
	if err != nil {
		return nil, err
	}
	return html, nil
}

func (db *DB) SetTagIndexRender(ctx context.Context, html []byte) error {
	_, err := db.DB.ExecContext(ctx, `
		INSERT INTO tag_index_render (id, html) VALUES (1, ?)
		ON CONFLICT(id) DO UPDATE SET html = excluded.html, updated_at = datetime('now')
	`, html)
	if err != nil {
		return fmt.Errorf("set tag index render: %w", err)
	}
	return nil
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
	if s := strings.TrimSpace(f.Search); s != "" {
		like := "%" + strings.ToLower(s) + "%"
		where = append(where, "(LOWER(title) LIKE ? OR LOWER(slug) LIKE ? OR LOWER(search_text) LIKE ?)")
		args = append(args, like, like, like)
	}

	query := pageSelect
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	switch f.Sort {
	case "title":
		query += " ORDER BY LOWER(title) ASC"
	case "published":
		query += " ORDER BY published_at IS NULL, published_at DESC"
	case "created":
		query += " ORDER BY created_at DESC"
	default:
		query += " ORDER BY COALESCE(published_at, created_at) DESC"
	}

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

func (db *DB) ListAllTags(ctx context.Context) ([]string, error) {
	rows, err := db.DB.QueryContext(ctx, `SELECT tags FROM pages WHERE tags != '[]' AND tags != ''`)
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}
	defer rows.Close()

	seen := make(map[string]struct{})
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var tags []string
		if err := json.Unmarshal([]byte(raw), &tags); err != nil {
			continue
		}
		for _, t := range tags {
			t = strings.TrimSpace(t)
			if t != "" {
				seen[t] = struct{}{}
			}
		}
	}

	out := make([]string, 0, len(seen))
	for t := range seen {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i]) < strings.ToLower(out[j]) })
	return out, nil
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

type SearchHit struct {
	Page
	TitleHTML   string
	SnippetHTML string
}

func buildFTSQuery(input string) string {
	var tokens []string
	for _, raw := range strings.Fields(strings.ToLower(input)) {
		var b strings.Builder
		for _, r := range raw {
			if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
				b.WriteRune(r)
			}
		}
		if b.Len() > 0 {
			tokens = append(tokens, b.String())
		}
	}
	if len(tokens) == 0 {
		return ""
	}
	var parts []string
	for i, t := range tokens {
		if i == len(tokens)-1 {
			parts = append(parts, `"`+t+`"*`)
		} else {
			parts = append(parts, `"`+t+`"`)
		}
	}
	return strings.Join(parts, " ")
}

func escapeAroundMark(s string) string {
	const open, close = "<mark>", "</mark>"
	var b strings.Builder
	for {
		i := strings.Index(s, open)
		if i < 0 {
			b.WriteString(html.EscapeString(s))
			return b.String()
		}
		b.WriteString(html.EscapeString(s[:i]))
		b.WriteString(open)
		s = s[i+len(open):]
		j := strings.Index(s, close)
		if j < 0 {
			b.WriteString(html.EscapeString(s))
			return b.String()
		}
		b.WriteString(html.EscapeString(s[:j]))
		b.WriteString(close)
		s = s[j+len(close):]
	}
}

func (db *DB) SearchPages(ctx context.Context, query string) ([]SearchHit, error) {
	fts := buildFTSQuery(query)
	if fts == "" {
		return nil, nil
	}
	rows, err := db.DB.QueryContext(ctx, `
		SELECT p.id, p.title, p.slug, p.content, p.content_html, p.excerpt, p.status, p.visibility, p.tags, p.collection_id, p.position, p.version, p.published_at, p.created_at, p.updated_at,
		       highlight(pages_fts, 0, '<mark>', '</mark>') AS title_html,
		       snippet(pages_fts, 1, '<mark>', '</mark>', '…', 16) AS snippet_html
		FROM pages p
		JOIN pages_fts ON pages_fts.rowid = p.rowid
		WHERE pages_fts MATCH ?
		ORDER BY bm25(pages_fts, 10.0, 1.0, 3.0)
		LIMIT 20
	`, fts)
	if err != nil {
		return nil, fmt.Errorf("search pages: %w", err)
	}
	defer rows.Close()

	var hits []SearchHit
	for rows.Next() {
		var p Page
		var titleHTML, snippetHTML string
		err := rows.Scan(&p.ID, &p.Title, &p.Slug, &p.Content, &p.ContentHTML, &p.Excerpt,
			&p.Status, &p.Visibility, &p.Tags, &p.CollectionID, &p.Position, &p.Version,
			&p.PublishedAt, &p.CreatedAt, &p.UpdatedAt, &titleHTML, &snippetHTML)
		if err != nil {
			return nil, fmt.Errorf("scan search row: %w", err)
		}
		hits = append(hits, SearchHit{
			Page:        p,
			TitleHTML:   escapeAroundMark(titleHTML),
			SnippetHTML: escapeAroundMark(snippetHTML),
		})
	}
	return hits, nil
}

func (db *DB) CountStandalonePages(ctx context.Context, status string) (int, error) {
	var count int
	query := "SELECT COUNT(*) FROM pages WHERE collection_id IS NULL"
	args := []any{}
	if status != "" {
		query += " AND status = ?"
		args = append(args, status)
	}
	if err := db.DB.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("count standalone pages: %w", err)
	}
	return count, nil
}

func (db *DB) CountPages(ctx context.Context, status string) (int, error) {
	var count int
	query := "SELECT COUNT(*) FROM pages"
	if status != "" {
		query += " WHERE status = ?"
		if err := db.DB.QueryRowContext(ctx, query, status).Scan(&count); err != nil {
			return 0, fmt.Errorf("count pages (status=%q): %w", status, err)
		}
		return count, nil
	}
	if err := db.DB.QueryRowContext(ctx, query).Scan(&count); err != nil {
		return 0, fmt.Errorf("count pages: %w", err)
	}
	return count, nil
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
