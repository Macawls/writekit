package tenant

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestMigrations(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open in-memory sqlite: %v", err)
	}
	defer sqlDB.Close()

	db := &DB{DB: sqlDB}
	if _, err := db.migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// Verify all migrations were recorded
	rows, err := sqlDB.Query("SELECT version FROM schema_migrations ORDER BY version")
	if err != nil {
		t.Fatalf("query schema_migrations: %v", err)
	}
	defer rows.Close()

	var versions []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			t.Fatalf("scan version: %v", err)
		}
		versions = append(versions, v)
	}

	entries, _ := migrationsFS.ReadDir("migrations")
	if len(versions) != len(entries) {
		t.Fatalf("expected %d migrations, got %d: %v", len(entries), len(versions), versions)
	}

	// Verify visibility columns exist
	for _, table := range []string{"pages", "collections"} {
		found := false
		rows, err := sqlDB.Query("PRAGMA table_info(" + table + ")")
		if err != nil {
			t.Fatalf("pragma table_info(%s): %v", table, err)
		}
		for rows.Next() {
			var cid int
			var name, typ string
			var notnull int
			var dflt *string
			var pk int
			if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
				t.Fatalf("scan column info: %v", err)
			}
			if name == "visibility" {
				found = true
			}
		}
		rows.Close()
		if !found {
			t.Errorf("table %s missing visibility column", table)
		}
	}

	// Verify idempotency — running again should not error
	if _, err := db.migrate(); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
}
