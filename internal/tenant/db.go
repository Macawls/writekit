package tenant

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
)

type DB struct {
	DB       *sql.DB
	TenantID string
}

func (db *DB) migrate() error {
	_, err := db.DB.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version TEXT PRIMARY KEY,
		applied_at DATETIME DEFAULT (datetime('now'))
	)`)
	if err != nil {
		return fmt.Errorf("create migrations table: %w", err)
	}

	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	for _, f := range files {
		var exists bool
		err := db.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = ?)", f).Scan(&exists)
		if err != nil {
			return fmt.Errorf("check migration %s: %w", f, err)
		}
		if exists {
			continue
		}

		content, err := migrationsFS.ReadFile("migrations/" + f)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", f, err)
		}

		if _, err := db.DB.Exec(string(content)); err != nil {
			return fmt.Errorf("apply migration %s: %w", f, err)
		}

		if _, err := db.DB.Exec("INSERT INTO schema_migrations (version) VALUES (?)", f); err != nil {
			return fmt.Errorf("record migration %s: %w", f, err)
		}
	}

	return nil
}

func (db *DB) Close() {
	db.DB.Close()
}
