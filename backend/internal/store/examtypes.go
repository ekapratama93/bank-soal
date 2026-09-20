package store

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
)

// nullableIntMapJSON marshals a possibly-nil map[string]int for a nullable
// jsonb column — a nil map becomes a real SQL NULL (not the JSON literal
// "null"), matching the Python backend's `data["tipe_soal"] = None`.
func nullableIntMapJSON(m map[string]int) (*string, error) {
	if m == nil {
		return nil, nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	s := string(b)
	return &s, nil
}

func scanIntMapJSON(raw []byte) (map[string]int, error) {
	if raw == nil {
		return nil, nil
	}
	var m map[string]int
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func (s *Store) scanExamType(row pgx.Row) (*ExamType, error) {
	var et ExamType
	var tipeSoal, poinPerTipe []byte
	err := row.Scan(&et.ID, &et.Name, &et.JumlahSoal, &et.DurasiMenit, &tipeSoal, &poinPerTipe, &et.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if et.TipeSoal, err = scanIntMapJSON(tipeSoal); err != nil {
		return nil, err
	}
	if et.PoinPerTipe, err = scanIntMapJSON(poinPerTipe); err != nil {
		return nil, err
	}
	return &et, nil
}

const examTypeColumns = `id, name, jumlah_soal, durasi_menit, tipe_soal, poin_per_tipe, created_at`

func (s *Store) ListExamTypes(ctx context.Context) ([]ExamType, error) {
	rows, err := s.Pool.Query(ctx, `select `+examTypeColumns+` from exam_types order by name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ExamType{} // never nil: marshals as [] rather than null when empty
	for rows.Next() {
		var tipeSoal, poinPerTipe []byte
		var et ExamType
		if err := rows.Scan(&et.ID, &et.Name, &et.JumlahSoal, &et.DurasiMenit, &tipeSoal, &poinPerTipe, &et.CreatedAt); err != nil {
			return nil, err
		}
		if et.TipeSoal, err = scanIntMapJSON(tipeSoal); err != nil {
			return nil, err
		}
		if et.PoinPerTipe, err = scanIntMapJSON(poinPerTipe); err != nil {
			return nil, err
		}
		out = append(out, et)
	}
	return out, rows.Err()
}

func (s *Store) GetExamType(ctx context.Context, id string) (*ExamType, error) {
	row := s.Pool.QueryRow(ctx, `select `+examTypeColumns+` from exam_types where id = $1`, id)
	return s.scanExamType(row)
}

func (s *Store) GetExamTypeByName(ctx context.Context, name string) (*ExamType, error) {
	row := s.Pool.QueryRow(ctx, `select `+examTypeColumns+` from exam_types where name = $1`, name)
	return s.scanExamType(row)
}

type ExamTypeInput struct {
	Name        string
	JumlahSoal  *int
	DurasiMenit *int
	TipeSoal    map[string]int
	PoinPerTipe map[string]int
}

func (s *Store) CreateExamType(ctx context.Context, in ExamTypeInput) (*ExamType, error) {
	tipeSoal, err := nullableIntMapJSON(in.TipeSoal)
	if err != nil {
		return nil, err
	}
	poinPerTipe, err := nullableIntMapJSON(in.PoinPerTipe)
	if err != nil {
		return nil, err
	}
	row := s.Pool.QueryRow(ctx,
		`insert into exam_types (name, jumlah_soal, durasi_menit, tipe_soal, poin_per_tipe)
		 values ($1, $2, $3, $4::jsonb, $5::jsonb)
		 returning `+examTypeColumns,
		in.Name, in.JumlahSoal, in.DurasiMenit, tipeSoal, poinPerTipe,
	)
	return s.scanExamType(row)
}

// ExamTypeUpdate mirrors the Python router's `model_fields_set` handling:
// a field is only touched if its pointer is non-nil, and the *value*
// pointed to may itself be nil to explicitly clear the column.
type ExamTypeUpdate struct {
	Name        *string
	JumlahSoal  **int
	DurasiMenit **int
	TipeSoal    *map[string]int
	PoinPerTipe *map[string]int
}

func (s *Store) UpdateExamType(ctx context.Context, id string, upd ExamTypeUpdate) (*ExamType, error) {
	set := []string{}
	args := []any{}
	next := func(v any) string {
		args = append(args, v)
		return placeholder(len(args))
	}
	if upd.Name != nil {
		set = append(set, "name = "+next(*upd.Name))
	}
	if upd.JumlahSoal != nil {
		set = append(set, "jumlah_soal = "+next(*upd.JumlahSoal))
	}
	if upd.DurasiMenit != nil {
		set = append(set, "durasi_menit = "+next(*upd.DurasiMenit))
	}
	if upd.TipeSoal != nil {
		v, err := nullableIntMapJSON(*upd.TipeSoal)
		if err != nil {
			return nil, err
		}
		set = append(set, "tipe_soal = "+next(v)+"::jsonb")
	}
	if upd.PoinPerTipe != nil {
		v, err := nullableIntMapJSON(*upd.PoinPerTipe)
		if err != nil {
			return nil, err
		}
		set = append(set, "poin_per_tipe = "+next(v)+"::jsonb")
	}
	if len(set) == 0 {
		return s.GetExamType(ctx, id)
	}
	args = append(args, id)
	query := "update exam_types set " + joinComma(set) + " where id = " + placeholder(len(args)) + " returning " + examTypeColumns
	row := s.Pool.QueryRow(ctx, query, args...)
	return s.scanExamType(row)
}

func (s *Store) DeleteExamType(ctx context.Context, id string) error {
	_, err := s.Pool.Exec(ctx, `delete from exam_types where id = $1`, id)
	return err
}

func (s *Store) ExamTypeUsedByMaterials(ctx context.Context, id string) (bool, error) {
	return s.existsWhere(ctx, `select 1 from materials where exam_type_id = $1 limit 1`, id)
}

func (s *Store) ExamTypeUsedByQuizzes(ctx context.Context, id string) (bool, error) {
	return s.existsWhere(ctx, `select 1 from quizzes where exam_type_id = $1 limit 1`, id)
}

func (s *Store) ExamTypeNamesMap(ctx context.Context) (map[string]string, error) {
	rows, err := s.Pool.Query(ctx, `select id, name from exam_types`)
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
