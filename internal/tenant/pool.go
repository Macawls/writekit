package tenant

import (
	"container/list"
	"context"
	"database/sql"
	"embed"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"
	_ "modernc.org/sqlite"
	"writekit/internal/markdown"
	"writekit/internal/welcome"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

type Pool struct {
	mu      sync.Mutex
	dbs     map[string]*poolEntry
	lru     *list.List
	maxSize int
	dataDir string
}

type poolEntry struct {
	db      *DB
	element *list.Element
}

func NewPool(dataDir string, maxSize int) (*Pool, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	return &Pool{
		dbs:     make(map[string]*poolEntry),
		lru:     list.New(),
		maxSize: maxSize,
		dataDir: dataDir,
	}, nil
}

func (p *Pool) Get(tenantID string) (*DB, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if entry, ok := p.dbs[tenantID]; ok {
		p.lru.MoveToFront(entry.element)
		return entry.db, nil
	}

	for p.lru.Len() >= p.maxSize {
		back := p.lru.Back()
		if back == nil {
			break
		}
		evictID := back.Value.(string)
		p.evict(evictID)
	}

	db, err := p.open(tenantID)
	if err != nil {
		slog.Error("tenant pool: open failed", "tenant", tenantID, "err", err)
		return nil, err
	}

	elem := p.lru.PushFront(tenantID)
	p.dbs[tenantID] = &poolEntry{db: db, element: elem}

	return db, nil
}

func (p *Pool) open(tenantID string) (*DB, error) {
	dbPath := filepath.Join(p.dataDir, tenantID+".db")
	sqlDB, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)")
	if err != nil {
		return nil, fmt.Errorf("open sqlite %s: %w", tenantID, err)
	}

	sqlDB.SetMaxOpenConns(1)

	db := &DB{DB: sqlDB, TenantID: tenantID}

	migrated, err := db.migrate()
	if err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("migrate sqlite %s: %w", tenantID, err)
	}

	needBackfill := migrated
	if !needBackfill {
		var pending int
		if err := sqlDB.QueryRow(
			"SELECT COUNT(*) FROM pages WHERE content != '' AND search_text = ''",
		).Scan(&pending); err == nil && pending > 0 {
			slog.Info("pages missing search_text, scheduling backfill", "tenant", tenantID, "pending", pending)
			needBackfill = true
		}
	}

	if needBackfill {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					slog.Error("panic re-rendering posts after migration", "tenant", tenantID, "panic", r)
				}
			}()
			if err := db.rerenderPages(); err != nil {
				slog.Warn("failed to re-render posts after migration", "tenant", tenantID, "err", err)
			}
		}()
	}

	if err := seedWelcomePage(db); err != nil {
		slog.Warn("seed welcome page", "tenant", tenantID, "err", err)
	}

	slog.Info("opened tenant db", "tenant", tenantID)
	return db, nil
}

func seedWelcomePage(db *DB) error {
	var count int
	if err := db.DB.QueryRow("SELECT COUNT(*) FROM pages").Scan(&count); err != nil {
		return fmt.Errorf("count pages: %w", err)
	}
	if count > 0 {
		return nil
	}
	html, err := markdown.Render(welcome.Markdown)
	if err != nil {
		html = ""
	}
	now := time.Now()
	return db.CreatePage(context.Background(), &Page{
		ID:          ulid.Make().String(),
		Title:       welcome.Title,
		Slug:        welcome.Slug,
		Content:     welcome.Markdown,
		ContentHTML: html,
		Excerpt:     welcome.Excerpt,
		Status:      "published",
		Visibility:  welcome.Visibility,
		Tags:        "[]",
		Version:     1,
		PublishedAt: &now,
	})
}

func (p *Pool) evict(tenantID string) {
	entry, ok := p.dbs[tenantID]
	if !ok {
		return
	}
	entry.db.Close()
	p.lru.Remove(entry.element)
	delete(p.dbs, tenantID)
	slog.Info("evicted tenant db", "tenant", tenantID)
}

func (p *Pool) Rename(oldID, newID string, commit func() error) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, ok := p.dbs[newID]; ok {
		p.evict(newID)
	}
	if entry, ok := p.dbs[oldID]; ok {
		entry.db.Close()
		p.lru.Remove(entry.element)
		delete(p.dbs, oldID)
	}

	oldPath := filepath.Join(p.dataDir, oldID+".db")
	newPath := filepath.Join(p.dataDir, newID+".db")

	if err := os.Rename(oldPath, newPath); err != nil {
		slog.Error("rename tenant db file", "old", oldID, "new", newID, "err", err)
		return fmt.Errorf("rename tenant db %s -> %s: %w", oldID, newID, err)
	}
	renameSidecar := func(from, to string) {
		if err := os.Rename(from, to); err != nil && !os.IsNotExist(err) {
			slog.Warn("rename sidecar file", "from", from, "to", to, "err", err)
		}
	}
	renameSidecar(oldPath+"-wal", newPath+"-wal")
	renameSidecar(oldPath+"-shm", newPath+"-shm")

	if commit != nil {
		if err := commit(); err != nil {
			if rbErr := os.Rename(newPath, oldPath); rbErr != nil {
				slog.Error("rollback file rename", "new", newID, "old", oldID, "err", rbErr)
			}
			renameSidecar(newPath+"-wal", oldPath+"-wal")
			renameSidecar(newPath+"-shm", oldPath+"-shm")
			return err
		}
	}

	slog.Info("renamed tenant db", "old", oldID, "new", newID)
	return nil
}

func (p *Pool) Delete(tenantID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if entry, ok := p.dbs[tenantID]; ok {
		entry.db.Close()
		p.lru.Remove(entry.element)
		delete(p.dbs, tenantID)
	}

	dbPath := filepath.Join(p.dataDir, tenantID+".db")
	os.Remove(dbPath + "-wal")
	os.Remove(dbPath + "-shm")
	if err := os.Remove(dbPath); err != nil && !os.IsNotExist(err) {
		slog.Error("delete tenant db file", "tenant", tenantID, "err", err)
		return fmt.Errorf("delete tenant db %s: %w", tenantID, err)
	}
	slog.Info("deleted tenant db", "tenant", tenantID)
	return nil
}

func (p *Pool) DataDir() string {
	return p.dataDir
}

// ActiveTenants returns tenant IDs currently held in the pool. Safe snapshot;
// callers can iterate without holding the pool lock. Used by the embedding
// worker's reconciliation sweep.
func (p *Pool) ActiveTenants() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	ids := make([]string, 0, len(p.dbs))
	for id := range p.dbs {
		ids = append(ids, id)
	}
	return ids
}

func (p *Pool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for id := range p.dbs {
		p.dbs[id].db.Close()
	}
	p.dbs = make(map[string]*poolEntry)
	p.lru.Init()
}
