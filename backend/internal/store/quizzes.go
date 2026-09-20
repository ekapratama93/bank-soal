package store

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
)

const quizColumns = `id, subject_id, grade, questions, exam_type_id, batch_id, durasi_menit, started, created_at`

func scanQuiz(row pgx.Row) (*Quiz, error) {
	var q Quiz
	var questionsRaw []byte
	err := row.Scan(&q.ID, &q.SubjectID, &q.Grade, &questionsRaw, &q.ExamTypeID, &q.BatchID, &q.DurasiMenit, &q.Started, &q.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(questionsRaw, &q.Questions); err != nil {
		return nil, err
	}
	return &q, nil
}

func (s *Store) GetQuiz(ctx context.Context, id string) (*Quiz, error) {
	row := s.Pool.QueryRow(ctx, `select `+quizColumns+` from quizzes where id = $1`, id)
	return scanQuiz(row)
}

type QuizInput struct {
	SubjectID   string
	Grade       int
	ExamTypeID  string
	Questions   []Question
	BatchID     string
	DurasiMenit int
}

func (s *Store) CreateQuiz(ctx context.Context, in QuizInput) (*Quiz, error) {
	qJSON, err := json.Marshal(in.Questions)
	if err != nil {
		return nil, err
	}
	row := s.Pool.QueryRow(ctx,
		`insert into quizzes (subject_id, grade, exam_type_id, questions, batch_id, durasi_menit, started, expires_at)
		 values ($1, $2, $3, $4::jsonb, $5, $6, false, null)
		 returning `+quizColumns,
		in.SubjectID, in.Grade, in.ExamTypeID, string(qJSON), in.BatchID, in.DurasiMenit,
	)
	return scanQuiz(row)
}

func (s *Store) MarkQuizStarted(ctx context.Context, id string) error {
	_, err := s.Pool.Exec(ctx, `update quizzes set started = true where id = $1`, id)
	return err
}

func (s *Store) UpdateQuizQuestions(ctx context.Context, id string, questions []Question) error {
	qJSON, err := json.Marshal(questions)
	if err != nil {
		return err
	}
	_, err = s.Pool.Exec(ctx, `update quizzes set questions = $2::jsonb where id = $1`, id, string(qJSON))
	return err
}

func (s *Store) DeleteQuiz(ctx context.Context, id string) error {
	_, err := s.Pool.Exec(ctx, `delete from quizzes where id = $1`, id)
	return err
}

// BulkDeleteQuizzes returns how many rows were actually deleted (ids may
// contain duplicates or ids that no longer exist).
func (s *Store) BulkDeleteQuizzes(ctx context.Context, ids []string) (int, error) {
	tag, err := s.Pool.Exec(ctx, `delete from quizzes where id = any($1)`, ids)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

// DeletePoolForCombo deletes not-yet-started packages for a subject/grade/
// exam-type combo (used by pool reset and by material create/update/delete
// invalidation) and returns how many were removed.
func (s *Store) DeletePoolForCombo(ctx context.Context, subjectID string, grade int, examTypeID string) (int, error) {
	tag, err := s.Pool.Exec(ctx,
		`delete from quizzes where subject_id = $1 and grade = $2 and exam_type_id = $3 and started = false`,
		subjectID, grade, examTypeID,
	)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

// PoolForCombo returns every quiz package (any state) for a combo — the
// student "request" endpoint picks a random one from this set.
func (s *Store) PoolForCombo(ctx context.Context, subjectID string, grade int, examTypeID string) ([]Quiz, error) {
	rows, err := s.Pool.Query(ctx,
		`select `+quizColumns+` from quizzes where subject_id = $1 and grade = $2 and exam_type_id = $3`,
		subjectID, grade, examTypeID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Quiz{}
	for rows.Next() {
		var q Quiz
		var questionsRaw []byte
		if err := rows.Scan(&q.ID, &q.SubjectID, &q.Grade, &questionsRaw, &q.ExamTypeID, &q.BatchID, &q.DurasiMenit, &q.Started, &q.CreatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(questionsRaw, &q.Questions); err != nil {
			return nil, err
		}
		out = append(out, q)
	}
	return out, rows.Err()
}

// AvailableComboRow is one (subject, grade, exam_type) quiz row's metadata,
// used to build the /api/quiz/available aggregate.
type AvailableComboRow struct {
	SubjectID  *string
	Grade      int
	ExamTypeID string
	Started    bool
}

func (s *Store) ListQuizComboRows(ctx context.Context) ([]AvailableComboRow, error) {
	rows, err := s.Pool.Query(ctx, `select subject_id, grade, exam_type_id, started from quizzes`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AvailableComboRow{}
	for rows.Next() {
		var r AvailableComboRow
		if err := rows.Scan(&r.SubjectID, &r.Grade, &r.ExamTypeID, &r.Started); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// AdminQuizFilter filters the admin quiz-package listing.
type AdminQuizFilter struct {
	SubjectID  *string
	Grade      *int
	ExamTypeID *string
}

func (s *Store) ListQuizzesAdmin(ctx context.Context, f AdminQuizFilter) ([]Quiz, error) {
	sqlq := `
		select ` + quizColumns + ` from quizzes
		where ($1::uuid is null or subject_id = $1)
		  and ($2::int is null or grade = $2)
		  and ($3::uuid is null or exam_type_id = $3)
		order by created_at desc`
	rows, err := s.Pool.Query(ctx, sqlq, f.SubjectID, f.Grade, f.ExamTypeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Quiz{}
	for rows.Next() {
		var q Quiz
		var questionsRaw []byte
		if err := rows.Scan(&q.ID, &q.SubjectID, &q.Grade, &questionsRaw, &q.ExamTypeID, &q.BatchID, &q.DurasiMenit, &q.Started, &q.CreatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(questionsRaw, &q.Questions); err != nil {
			return nil, err
		}
		out = append(out, q)
	}
	return out, rows.Err()
}

// QuizzesMeta fetches (id, subject_id, grade, exam_type_id) for every quiz —
// used to decorate attempt/admin listings without pulling question data.
func (s *Store) QuizzesMeta(ctx context.Context) (map[string]Quiz, error) {
	rows, err := s.Pool.Query(ctx, `select id, subject_id, grade, exam_type_id from quizzes`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]Quiz{}
	for rows.Next() {
		var q Quiz
		if err := rows.Scan(&q.ID, &q.SubjectID, &q.Grade, &q.ExamTypeID); err != nil {
			return nil, err
		}
		out[q.ID] = q
	}
	return out, rows.Err()
}
