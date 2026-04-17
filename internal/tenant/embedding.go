package tenant

import (
	"context"
	"encoding/binary"
	"fmt"
	"log/slog"
	"math"
	"time"
)

type PageEmbedding struct {
	PageID    string
	Model     string
	Dims      int
	Vec       []float32
	UpdatedAt time.Time
}

func (db *DB) UpsertPageEmbedding(ctx context.Context, pageID, model string, vec []float32) error {
	blob := packFloat32(vec)
	_, err := db.DB.ExecContext(ctx, `
		INSERT INTO page_embeddings (page_id, model, dims, vec, updated_at)
		VALUES (?, ?, ?, ?, datetime('now'))
		ON CONFLICT(page_id) DO UPDATE SET
			model=excluded.model,
			dims=excluded.dims,
			vec=excluded.vec,
			updated_at=excluded.updated_at
	`, pageID, model, len(vec), blob)
	if err != nil {
		return fmt.Errorf("upsert page embedding: %w", err)
	}
	return nil
}

func (db *DB) DeletePageEmbedding(ctx context.Context, pageID string) error {
	_, err := db.DB.ExecContext(ctx, `DELETE FROM page_embeddings WHERE page_id = ?`, pageID)
	return err
}

func (db *DB) ListPageEmbeddings(ctx context.Context, model string) ([]PageEmbedding, error) {
	rows, err := db.DB.QueryContext(ctx, `
		SELECT e.page_id, e.model, e.dims, e.vec, e.updated_at
		FROM page_embeddings e
		JOIN pages p ON p.id = e.page_id
		WHERE p.status = 'published'
		  AND e.model = ?
	`, model)
	if err != nil {
		return nil, fmt.Errorf("list page embeddings: %w", err)
	}
	defer rows.Close()

	var out []PageEmbedding
	for rows.Next() {
		var e PageEmbedding
		var blob []byte
		if err := rows.Scan(&e.PageID, &e.Model, &e.Dims, &blob, &e.UpdatedAt); err != nil {
			return nil, err
		}
		e.Vec = unpackFloat32(blob, e.Dims)
		out = append(out, e)
	}
	return out, rows.Err()
}

func (db *DB) ListStalePageIDs(ctx context.Context, model string) ([]string, error) {
	rows, err := db.DB.QueryContext(ctx, `
		SELECT p.id
		FROM pages p
		LEFT JOIN page_embeddings e ON e.page_id = p.id
		WHERE p.status = 'published'
		  AND (
		       e.page_id IS NULL
		    OR e.model != ?
		    OR e.updated_at < p.updated_at
		  )
	`, model)
	if err != nil {
		return nil, fmt.Errorf("list stale page ids: %w", err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func packFloat32(vec []float32) []byte {
	buf := make([]byte, 4*len(vec))
	for i, v := range vec {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(v))
	}
	return buf
}

func unpackFloat32(buf []byte, dims int) []float32 {
	n := len(buf) / 4
	if dims > 0 && n != dims {
		slog.Warn("page embedding dims mismatch", "stored_dims", dims, "blob_floats", n)
		if n > dims {
			n = dims
		}
	}
	out := make([]float32, n)
	for i := 0; i < n; i++ {
		out[i] = math.Float32frombits(binary.LittleEndian.Uint32(buf[i*4:]))
	}
	return out
}
