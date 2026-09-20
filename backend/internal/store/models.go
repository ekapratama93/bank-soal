// Package store is the direct-Postgres data-access layer, replacing the
// Python backend's sb.table(...) calls through Supabase's PostgREST.
package store

import (
	"encoding/json"
	"time"
)

type Subject struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type ExamType struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	JumlahSoal  *int           `json:"jumlah_soal"`
	DurasiMenit *int           `json:"durasi_menit"`
	TipeSoal    map[string]int `json:"tipe_soal"`
	PoinPerTipe map[string]int `json:"poin_per_tipe"`
	CreatedAt   time.Time      `json:"created_at"`
}

type ExamTypeNameOnly struct {
	Name string `json:"name"`
}

type Material struct {
	ID         string            `json:"id"`
	SubjectID  *string           `json:"subject_id"`
	Subject    string            `json:"subject"`
	Grade      int               `json:"grade"`
	ExamTypeID string            `json:"exam_type_id"`
	ExamTypes  *ExamTypeNameOnly `json:"exam_types,omitempty"`
	Title      string            `json:"title"`
	Content    string            `json:"content"`
	FileName   *string           `json:"file_name"`
	FileURL    *string           `json:"file_url"`
	Images     []MaterialImage   `json:"images"`
	CreatedBy  *string           `json:"created_by"`
	CreatedAt  time.Time         `json:"created_at"`
}

// MaterialImage is one image extracted from a material's uploaded file
// (docx media or embedded PDF image), already uploaded to Supabase Storage.
// It's offered to quiz generation both as multimodal LLM context and as a
// gambar_tipe:"material" candidate to attach to a generated question.
type MaterialImage struct {
	URL  string `json:"url"`
	Name string `json:"name"`
}

// Question is one item of a quiz package's `questions` jsonb array.
// Jawaban is an int (0-3) for "pilihan_ganda" and a string for every
// other question type — a custom (Un)MarshalJSON keeps that distinction
// exact instead of letting it collapse to float64 the way a plain `any`
// field would after a JSON round-trip.
type Question struct {
	Tipe       string
	Pertanyaan string
	Opsi       []string
	Jawaban    any
	Pembahasan string
	Gambar     string
}

type questionWire struct {
	Tipe       string          `json:"tipe"`
	Pertanyaan string          `json:"pertanyaan"`
	Opsi       []string        `json:"opsi,omitempty"`
	Jawaban    json.RawMessage `json:"jawaban"`
	Pembahasan string          `json:"pembahasan"`
	Gambar     string          `json:"gambar,omitempty"`
}

func (q *Question) UnmarshalJSON(data []byte) error {
	var raw questionWire
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	q.Tipe = raw.Tipe
	q.Pertanyaan = raw.Pertanyaan
	q.Opsi = raw.Opsi
	q.Pembahasan = raw.Pembahasan
	q.Gambar = raw.Gambar
	if len(raw.Jawaban) == 0 || string(raw.Jawaban) == "null" {
		q.Jawaban = nil
		return nil
	}
	if raw.Tipe == "pilihan_ganda" {
		var idx int
		if err := json.Unmarshal(raw.Jawaban, &idx); err != nil {
			return err
		}
		q.Jawaban = idx
		return nil
	}
	var s string
	if err := json.Unmarshal(raw.Jawaban, &s); err != nil {
		return err
	}
	q.Jawaban = s
	return nil
}

func (q Question) MarshalJSON() ([]byte, error) {
	jawaban, err := json.Marshal(q.Jawaban)
	if err != nil {
		return nil, err
	}
	return json.Marshal(questionWire{
		Tipe:       q.Tipe,
		Pertanyaan: q.Pertanyaan,
		Opsi:       q.Opsi,
		Jawaban:    jawaban,
		Pembahasan: q.Pembahasan,
		Gambar:     q.Gambar,
	})
}

// JawabanString returns Jawaban as a string ("" if it isn't one) — used for
// benar_salah/isian/deskripsi questions.
func (q Question) JawabanString() string {
	s, _ := q.Jawaban.(string)
	return s
}

// JawabanIndex returns Jawaban as an int (-1 if it isn't one) — used for
// pilihan_ganda questions.
func (q Question) JawabanIndex() int {
	if i, ok := q.Jawaban.(int); ok {
		return i
	}
	return -1
}

type Quiz struct {
	ID          string     `json:"id"`
	SubjectID   *string    `json:"subject_id"`
	Grade       int        `json:"grade"`
	Questions   []Question `json:"questions"`
	ExamTypeID  string     `json:"exam_type_id"`
	BatchID     *string    `json:"batch_id"`
	DurasiMenit *int       `json:"durasi_menit"`
	Started     bool       `json:"started"`
	CreatedAt   time.Time  `json:"created_at"`
}

// PerQuestionResult is one line of a submitted attempt's score breakdown.
type PerQuestionResult struct {
	Nomor        int      `json:"nomor"`
	Tipe         string   `json:"tipe"`
	Pertanyaan   string   `json:"pertanyaan"`
	JawabanSiswa string   `json:"jawaban_siswa"`
	JawabanBenar any      `json:"jawaban_benar"`
	Verdict      string   `json:"verdict"`
	Skor         float64  `json:"skor"`
	Poin         float64  `json:"poin"`
	PoinMaks     int      `json:"poin_maks"`
	UmpanBalik   string   `json:"umpan_balik"`
	Pembahasan   string   `json:"pembahasan"`
	Gambar       *string  `json:"gambar,omitempty"`
	Opsi         []string `json:"opsi,omitempty"`
}

type Score struct {
	Nilai       int                 `json:"nilai"`
	Poin        float64             `json:"poin"`
	PoinMaks    int                 `json:"poin_maks"`
	PerQuestion []PerQuestionResult `json:"per_question"`
}

type Attempt struct {
	ID          string         `json:"id"`
	QuizID      string         `json:"quiz_id"`
	ClientID    *string        `json:"client_id"`
	Answers     map[string]any `json:"answers"`
	Score       *Score         `json:"score"`
	Expired     bool           `json:"expired"`
	StartedAt   time.Time      `json:"started_at"`
	ExpiresAt   *time.Time     `json:"expires_at"`
	SubmittedAt *time.Time     `json:"submitted_at"`
}
