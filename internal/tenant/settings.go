package tenant

import (
	"context"
	"fmt"
)

func (db *DB) GetSettings(ctx context.Context) (map[string]string, error) {
	rows, err := db.DB.QueryContext(ctx, `SELECT key, value FROM settings`)
	if err != nil {
		return nil, fmt.Errorf("get settings: %w", err)
	}
	defer rows.Close()

	settings := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		settings[k] = v
	}
	return settings, nil
}

func (db *DB) UpdateSettings(ctx context.Context, updates map[string]string) error {
	tx, err := db.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for k, v := range updates {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO settings (key, value) VALUES (?, ?)
			ON CONFLICT(key) DO UPDATE SET value = excluded.value
		`, k, v)
		if err != nil {
			return fmt.Errorf("update setting %s: %w", k, err)
		}
	}

	return tx.Commit()
}
