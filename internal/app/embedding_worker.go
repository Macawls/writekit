package app

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"writekit/internal/embedding"
	"writekit/internal/events"
	"writekit/internal/markdown"
	"writekit/internal/tenant"
)

const (
	embedWorkerCount   = 4
	embedJobQueueSize  = 256
	embedSweepInterval = 60 * time.Second
	embedCallTimeout   = 30 * time.Second
)

type embedJob struct {
	tenantID string
	pageID   string
}

type EmbeddingWorker struct {
	pool     *tenant.Pool
	bus      *events.Bus
	client   *embedding.Client
	jobs     chan embedJob
	inFlight sync.Map
}

func NewEmbeddingWorker(pool *tenant.Pool, bus *events.Bus, client *embedding.Client) *EmbeddingWorker {
	return &EmbeddingWorker{
		pool:   pool,
		bus:    bus,
		client: client,
		jobs:   make(chan embedJob, embedJobQueueSize),
	}
}

func (w *EmbeddingWorker) Start(ctx context.Context) {
	if !w.client.Enabled() {
		slog.Info("embedding worker disabled (OLLAMA_HOST or EMBEDDING_MODEL unset)")
		return
	}

	slog.Info("embedding worker starting", "model", w.client.Model(), "workers", embedWorkerCount, "sweep", embedSweepInterval)

	for range embedWorkerCount {
		go func() {
			defer recoverGoroutine("embedding worker")
			w.runWorker(ctx)
		}()
	}

	for _, ev := range []string{events.PageCreated, events.PageUpdated, events.PageContentSaved, events.PagePublished} {
		w.bus.On(ev, func(e events.Event) {
			defer recoverGoroutine("embedding event handler")
			w.enqueue(e.TenantID, e.PageID)
		})
	}

	go func() {
		defer recoverGoroutine("embedding sweeper")
		w.runSweeper(ctx)
	}()
}

func recoverGoroutine(name string) {
	if r := recover(); r != nil {
		slog.Error("goroutine panic", "where", name, "panic", r)
	}
}

func (w *EmbeddingWorker) enqueue(tenantID, pageID string) {
	if tenantID == "" || pageID == "" {
		return
	}
	key := tenantID + "\x00" + pageID
	if _, loaded := w.inFlight.LoadOrStore(key, true); loaded {
		return
	}
	select {
	case w.jobs <- embedJob{tenantID: tenantID, pageID: pageID}:
	default:
		w.inFlight.Delete(key)
		slog.Warn("embedding queue full, dropping job", "tenant", tenantID, "page", pageID)
	}
}

func (w *EmbeddingWorker) runWorker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case job := <-w.jobs:
			w.process(ctx, job)
		}
	}
}

func (w *EmbeddingWorker) process(ctx context.Context, job embedJob) {
	defer w.inFlight.Delete(job.tenantID + "\x00" + job.pageID)

	db, err := w.pool.Get(job.tenantID)
	if err != nil {
		slog.Warn("embedding: get tenant db", "tenant", job.tenantID, "err", err)
		return
	}

	page, err := db.GetPage(ctx, job.pageID)
	if err != nil {
		return
	}

	if page.Status != "published" {
		if err := db.DeletePageEmbedding(ctx, page.ID); err != nil {
			slog.Warn("embedding: delete stale", "tenant", job.tenantID, "page", page.ID, "err", err)
		}
		return
	}

	text := markdown.PlainText(page.Content)
	if text == "" {
		text = page.Title
	}
	if text == "" {
		return
	}

	callCtx, cancel := context.WithTimeout(ctx, embedCallTimeout)
	defer cancel()

	vec, err := w.client.Embed(callCtx, text)
	if err != nil {
		if errors.Is(err, embedding.ErrDisabled) {
			return
		}
		slog.Warn("embedding: ollama call failed", "tenant", job.tenantID, "page", page.ID, "err", err)
		return
	}

	if err := db.UpsertPageEmbedding(ctx, page.ID, w.client.Model(), vec); err != nil {
		slog.Warn("embedding: upsert", "tenant", job.tenantID, "page", page.ID, "err", err)
	}
}

func (w *EmbeddingWorker) runSweeper(ctx context.Context) {
	w.sweep(ctx)

	ticker := time.NewTicker(embedSweepInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.sweep(ctx)
		}
	}
}

func (w *EmbeddingWorker) sweep(ctx context.Context) {
	model := w.client.Model()
	for _, tenantID := range w.pool.ActiveTenants() {
		db, err := w.pool.Get(tenantID)
		if err != nil {
			continue
		}
		ids, err := db.ListStalePageIDs(ctx, model)
		if err != nil {
			slog.Warn("embedding sweep: list stale", "tenant", tenantID, "err", err)
			continue
		}
		for _, id := range ids {
			w.enqueue(tenantID, id)
		}
	}
}
