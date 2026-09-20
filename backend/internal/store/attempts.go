package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

const attemptColumns = `id, quiz_id, client_id, answers, score, expired, started_at, expires_at, submitted_at`

func scanAttempt(row pgx.Row) (*Attempt, error) {
	var a Attempt
	var answersRaw, scoreRaw []byte
	err := row.Scan(&a.ID, &a.QuizID, &a.ClientID, &answersRaw, &scoreRaw, &a.Expired, &a.StartedAt, &a.ExpiresAt, &a.SubmittedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if answersRaw != nil {
		if err := json.Unmarshal(answersRaw, &a.Answers); err != nil {
			return nil, err
		}
	}
	if scoreRaw != nil {
		var sc Score
		if err := json.Unmarshal(scoreRaw, &sc); err != nil {
			return nil, err
		}
		a.Score = &sc
	}
	return &a, nil
}

func (s *Store) GetAttempt(ctx context.Context, quizID, clientID string) (*Attempt, error) {
	row := s.Pool.QueryRow(ctx,
		`select `+attemptColumns+` from attempts where quiz_id = $1 and client_id = $2`,
		quizID, clientID,
	)
	return scanAttempt(row)
}

// CreateAttempt inserts a new attempt row. On a unique-violation race (two
// near-simultaneous requests both missed an existing row and both tried to
// insert), the caller should re-fetch via GetAttempt instead of failing —
// see db.IsUniqueViolation.
func (s *Store) CreateAttempt(ctx context.Context, quizID, clientID string, expiresAt time.Time) (*Attempt, error) {
	row := s.Pool.QueryRow(ctx,
		`insert into attempts (quiz_id, client_id, expires_at, expired)
		 values ($1, $2, $3, false)
		 returning `+attemptColumns,
		quizID, clientID, expiresAt,
	)
	return scanAttempt(row)
}

func (s *Store) SubmitAttempt(ctx context.Context, id string, answers map[string]any, score Score, expired bool, submittedAt time.Time) error {
	answersJSON, err := json.Marshal(answers)
	if err != nil {
		return err
	}
	scoreJSON, err := json.Marshal(score)
	if err != nil {
		return err
	}
	_, err = s.Pool.Exec(ctx,
		`update attempts set answers = $2::jsonb, score = $3::jsonb, expired = $4, submitted_at = $5 where id = $1`,
		id, string(answersJSON), string(scoreJSON), expired, submittedAt,
	)
	return err
}

// AttemptQuizIDsForClient lists every quiz_id this client has an attempt
// for (submitted or not) — used to exclude already-taken packages from the
// pool.
func (s *Store) AttemptQuizIDsForClient(ctx context.Context, clientID string) ([]string, error) {
	rows, err := s.Pool.Query(ctx, `select quiz_id from attempts where client_id = $1`, clientID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// SubmittedAttemptsForClient lists this client's submitted attempts,
// newest first — the /api/quiz/attempts history endpoint.
func (s *Store) SubmittedAttemptsForClient(ctx context.Context, clientID string) ([]Attempt, error) {
	rows, err := s.Pool.Query(ctx,
		`select `+attemptColumns+` from attempts
		 where client_id = $1 and submitted_at is not null
		 order by submitted_at desc`,
		clientID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Attempt{}
	for rows.Next() {
		var a Attempt
		var answersRaw, scoreRaw []byte
		if err := rows.Scan(&a.ID, &a.QuizID, &a.ClientID, &answersRaw, &scoreRaw, &a.Expired, &a.StartedAt, &a.ExpiresAt, &a.SubmittedAt); err != nil {
			return nil, err
		}
		if answersRaw != nil {
			if err := json.Unmarshal(answersRaw, &a.Answers); err != nil {
				return nil, err
			}
		}
		if scoreRaw != nil {
			var sc Score
			if err := json.Unmarshal(scoreRaw, &sc); err != nil {
				return nil, err
			}
			a.Score = &sc
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
