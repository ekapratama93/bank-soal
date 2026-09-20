package httpapi

import (
	"context"
	"time"

	"banksoal/internal/db"
	"banksoal/internal/gradeconfig"
	"banksoal/internal/store"
)

type questionPublic struct {
	Nomor      int      `json:"nomor"`
	Tipe       string   `json:"tipe"`
	Pertanyaan string   `json:"pertanyaan"`
	Opsi       []string `json:"opsi,omitempty"`
	Gambar     string   `json:"gambar,omitempty"`
}

// stripQuestions removes answers/explanations — these never leave the
// backend for a student-facing quiz payload.
func stripQuestions(questions []store.Question) []questionPublic {
	out := make([]questionPublic, len(questions))
	for i, q := range questions {
		item := questionPublic{Nomor: i + 1, Tipe: q.Tipe, Pertanyaan: q.Pertanyaan}
		if q.Tipe == "pilihan_ganda" {
			item.Opsi = q.Opsi
		}
		if q.Gambar != "" {
			item.Gambar = q.Gambar
		}
		out[i] = item
	}
	return out
}

func (h *Handlers) examTypeName(ctx context.Context, examTypeID string) (string, error) {
	et, err := h.Store.GetExamType(ctx, examTypeID)
	if err != nil {
		return "", err
	}
	if et == nil {
		return "", nil
	}
	return et.Name, nil
}

func (h *Handlers) displaySubject(ctx context.Context, subjectID *string) (string, error) {
	if subjectID == nil {
		return "", nil
	}
	names, err := h.Store.SubjectNamesMap(ctx)
	if err != nil {
		return "", err
	}
	return names[*subjectID], nil
}

type quizPublicResponse struct {
	QuizID      string           `json:"quiz_id"`
	Questions   []questionPublic `json:"questions"`
	SubjectID   *string          `json:"subject_id"`
	Subject     string           `json:"subject"`
	Grade       int              `json:"grade"`
	ExamType    string           `json:"exam_type"`
	DurasiMenit *int             `json:"durasi_menit"`
	ExpiresAt   *time.Time       `json:"expires_at"`
	Repeat      *bool            `json:"repeat,omitempty"`
}

func (h *Handlers) quizPublic(ctx context.Context, quiz *store.Quiz, attempt *store.Attempt) (quizPublicResponse, error) {
	examType, err := h.examTypeName(ctx, quiz.ExamTypeID)
	if err != nil {
		return quizPublicResponse{}, err
	}
	subject, err := h.displaySubject(ctx, quiz.SubjectID)
	if err != nil {
		return quizPublicResponse{}, err
	}
	return quizPublicResponse{
		QuizID:      quiz.ID,
		Questions:   stripQuestions(quiz.Questions),
		SubjectID:   quiz.SubjectID,
		Subject:     subject,
		Grade:       quiz.Grade,
		ExamType:    examType,
		DurasiMenit: quiz.DurasiMenit,
		ExpiresAt:   attempt.ExpiresAt,
	}, nil
}

// getOrCreateAttempt starts (or resumes) this client's attempt at a quiz —
// creating one marks the package "started" and begins its timer, but the
// package stays available to every OTHER student (the timer lives on the
// attempt, not on the quiz row).
func (h *Handlers) getOrCreateAttempt(ctx context.Context, quiz *store.Quiz, clientID string) (*store.Attempt, error) {
	return h.getOrCreateAttemptAt(ctx, quiz, clientID, nil)
}

func (h *Handlers) getOrCreateAttemptAt(ctx context.Context, quiz *store.Quiz, clientID string, expiresAtOverride *time.Time) (*store.Attempt, error) {
	existing, err := h.Store.GetAttempt(ctx, quiz.ID, clientID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}
	var expiresAt time.Time
	if expiresAtOverride != nil {
		expiresAt = *expiresAtOverride
	} else {
		durasi := gradeconfig.Get(quiz.Grade).DurasiMenit
		if quiz.DurasiMenit != nil {
			durasi = *quiz.DurasiMenit
		}
		expiresAt = time.Now().UTC().Add(time.Duration(durasi) * time.Minute)
	}
	if !quiz.Started {
		if err := h.Store.MarkQuizStarted(ctx, quiz.ID); err != nil {
			return nil, err
		}
		quiz.Started = true
	}
	attempt, err := h.Store.CreateAttempt(ctx, quiz.ID, clientID, expiresAt)
	if err != nil {
		if db.IsUniqueViolation(err) {
			// Two near-simultaneous requests (double-click, two tabs, a
			// client retry) both missed the same not-yet-existing row and
			// both tried to insert — the DB's unique index rejects the
			// loser; fetch the winner's row instead of a raw 500.
			winner, gerr := h.Store.GetAttempt(ctx, quiz.ID, clientID)
			if gerr != nil {
				return nil, gerr
			}
			if winner != nil {
				return winner, nil
			}
		}
		return nil, err
	}
	return attempt, nil
}
