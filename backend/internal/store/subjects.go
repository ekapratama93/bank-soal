package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

func (s *Store) ListSubjects(ctx context.Context) ([]Subject, error) {
	rows, err := s.Pool.Query(ctx, `select id, name, created_at from subjects order by name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Subject{} // never nil: marshals as [] rather than null when empty
	for rows.Next() {
		var sub Subject
		if err := rows.Scan(&sub.ID, &sub.Name, &sub.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, sub)
	}
	return out, rows.Err()
}

// SubjectNamesMap returns {id: name} for every subject — used to decorate
// materials/quizzes rows with a display name.
func (s *Store) SubjectNamesMap(ctx context.Context) (map[string]string, error) {
	rows, err := s.Pool.Query(ctx, `select id, name from subjects`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		out[id] = name
	}
	return out, rows.Err()
}

func (s *Store) GetSubjectByID(ctx context.Context, id string) (*Subject, error) {
	var sub Subject
	err := s.Pool.QueryRow(ctx, `select id, name, created_at from subjects where id = $1`, id).
		Scan(&sub.ID, &sub.Name, &sub.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &sub, nil
}

func (s *Store) GetSubjectByName(ctx context.Context, name string) (*Subject, error) {
	var sub Subject
	err := s.Pool.QueryRow(ctx, `select id, name, created_at from subjects where name = $1`, name).
		Scan(&sub.ID, &sub.Name, &sub.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &sub, nil
}

func (s *Store) CreateSubject(ctx context.Context, name string) (*Subject, error) {
	var sub Subject
	err := s.Pool.QueryRow(ctx,
		`insert into subjects (name) values ($1) returning id, name, created_at`, name,
	).Scan(&sub.ID, &sub.Name, &sub.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &sub, nil
}

func (s *Store) RenameSubject(ctx context.Context, id, name string) (*Subject, error) {
	var sub Subject
	err := s.Pool.QueryRow(ctx,
		`update subjects set name = $2 where id = $1 returning id, name, created_at`, id, name,
	).Scan(&sub.ID, &sub.Name, &sub.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &sub, nil
}

func (s *Store) DeleteSubject(ctx context.Context, id string) error {
	_, err := s.Pool.Exec(ctx, `delete from subjects where id = $1`, id)
	return err
}

func (s *Store) SubjectUsedByMaterials(ctx context.Context, id string) (bool, error) {
	return s.existsWhere(ctx, `select 1 from materials where subject_id = $1 limit 1`, id)
}

func (s *Store) SubjectUsedByQuizzes(ctx context.Context, id string) (bool, error) {
	return s.existsWhere(ctx, `select 1 from quizzes where subject_id = $1 limit 1`, id)
}

func (s *Store) existsWhere(ctx context.Context, query string, args ...any) (bool, error) {
	rows, err := s.Pool.Query(ctx, query, args...)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	return rows.Next(), rows.Err()
}
