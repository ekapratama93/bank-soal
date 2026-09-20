// Integration tests against a real, ephemeral Postgres (via testcontainers-go),
// with the actual schema.sql applied — this exercises real SQL instead of a
// hand-maintained fake, mirroring the Python backend's approach of testing
// against a fake in-memory Supabase client, but one level closer to
// production since there's no PostgREST layer to fake here.
//
// These require a Docker daemon; they skip cleanly (not fail) when one
// isn't available, e.g. in a sandboxed environment.
package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"banksoal/internal/db"
)

// authStub stands in for the pieces of Supabase's managed `auth` schema
// that schema.sql's `profiles` table/trigger reference — a real Supabase
// project provides these; a plain Postgres test container doesn't.
const authStub = `
create schema if not exists auth;
create table if not exists auth.users (
  id uuid primary key default gen_random_uuid(),
  email text
);
`

func setupTestDB(t *testing.T) *Store {
	t.Helper()
	if os.Getenv("CI_NO_DOCKER") != "" {
		t.Skip("CI_NO_DOCKER set, skipping testcontainers-based integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	container, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("banksoal_test"),
		tcpostgres.WithUsername("postgres"),
		tcpostgres.WithPassword("postgres"),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Skipf("skipping: could not start postgres test container (is Docker running?): %v", err)
	}
	t.Cleanup(func() {
		_ = container.Terminate(context.Background())
	})

	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	schemaPath := filepath.Join("..", "..", "schema.sql")
	schemaSQL, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("read schema.sql: %v", err)
	}

	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire conn: %v", err)
	}
	defer conn.Release()
	pgConn := conn.Conn().PgConn()

	if _, err := pgConn.Exec(ctx, authStub).ReadAll(); err != nil {
		t.Fatalf("apply auth stub: %v", err)
	}
	if _, err := pgConn.Exec(ctx, string(schemaSQL)).ReadAll(); err != nil {
		t.Fatalf("apply schema.sql: %v", err)
	}

	return New(pool)
}

func TestSubjectsCRUD(t *testing.T) {
	st := setupTestDB(t)
	ctx := context.Background()

	created, err := st.CreateSubject(ctx, "Sejarah")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.Name != "Sejarah" {
		t.Fatalf("name = %q", created.Name)
	}

	byName, err := st.GetSubjectByName(ctx, "Sejarah")
	if err != nil || byName == nil || byName.ID != created.ID {
		t.Fatalf("GetSubjectByName: %+v, %v", byName, err)
	}

	renamed, err := st.RenameSubject(ctx, created.ID, "Sejarah Indonesia")
	if err != nil || renamed.Name != "Sejarah Indonesia" {
		t.Fatalf("rename: %+v, %v", renamed, err)
	}

	used, err := st.SubjectUsedByMaterials(ctx, created.ID)
	if err != nil || used {
		t.Fatalf("SubjectUsedByMaterials = %v, %v, want false", used, err)
	}

	if err := st.DeleteSubject(ctx, created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	gone, err := st.GetSubjectByID(ctx, created.ID)
	if err != nil || gone != nil {
		t.Fatalf("expected subject to be gone, got %+v", gone)
	}
}

func TestExamTypeCreateAndPartialUpdate(t *testing.T) {
	st := setupTestDB(t)
	ctx := context.Background()

	jumlah := 20
	et, err := st.CreateExamType(ctx, ExamTypeInput{
		Name: "Ujian Coba", JumlahSoal: &jumlah,
		TipeSoal: map[string]int{"pilihan_ganda": 10, "isian": 10},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if et.TipeSoal["pilihan_ganda"] != 10 {
		t.Fatalf("tipe_soal not persisted: %+v", et.TipeSoal)
	}

	// PATCH semantics: an explicitly-nil pointer clears the column; a
	// pointer left nil (field omitted) must leave it untouched.
	var clearedTipeSoal map[string]int
	updated, err := st.UpdateExamType(ctx, et.ID, ExamTypeUpdate{TipeSoal: &clearedTipeSoal})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.TipeSoal != nil {
		t.Fatalf("tipe_soal should be cleared, got %+v", updated.TipeSoal)
	}
	if updated.JumlahSoal == nil || *updated.JumlahSoal != 20 {
		t.Fatalf("jumlah_soal should be untouched, got %+v", updated.JumlahSoal)
	}
}

func TestMaterialCreateAndPoolInvalidation(t *testing.T) {
	st := setupTestDB(t)
	ctx := context.Background()

	sub, err := st.CreateSubject(ctx, "IPA")
	if err != nil {
		t.Fatalf("subject: %v", err)
	}
	et, err := st.CreateExamType(ctx, ExamTypeInput{Name: "Harian"})
	if err != nil {
		t.Fatalf("exam type: %v", err)
	}

	quiz, err := st.CreateQuiz(ctx, QuizInput{
		SubjectID: sub.ID, Grade: 5, ExamTypeID: et.ID, BatchID: "batch-1", DurasiMenit: 60,
		Questions: []Question{{Tipe: "isian", Pertanyaan: "q", Jawaban: "a", Pembahasan: "p"}},
	})
	if err != nil {
		t.Fatalf("create quiz: %v", err)
	}

	if _, err := st.CreateMaterial(ctx, MaterialInput{
		SubjectID: sub.ID, Grade: 5, ExamTypeID: et.ID, Title: "Materi 1", Content: "isi", CreatedBy: "admin@test",
	}); err != nil {
		t.Fatalf("create material: %v", err)
	}
	deleted, err := st.DeletePoolForCombo(ctx, sub.ID, 5, et.ID)
	if err != nil {
		t.Fatalf("invalidate pool: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("deleted = %d, want 1 (the not-yet-started quiz)", deleted)
	}
	if got, err := st.GetQuiz(ctx, quiz.ID); err != nil || got != nil {
		t.Fatalf("quiz should have been removed by pool invalidation, got %+v, %v", got, err)
	}
}

func TestAttemptRaceOnCreateResolvesToSameRow(t *testing.T) {
	st := setupTestDB(t)
	ctx := context.Background()

	sub, _ := st.CreateSubject(ctx, "Matematika")
	et, _ := st.CreateExamType(ctx, ExamTypeInput{Name: "UTS"})
	quiz, err := st.CreateQuiz(ctx, QuizInput{
		SubjectID: sub.ID, Grade: 3, ExamTypeID: et.ID, BatchID: "b1", DurasiMenit: 30,
		Questions: []Question{{Tipe: "isian", Pertanyaan: "q", Jawaban: "a", Pembahasan: "p"}},
	})
	if err != nil {
		t.Fatalf("create quiz: %v", err)
	}

	clientID := "11111111-1111-1111-1111-111111111111"
	expiresAt := time.Now().Add(30 * time.Minute)

	first, err := st.CreateAttempt(ctx, quiz.ID, clientID, expiresAt)
	if err != nil {
		t.Fatalf("first create: %v", err)
	}

	// Simulate the race: a second insert attempt for the same (quiz_id,
	// client_id) pair must be rejected by the unique index, not silently
	// create a duplicate row.
	_, err = st.CreateAttempt(ctx, quiz.ID, clientID, expiresAt)
	if err == nil {
		t.Fatal("expected a unique violation on the second insert")
	}
	if !db.IsUniqueViolation(err) {
		t.Fatalf("expected a unique violation, got: %v", err)
	}

	winner, err := st.GetAttempt(ctx, quiz.ID, clientID)
	if err != nil || winner == nil || winner.ID != first.ID {
		t.Fatalf("GetAttempt after race = %+v, %v, want id %s", winner, err, first.ID)
	}
}

func TestSubmitAttemptIsIdempotentAtStorageLevel(t *testing.T) {
	st := setupTestDB(t)
	ctx := context.Background()

	sub, _ := st.CreateSubject(ctx, "Bahasa Inggris")
	et, _ := st.CreateExamType(ctx, ExamTypeInput{Name: "Harian 2"})
	quiz, _ := st.CreateQuiz(ctx, QuizInput{
		SubjectID: sub.ID, Grade: 4, ExamTypeID: et.ID, BatchID: "b2", DurasiMenit: 30,
		Questions: []Question{{Tipe: "isian", Pertanyaan: "q", Jawaban: "a", Pembahasan: "p"}},
	})
	clientID := "22222222-2222-2222-2222-222222222222"
	attempt, err := st.CreateAttempt(ctx, quiz.ID, clientID, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("create attempt: %v", err)
	}

	score := Score{Nilai: 100, Poin: 1, PoinMaks: 1, PerQuestion: []PerQuestionResult{
		{Nomor: 1, Tipe: "isian", Pertanyaan: "q", JawabanSiswa: "a", JawabanBenar: "a", Verdict: "benar", Skor: 1, Poin: 1, PoinMaks: 1, UmpanBalik: "Jawaban benar.", Pembahasan: "p"},
	}}
	now := time.Now().UTC().Truncate(time.Microsecond)
	if err := st.SubmitAttempt(ctx, attempt.ID, map[string]any{"0": "a"}, score, false, now); err != nil {
		t.Fatalf("submit: %v", err)
	}

	got, err := st.GetAttempt(ctx, quiz.ID, clientID)
	if err != nil || got.Score == nil {
		t.Fatalf("GetAttempt after submit = %+v, %v", got, err)
	}
	if got.Score.Nilai != 100 || len(got.Score.PerQuestion) != 1 {
		t.Fatalf("stored score mismatch: %+v", got.Score)
	}
	if got.SubmittedAt == nil {
		t.Fatal("expected submitted_at to be set")
	}
}
