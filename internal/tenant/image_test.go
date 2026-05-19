package tenant

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func newTestDB(t *testing.T) *DB {
	t.Helper()
	sqlDB, err := sql.Open("sqlite", ":memory:?_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { sqlDB.Close() })
	db := &DB{DB: sqlDB}
	if _, err := db.migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestCreateImageDedupesOnSameID(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	img := &Image{ID: strings.Repeat("a", 64), Bytes: []byte("x"), Width: 10, Height: 10, SizeBytes: 1}
	created, err := db.CreateImage(ctx, img)
	if err != nil || !created {
		t.Fatalf("first insert: created=%v err=%v", created, err)
	}
	created2, err := db.CreateImage(ctx, img)
	if err != nil {
		t.Fatalf("second insert: %v", err)
	}
	if created2 {
		t.Fatalf("expected created=false on dedupe")
	}
}

func TestExtractImageRefs(t *testing.T) {
	hex := strings.Repeat("a", 64)
	hex2 := strings.Repeat("b", 64)
	md := "![one](/img/" + hex + ".webp) and ![two](/img/" + hex2 + ".webp) and again /img/" + hex + ".webp"
	ids := ExtractImageRefs(md)
	if len(ids) != 2 {
		t.Fatalf("want 2 unique ids, got %v", ids)
	}
	if ids[0] != hex || ids[1] != hex2 {
		t.Fatalf("unexpected order: %v", ids)
	}
}

func TestSyncImageRefsAndCascade(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	hexA := strings.Repeat("a", 64)
	hexB := strings.Repeat("b", 64)
	mustCreateImage(t, db, hexA)
	mustCreateImage(t, db, hexB)
	mustCreatePage(t, db, "p1", "![a](/img/"+hexA+".webp) ![b](/img/"+hexB+".webp)")

	if err := db.SyncImageRefs(ctx, "p1", "![a](/img/"+hexA+".webp) ![b](/img/"+hexB+".webp)"); err != nil {
		t.Fatalf("sync: %v", err)
	}
	if got := countRefs(t, db, "p1"); got != 2 {
		t.Fatalf("want 2 refs, got %d", got)
	}

	if err := db.SyncImageRefs(ctx, "p1", "![a](/img/"+hexA+".webp)"); err != nil {
		t.Fatalf("sync 2: %v", err)
	}
	if got := countRefs(t, db, "p1"); got != 1 {
		t.Fatalf("want 1 ref after update, got %d", got)
	}

	if err := db.DeletePage(ctx, "p1"); err != nil {
		t.Fatalf("delete page: %v", err)
	}
	if got := countRefs(t, db, "p1"); got != 0 {
		t.Fatalf("want 0 refs after cascade, got %d", got)
	}
}

func TestSyncImageRefsIgnoresUnknownIDs(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	hex := strings.Repeat("a", 64)
	mustCreatePage(t, db, "p1", "")
	if err := db.SyncImageRefs(ctx, "p1", "![a](/img/"+hex+".webp)"); err != nil {
		t.Fatalf("sync: %v", err)
	}
	if got := countRefs(t, db, "p1"); got != 0 {
		t.Fatalf("want 0 refs for unknown id, got %d", got)
	}
}

func TestDeleteUnreferencedImages(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	hexOld := strings.Repeat("a", 64)
	hexFresh := strings.Repeat("b", 64)
	hexRef := strings.Repeat("c", 64)
	mustCreateImage(t, db, hexOld)
	mustCreateImage(t, db, hexFresh)
	mustCreateImage(t, db, hexRef)

	if _, err := db.DB.ExecContext(ctx, `UPDATE images SET created_at = ? WHERE id = ?`,
		time.Now().Add(-48*time.Hour).UTC().Format("2006-01-02 15:04:05"), hexOld); err != nil {
		t.Fatalf("backdate: %v", err)
	}
	if _, err := db.DB.ExecContext(ctx, `UPDATE images SET created_at = ? WHERE id = ?`,
		time.Now().Add(-48*time.Hour).UTC().Format("2006-01-02 15:04:05"), hexRef); err != nil {
		t.Fatalf("backdate ref: %v", err)
	}

	if _, err := db.DB.ExecContext(ctx, `INSERT INTO pages (id, title, slug, content, status, tags, version) VALUES ('p1', 't', 'p1', '', 'draft', '[]', 1)`); err != nil {
		t.Fatalf("insert page: %v", err)
	}
	if _, err := db.DB.ExecContext(ctx, `INSERT INTO image_refs (image_id, page_id) VALUES (?, 'p1')`, hexRef); err != nil {
		t.Fatalf("insert ref: %v", err)
	}

	n, err := db.DeleteUnreferencedImages(ctx, 24*time.Hour)
	if err != nil {
		t.Fatalf("delete unrefd: %v", err)
	}
	if n != 1 {
		t.Fatalf("want 1 deletion (only hexOld), got %d", n)
	}

	if _, err := db.GetImage(ctx, hexOld); err == nil {
		t.Fatalf("hexOld should be gone")
	}
	if _, err := db.GetImage(ctx, hexFresh); err != nil {
		t.Fatalf("hexFresh should remain: %v", err)
	}
	if _, err := db.GetImage(ctx, hexRef); err != nil {
		t.Fatalf("hexRef should remain: %v", err)
	}
}

func mustCreateImage(t *testing.T, db *DB, id string) {
	t.Helper()
	if _, err := db.CreateImage(context.Background(), &Image{
		ID: id, Bytes: []byte{0}, Width: 1, Height: 1, SizeBytes: 1,
	}); err != nil {
		t.Fatalf("create image %s: %v", id, err)
	}
}

func mustCreatePage(t *testing.T, db *DB, id, content string) {
	t.Helper()
	if err := db.CreatePage(context.Background(), &Page{
		ID: id, Title: "t", Slug: id, Content: content, Status: "draft", Tags: "[]", Version: 1,
	}); err != nil {
		t.Fatalf("create page %s: %v", id, err)
	}
}

func countRefs(t *testing.T, db *DB, pageID string) int {
	t.Helper()
	var n int
	if err := db.DB.QueryRow(`SELECT COUNT(*) FROM image_refs WHERE page_id = ?`, pageID).Scan(&n); err != nil {
		t.Fatalf("count refs: %v", err)
	}
	return n
}
