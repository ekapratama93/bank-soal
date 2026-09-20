// Package db wires up the Postgres connection pool and helpers for
// classifying Postgres errors (unique/foreign-key violations), replacing
// the Python backend's Supabase PostgREST client.
package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool opens a pgx connection pool against the given connection string
// (Supabase's pooler connection string works fine here).
func NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	if databaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL belum diatur di server")
	}
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("gagal membuat koneksi database: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("gagal terhubung ke database: %w", err)
	}
	return pool, nil
}
