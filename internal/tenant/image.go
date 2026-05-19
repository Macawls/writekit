package tenant

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"time"
)

type Image struct {
	ID         string
	Bytes      []byte
	Width      int
	Height     int
	SizeBytes  int64
	FrameCount int
	CreatedAt  time.Time
}

func (img *Image) IsAnimated() bool { return img.FrameCount > 1 }

type ImageMeta struct {
	ID         string
	Width      int
	Height     int
	SizeBytes  int64
	FrameCount int
	CreatedAt  time.Time
}

func (m *ImageMeta) IsAnimated() bool { return m.FrameCount > 1 }

func (db *DB) CreateImage(ctx context.Context, img *Image) (bool, error) {
	frames := max(img.FrameCount, 1)
	res, err := db.DB.ExecContext(ctx, `
		INSERT OR IGNORE INTO images (id, bytes, width, height, size_bytes, frame_count)
		VALUES (?, ?, ?, ?, ?, ?)
	`, img.ID, img.Bytes, img.Width, img.Height, img.SizeBytes, frames)
	if err != nil {
		return false, fmt.Errorf("create image: %w", err)
	}
	n, _ := res.RowsAffected()
	return n == 1, nil
}

func (db *DB) GetImage(ctx context.Context, id string) (*Image, error) {
	var img Image
	err := db.DB.QueryRowContext(ctx, `
		SELECT id, bytes, width, height, size_bytes, frame_count, created_at
		FROM images WHERE id = ?
	`, id).Scan(&img.ID, &img.Bytes, &img.Width, &img.Height, &img.SizeBytes, &img.FrameCount, &img.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &img, nil
}

func (db *DB) DeleteImage(ctx context.Context, id string) error {
	_, err := db.DB.ExecContext(ctx, `DELETE FROM images WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete image %s: %w", id, err)
	}
	return nil
}

func (db *DB) ListImages(ctx context.Context, limit, offset int) ([]ImageMeta, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := db.DB.QueryContext(ctx, `
		SELECT id, width, height, size_bytes, frame_count, created_at
		FROM images ORDER BY created_at DESC LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list images: %w", err)
	}
	defer rows.Close()

	var out []ImageMeta
	for rows.Next() {
		var m ImageMeta
		if err := rows.Scan(&m.ID, &m.Width, &m.Height, &m.SizeBytes, &m.FrameCount, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}

func (db *DB) SumImageBytes(ctx context.Context) (int64, error) {
	var n sql.NullInt64
	err := db.DB.QueryRowContext(ctx, `SELECT SUM(size_bytes) FROM images`).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("sum image bytes: %w", err)
	}
	return n.Int64, nil
}

var imageRefPattern = regexp.MustCompile(`/img/([a-f0-9]{64})\.webp`)

func ExtractImageRefs(content string) []string {
	matches := imageRefPattern.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(matches))
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		if _, ok := seen[m[1]]; ok {
			continue
		}
		seen[m[1]] = struct{}{}
		out = append(out, m[1])
	}
	return out
}

func (db *DB) SyncImageRefs(ctx context.Context, pageID, content string) error {
	want := ExtractImageRefs(content)

	tx, err := db.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin sync image refs: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM image_refs WHERE page_id = ?`, pageID); err != nil {
		return fmt.Errorf("clear image refs: %w", err)
	}

	if len(want) > 0 {
		stmt, err := tx.PrepareContext(ctx, `
			INSERT OR IGNORE INTO image_refs (image_id, page_id)
			SELECT ?, ? WHERE EXISTS (SELECT 1 FROM images WHERE id = ?)
		`)
		if err != nil {
			return fmt.Errorf("prepare image ref insert: %w", err)
		}
		defer stmt.Close()

		for _, id := range want {
			if _, err := stmt.ExecContext(ctx, id, pageID, id); err != nil {
				return fmt.Errorf("insert image ref: %w", err)
			}
		}
	}

	return tx.Commit()
}

func (db *DB) DeleteUnreferencedImages(ctx context.Context, olderThan time.Duration) (int, error) {
	cutoff := time.Now().Add(-olderThan).UTC().Format("2006-01-02 15:04:05")
	res, err := db.DB.ExecContext(ctx, `
		DELETE FROM images
		WHERE created_at < ?
		  AND id NOT IN (SELECT image_id FROM image_refs)
	`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("delete unreferenced images: %w", err)
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}
