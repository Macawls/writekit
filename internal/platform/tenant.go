package platform

import (
	"context"
	"fmt"
	"time"
)

type Tenant struct {
	ID        string
	UserID    string
	Name      string
	CreatedAt time.Time
}

func (db *DB) CreateTenant(ctx context.Context, t *Tenant) error {
	_, err := db.Pool.Exec(ctx, `
		INSERT INTO tenants (id, user_id, name) VALUES ($1, $2, $3)
	`, t.ID, t.UserID, t.Name)
	if err != nil {
		return fmt.Errorf("create tenant: %w", err)
	}
	return nil
}

func (db *DB) GetTenant(ctx context.Context, id string) (*Tenant, error) {
	row := db.Pool.QueryRow(ctx, `
		SELECT id, user_id, name, created_at FROM tenants WHERE id = $1
	`, id)

	var t Tenant
	err := row.Scan(&t.ID, &t.UserID, &t.Name, &t.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get tenant: %w", err)
	}
	return &t, nil
}

func (db *DB) ListTenantsByUser(ctx context.Context, userID string) ([]Tenant, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT id, user_id, name, created_at FROM tenants WHERE user_id = $1 ORDER BY created_at
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("list tenants: %w", err)
	}
	defer rows.Close()

	var tenants []Tenant
	for rows.Next() {
		var t Tenant
		if err := rows.Scan(&t.ID, &t.UserID, &t.Name, &t.CreatedAt); err != nil {
			return nil, err
		}
		tenants = append(tenants, t)
	}
	return tenants, nil
}

func (db *DB) DeleteTenant(ctx context.Context, id string) error {
	_, err := db.Pool.Exec(ctx, `DELETE FROM tenants WHERE id = $1`, id)
	return err
}
