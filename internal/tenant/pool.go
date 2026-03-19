package tenant

import (
	"container/list"
	"database/sql"
	"embed"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	_ "modernc.org/sqlite"
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

	if migrated {
		if err := db.rerenderPosts(); err != nil {
			slog.Warn("failed to re-render posts after migration", "tenant", tenantID, "err", err)
		}
	}

	slog.Info("opened tenant db", "tenant", tenantID)
	return db, nil
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

func (p *Pool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for id := range p.dbs {
		p.dbs[id].db.Close()
	}
	p.dbs = make(map[string]*poolEntry)
	p.lru.Init()
}
