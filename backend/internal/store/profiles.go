package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// ProfileRole returns the role ("admin"/"student") for a Supabase Auth
// user id, or "" if no profile row exists yet.
func (s *Store) ProfileRole(ctx context.Context, userID string) (string, error) {
	var role string
	err := s.Pool.QueryRow(ctx, `select role from profiles where id = $1`, userID).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return role, nil
}
