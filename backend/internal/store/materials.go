package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

const materialColumns = `id, subject_id, grade, exam_type_id, title, content, file_name, created_by, created_at`

func scanMaterial(row pgx.Row) (*Material, error) {
	var m Material
	err := row.Scan(&m.ID, &m.SubjectID, &m.Grade, &m.ExamTypeID, &m.Title, &m.Content, &m.FileName, &m.CreatedBy, &m.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

type MaterialFilter struct {
	SubjectID *string
	Grade     *int
}

func (s *Store) ListMaterials(ctx context.Context, f MaterialFilter) ([]Material, error) {
	sqlq := `
		select m.id, m.subject_id, m.grade, m.exam_type_id, m.title, m.content, m.file_name,
		       m.created_by, m.created_at, et.name
		from materials m
		left join exam_types et on et.id = m.exam_type_id
		where ($1::uuid is null or m.subject_id = $1)
		  and ($2::int is null or m.grade = $2)
		order by m.created_at desc`
	rows, err := s.Pool.Query(ctx, sqlq, f.SubjectID, f.Grade)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Material{} // never nil: marshals as [] rather than null when empty
	for rows.Next() {
		var m Material
		var examTypeName *string
		if err := rows.Scan(&m.ID, &m.SubjectID, &m.Grade, &m.ExamTypeID, &m.Title, &m.Content,
			&m.FileName, &m.CreatedBy, &m.CreatedAt, &examTypeName); err != nil {
			return nil, err
		}
		if examTypeName != nil {
			m.ExamTypes = &ExamTypeNameOnly{Name: *examTypeName}
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) GetMaterial(ctx context.Context, id string) (*Material, error) {
	row := s.Pool.QueryRow(ctx, `select `+materialColumns+` from materials where id = $1`, id)
	return scanMaterial(row)
}

func (s *Store) GetMaterialWithExamType(ctx context.Context, id string) (*Material, error) {
	sqlq := `
		select m.id, m.subject_id, m.grade, m.exam_type_id, m.title, m.content, m.file_name,
		       m.created_by, m.created_at, et.name
		from materials m
		left join exam_types et on et.id = m.exam_type_id
		where m.id = $1`
	var m Material
	var examTypeName *string
	err := s.Pool.QueryRow(ctx, sqlq, id).Scan(&m.ID, &m.SubjectID, &m.Grade, &m.ExamTypeID, &m.Title,
		&m.Content, &m.FileName, &m.CreatedBy, &m.CreatedAt, &examTypeName)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if examTypeName != nil {
		m.ExamTypes = &ExamTypeNameOnly{Name: *examTypeName}
	}
	return &m, nil
}

type MaterialInput struct {
	SubjectID  string
	Grade      int
	ExamTypeID string
	Title      string
	Content    string
	FileName   *string
	CreatedBy  string
}

func (s *Store) CreateMaterial(ctx context.Context, in MaterialInput) (*Material, error) {
	row := s.Pool.QueryRow(ctx,
		`insert into materials (subject_id, grade, exam_type_id, title, content, file_name, created_by)
		 values ($1, $2, $3, $4, $5, $6, $7)
		 returning `+materialColumns,
		in.SubjectID, in.Grade, in.ExamTypeID, in.Title, in.Content, in.FileName, in.CreatedBy,
	)
	return scanMaterial(row)
}

// MaterialUpdate holds only the columns to change; nil fields are left
// untouched (mirrors the Python router's manual "if body.x is not None").
type MaterialUpdate struct {
	SubjectID  *string
	Grade      *int
	ExamTypeID *string
	Title      *string
	Content    *string
}

func (s *Store) UpdateMaterial(ctx context.Context, id string, upd MaterialUpdate) error {
	set := []string{}
	args := []any{}
	next := func(v any) string {
		args = append(args, v)
		return placeholder(len(args))
	}
	if upd.SubjectID != nil {
		set = append(set, "subject_id = "+next(*upd.SubjectID))
	}
	if upd.Grade != nil {
		set = append(set, "grade = "+next(*upd.Grade))
	}
	if upd.ExamTypeID != nil {
		set = append(set, "exam_type_id = "+next(*upd.ExamTypeID))
	}
	if upd.Title != nil {
		set = append(set, "title = "+next(*upd.Title))
	}
	if upd.Content != nil {
		set = append(set, "content = "+next(*upd.Content))
	}
	if len(set) == 0 {
		return nil
	}
	args = append(args, id)
	query := "update materials set " + joinComma(set) + " where id = " + placeholder(len(args))
	_, err := s.Pool.Exec(ctx, query, args...)
	return err
}

func (s *Store) DeleteMaterial(ctx context.Context, id string) error {
	_, err := s.Pool.Exec(ctx, `delete from materials where id = $1`, id)
	return err
}

// MaterialsForGeneration fetches title+content of every material matching a
// subject/grade/exam-type combo, used as AI generation context.
func (s *Store) MaterialsForGeneration(ctx context.Context, subjectID string, grade int, examTypeID string) ([]Material, error) {
	rows, err := s.Pool.Query(ctx,
		`select title, content from materials where subject_id = $1 and grade = $2 and exam_type_id = $3`,
		subjectID, grade, examTypeID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Material{} // never nil: marshals as [] rather than null when empty
	for rows.Next() {
		var m Material
		if err := rows.Scan(&m.Title, &m.Content); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
