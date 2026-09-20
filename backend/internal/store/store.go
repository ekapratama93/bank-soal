package store

import "github.com/jackc/pgx/v5/pgxpool"

type Store struct {
	Pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Store {
	return &Store{Pool: pool}
}

// ErrNotFound is returned by single-row lookups that find nothing —
// callers map it to a 404, mirroring the Python routers' `if not res.data`.
type ErrNotFound struct{ What string }

func (e ErrNotFound) Error() string { return e.What + " tidak ditemukan" }
