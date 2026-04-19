package tenant

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"writekit/internal/markdown"
)

type DB struct {
	DB       *sql.DB
	TenantID string
}

func (db *DB) migrate() (bool, error) {
	_, err := db.DB.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version TEXT PRIMARY KEY,
		applied_at DATETIME DEFAULT (datetime('now'))
	)`)
	if err != nil {
		return false, fmt.Errorf("create migrations table: %w", err)
	}

	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return false, fmt.Errorf("read migrations: %w", err)
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	applied := false
	for _, f := range files {
		var exists bool
		err := db.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = ?)", f).Scan(&exists)
		if err != nil {
			return false, fmt.Errorf("check migration %s: %w", f, err)
		}
		if exists {
			continue
		}

		content, err := migrationsFS.ReadFile("migrations/" + f)
		if err != nil {
			return false, fmt.Errorf("read migration %s: %w", f, err)
		}

		if _, err := db.DB.Exec(string(content)); err != nil {
			// Tolerate "duplicate column name" errors from ALTER TABLE
			// to handle migrations that were partially applied before being split.
			if strings.Contains(err.Error(), "duplicate column name") {
				slog.Warn("migration has duplicate column (already applied), skipping", "file", f, "err", err)
			} else {
				return false, fmt.Errorf("apply migration %s: %w", f, err)
			}
		}

		if _, err := db.DB.Exec("INSERT INTO schema_migrations (version) VALUES (?)", f); err != nil {
			return false, fmt.Errorf("record migration %s: %w", f, err)
		}
		applied = true
	}

	return applied, nil
}

func (db *DB) rerenderPages() error {
	rows, err := db.DB.QueryContext(context.Background(),
		"SELECT id, content FROM pages WHERE content != ''")
	if err != nil {
		return fmt.Errorf("query pages: %w", err)
	}
	defer rows.Close()

	updated := 0
	for rows.Next() {
		var id, content string
		if err := rows.Scan(&id, &content); err != nil {
			continue
		}
		func() {
			defer func() {
				if r := recover(); r != nil {
					slog.Warn("panic re-rendering page (isolated)", "tenant", db.TenantID, "id", id, "panic", r)
				}
			}()
			html, ok := safeRender(id, content)
			if !ok {
				return
			}
			plain := markdown.Plain(content)
			if _, err := db.DB.ExecContext(context.Background(),
				"UPDATE pages SET content_html = ?, search_text = ? WHERE id = ?", html, plain, id); err != nil {
				slog.Warn("failed to re-render page", "id", id, "err", err)
				return
			}
			updated++
		}()
	}

	if updated > 0 {
		if _, err := db.DB.ExecContext(context.Background(),
			"INSERT INTO pages_fts(pages_fts) VALUES('rebuild')"); err != nil {
			slog.Warn("failed to rebuild pages_fts after rerender", "tenant", db.TenantID, "err", err)
		}
	}

	slog.Info("re-rendered pages after migration", "tenant", db.TenantID, "count", updated)
	return rows.Err()
}

func safeRender(pageID, content string) (html string, ok bool) {
	defer func() {
		if r := recover(); r != nil {
			slog.Warn("panic rendering page during migration", "id", pageID, "panic", r)
			ok = false
		}
	}()
	rendered, err := markdown.Render(content)
	if err != nil {
		return "", false
	}
	return rendered, true
}

func (db *DB) Close() {
	db.DB.Close()
}
