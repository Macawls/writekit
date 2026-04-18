package app

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"strings"
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
	embedCallTimeout   = 90 * time.Second
	embedChunkSize     = 2000
	embedChunkOverlap  = 300
	embedMaxChunks     = 64
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
		text = page.Slug
	}
	if text == "" {
		return
	}
	chunks := chunkText(text, embedChunkSize, embedChunkOverlap)

	callCtx, cancel := context.WithTimeout(ctx, embedCallTimeout)
	defer cancel()

	start := time.Now()
	var sum []float32
	for i, chunk := range chunks {
		vec, err := w.client.Embed(callCtx, chunk)
		if err != nil {
			if errors.Is(err, embedding.ErrDisabled) {
				return
			}
			slog.Warn("embedding: ollama call failed", "tenant", job.tenantID, "page", page.ID, "chunk", i, "chunk_chars", len(chunk), "elapsed_ms", time.Since(start).Milliseconds(), "err", err)
			return
		}
		if sum == nil {
			sum = make([]float32, len(vec))
		}
		if len(vec) != len(sum) {
			slog.Warn("embedding: dim mismatch between chunks", "tenant", job.tenantID, "page", page.ID, "expected", len(sum), "got", len(vec))
			return
		}
		for j, v := range vec {
			sum[j] += v
		}
	}
	if sum == nil {
		return
	}

	pooled := meanPoolAndNormalize(sum, len(chunks))

	if err := db.UpsertPageEmbedding(ctx, page.ID, w.client.Model(), pooled); err != nil {
		slog.Warn("embedding: upsert", "tenant", job.tenantID, "page", page.ID, "err", err)
		return
	}
	slog.Info("embedding: upserted", "tenant", job.tenantID, "page", page.ID, "chars", len(text), "chunks", len(chunks), "dims", len(pooled), "elapsed_ms", time.Since(start).Milliseconds())
}

func chunkText(text string, size, overlap int) []string {
	if len(text) <= size {
		return []string{text}
	}
	var chunks []string
	start := 0
	for start < len(text) {
		end := start + size
		if end >= len(text) {
			chunks = append(chunks, text[start:])
			break
		}
		if b := findBoundary(text, end-100, end); b > start {
			end = b
		}
		chunks = append(chunks, text[start:end])
		if len(chunks) >= embedMaxChunks {
			break
		}
		start = max(end-overlap, 0)
	}
	return chunks
}

func findBoundary(s string, from, to int) int {
	if from < 0 {
		from = 0
	}
	if to > len(s) {
		to = len(s)
	}
	if from >= to {
		return -1
	}
	window := s[from:to]
	for _, sep := range []string{"\n\n", ". ", "\n"} {
		if i := strings.LastIndex(window, sep); i >= 0 {
			return from + i + len(sep)
		}
	}
	return -1
}

func meanPoolAndNormalize(sum []float32, n int) []float32 {
	out := make([]float32, len(sum))
	inv := 1.0 / float32(n)
	var sq float64
	for i, v := range sum {
		out[i] = v * inv
		sq += float64(out[i]) * float64(out[i])
	}
	if sq > 0 {
		norm := float32(1.0 / math.Sqrt(sq))
		for i := range out {
			out[i] *= norm
		}
	}
	return out
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
