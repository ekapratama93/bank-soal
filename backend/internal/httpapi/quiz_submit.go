package httpapi

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"banksoal/internal/llm"
	"banksoal/internal/store"
	"banksoal/internal/textmatch"
)

// maxAnswerKeys/maxAnswerValueLength bound the payload of this anonymous
// endpoint — an exam type's own question count is capped at 50 (see
// exam_types.go), and answer values go raw into the AI grading prompt for
// "isian"/"deskripsi" items, so an unbounded payload could needlessly
// inflate AI cost/latency.
const (
	maxAnswerKeys        = 100
	maxAnswerValueLength = 5000
)

type submitRequest struct {
	Answers map[string]any `json:"answers"`
}

func validateAnswers(answers map[string]any) error {
	if len(answers) > maxAnswerKeys {
		return fmt.Errorf("Jumlah jawaban melebihi batas (%d).", maxAnswerKeys)
	}
	for _, v := range answers {
		if v == nil {
			continue
		}
		switch val := v.(type) {
		case string:
			if len(val) > maxAnswerValueLength {
				return fmt.Errorf("Salah satu jawaban terlalu panjang.")
			}
		case float64, bool:
			// ok
		default:
			return fmt.Errorf("Tipe jawaban tidak valid.")
		}
	}
	return nil
}

type submitResponse struct {
	QuizID      string                    `json:"quiz_id"`
	SubjectID   *string                   `json:"subject_id"`
	Subject     string                    `json:"subject"`
	Grade       int                       `json:"grade"`
	ExamType    string                    `json:"exam_type"`
	Nilai       int                       `json:"nilai"`
	Poin        float64                   `json:"poin"`
	PoinMaks    int                       `json:"poin_maks"`
	PerQuestion []store.PerQuestionResult `json:"per_question"`
	Expired     bool                      `json:"expired"`
}

func (h *Handlers) handleSubmitQuiz(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	clientID := ensureClientID(w, r)
	quizID := r.PathValue("quiz_id")

	quiz, err := h.Store.GetQuiz(ctx, quizID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Terjadi kesalahan pada server.")
		return
	}
	if quiz == nil {
		writeError(w, http.StatusNotFound, "Kuis tidak ditemukan")
		return
	}

	var body submitRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	if err := validateAnswers(body.Answers); err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	examType, err := h.examTypeName(ctx, quiz.ExamTypeID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Terjadi kesalahan pada server.")
		return
	}
	subject, err := h.displaySubject(ctx, quiz.SubjectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Terjadi kesalahan pada server.")
		return
	}

	attempt, err := h.Store.GetAttempt(ctx, quizID, clientID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Terjadi kesalahan pada server.")
		return
	}
	if attempt == nil {
		// Answers submitted without ever opening the quiz: start and
		// immediately expire the attempt.
		now := time.Now().UTC()
		attempt, err = h.getOrCreateAttemptAt(ctx, quiz, clientID, &now)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Terjadi kesalahan pada server.")
			return
		}
	}

	// Idempotent: a package that's already been submitted returns the
	// stored result without re-grading — without this, a client retry
	// (connection dropped while the response was in flight) would call the
	// AI again and could overwrite an existing score.
	if attempt.SubmittedAt != nil && attempt.Score != nil {
		writeJSON(w, http.StatusOK, submitResponse{
			QuizID: quizID, SubjectID: quiz.SubjectID, Subject: subject, Grade: quiz.Grade,
			ExamType: examType, Nilai: attempt.Score.Nilai, Poin: attempt.Score.Poin,
			PoinMaks: attempt.Score.PoinMaks, PerQuestion: attempt.Score.PerQuestion,
			Expired: attempt.Expired,
		})
		return
	}

	now := time.Now().UTC()
	expired := attempt.ExpiresAt != nil && attempt.ExpiresAt.Before(now)

	examTypeRow, err := h.Store.GetExamType(ctx, quiz.ExamTypeID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Terjadi kesalahan pada server.")
		return
	}

	// Both "isian" and "deskripsi" are graded by AI, but kept as separate
	// batches/calls so they fail independently: "deskripsi" has no good
	// local fallback and is rare (never auto-composed, only admin-custom),
	// so it still hard-fails the submission on AI error. "isian" appears in
	// nearly every quiz, so it falls back to the local matcher (see
	// internal/textmatch) on AI error instead of failing the submission.
	var isianItems, deskripsiItems []llm.ShortAnswerItem
	for i, q := range quiz.Questions {
		if q.Tipe != "isian" && q.Tipe != "deskripsi" {
			continue
		}
		jawabanSiswa := strings.TrimSpace(answerString(body.Answers[strconv.Itoa(i)]))
		item := llm.ShortAnswerItem{
			Index: i, Tipe: q.Tipe, Pertanyaan: q.Pertanyaan,
			JawabanModel: q.JawabanString(), JawabanSiswa: jawabanSiswa,
		}
		if q.Tipe == "isian" {
			isianItems = append(isianItems, item)
		} else {
			deskripsiItems = append(deskripsiItems, item)
		}
	}
	deskripsiResults := map[int]llm.ShortAnswerResult{}
	if len(deskripsiItems) > 0 {
		// Skip the AI call entirely when there's no "deskripsi" question —
		// not just for speed, but so a package with none never fails (502)
		// over an AI hiccup it never actually needed.
		deskripsiResults, err = h.LLM.GradeShortAnswers(ctx, deskripsiItems)
		if err != nil {
			writeError(w, http.StatusBadGateway, err.Error())
			return
		}
	}
	// AI grading failure for "isian" is NOT fatal — isianResults is simply
	// left empty on error, and the per-question loop below falls back to
	// textmatch.GradeIsian for any index missing from it.
	var isianResults map[int]llm.ShortAnswerResult
	if len(isianItems) > 0 {
		isianResults, _ = h.LLM.GradeShortAnswers(ctx, isianItems)
	}

	poinCfg := map[string]int{}
	if examTypeRow != nil {
		poinCfg = examTypeRow.PoinPerTipe
	}
	poinFor := func(tipe string) int {
		if p, ok := poinCfg[tipe]; ok && p > 0 {
			return p
		}
		return 1
	}

	var perQuestion []store.PerQuestionResult
	totalPoinDapat := 0.0
	totalPoinMaks := 0

	for i, q := range quiz.Questions {
		jawaban := body.Answers[strconv.Itoa(i)]
		var verdict string
		var skor float64
		var umpanBalik string
		var jawabanBenar any
		var jawabanSiswa string
		var opsi []string

		switch q.Tipe {
		case "pilihan_ganda":
			idx, isIdx := answerIndex(jawaban)
			benar := isIdx && idx == q.JawabanIndex()
			verdict, skor = verdictAndScore(benar)
			umpanBalik = feedbackFor(benar)
			jawabanBenar = q.Opsi[q.JawabanIndex()]
			jawabanSiswa = "-"
			if isIdx && idx >= 0 && idx <= 3 {
				jawabanSiswa = q.Opsi[idx]
			}
			opsi = q.Opsi
		case "benar_salah":
			s := answerString(jawaban)
			benar := s == q.JawabanString()
			verdict, skor = verdictAndScore(benar)
			umpanBalik = feedbackFor(benar)
			jawabanBenar = q.JawabanString()
			jawabanSiswa = "-"
			if s == "benar" || s == "salah" {
				jawabanSiswa = s
			}
		case "isian":
			s := strings.TrimSpace(answerString(jawaban))
			if hasil, ok := isianResults[i]; ok {
				// Graded by AI (see isianItems above).
				verdict, skor = hasil.Verdict, hasil.Skor
				umpanBalik = hasil.UmpanBalik
				if umpanBalik == "" {
					umpanBalik = feedbackFor(verdict == "benar")
				}
			} else {
				// AI grading unavailable for this item (call failed, or
				// never attempted) — fall back to the local matcher so the
				// submission still succeeds.
				verdict, skor = textmatch.GradeIsian(s, q.JawabanString())
				umpanBalik = feedbackFor(verdict == "benar")
			}
			jawabanBenar = q.JawabanString()
			jawabanSiswa = s
			if jawabanSiswa == "" {
				jawabanSiswa = "-"
			}
		default: // "deskripsi" — graded by AI (see deskripsiItems above)
			if hasil, ok := deskripsiResults[i]; ok {
				verdict, skor = hasil.Verdict, hasil.Skor
				umpanBalik = hasil.UmpanBalik
				if umpanBalik == "" {
					umpanBalik = "-"
				}
			} else {
				verdict, skor, umpanBalik = "salah", 0.0, "-"
			}
			jawabanBenar = q.JawabanString()
			jawabanSiswa = "-"
			if s := answerString(jawaban); s != "" {
				jawabanSiswa = s
			}
		}

		poinTipe := poinFor(q.Tipe)
		poinDapat := round2(skor * float64(poinTipe))
		totalPoinDapat += poinDapat
		totalPoinMaks += poinTipe

		perQuestion = append(perQuestion, store.PerQuestionResult{
			Nomor: i + 1, Tipe: q.Tipe, Pertanyaan: q.Pertanyaan,
			JawabanSiswa: jawabanSiswa, JawabanBenar: jawabanBenar,
			Verdict: verdict, Skor: skor, Poin: poinDapat, PoinMaks: poinTipe,
			UmpanBalik: umpanBalik, Pembahasan: q.Pembahasan,
			Gambar: nilIfEmpty(q.Gambar), Opsi: opsi,
		})
	}

	nilai := 0
	if totalPoinMaks > 0 {
		nilai = int(round0(totalPoinDapat / float64(totalPoinMaks) * 100))
	}
	score := store.Score{
		Nilai: nilai, Poin: round2(totalPoinDapat), PoinMaks: totalPoinMaks, PerQuestion: perQuestion,
	}

	if err := h.Store.SubmitAttempt(ctx, attempt.ID, body.Answers, score, expired, now); err != nil {
		writeError(w, http.StatusInternalServerError, "Terjadi kesalahan pada server.")
		return
	}

	writeJSON(w, http.StatusOK, submitResponse{
		QuizID: quizID, SubjectID: quiz.SubjectID, Subject: subject, Grade: quiz.Grade,
		ExamType: examType, Nilai: nilai, Poin: score.Poin, PoinMaks: totalPoinMaks,
		PerQuestion: perQuestion, Expired: expired,
	})
}

func answerString(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", val)
	}
}

func answerIndex(v any) (int, bool) {
	f, ok := v.(float64)
	if !ok {
		return 0, false
	}
	return int(f), true
}

func verdictAndScore(benar bool) (string, float64) {
	if benar {
		return "benar", 1.0
	}
	return "salah", 0.0
}

func feedbackFor(benar bool) string {
	if benar {
		return "Jawaban benar."
	}
	return "Jawaban tidak tepat."
}

func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}

func round0(v float64) float64 {
	return float64(int(v + 0.5))
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
