package httpapi

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"banksoal/internal/store"
	"banksoal/internal/subjectsdata"
)

func sprintfSoal(n int, format string, args ...any) string {
	return fmt.Sprintf("Soal %d: "+format, append([]any{n}, args...)...)
}

type quizPackageListItem struct {
	ID          string     `json:"id"`
	SubjectID   *string    `json:"subject_id"`
	Subject     string     `json:"subject"`
	Grade       int        `json:"grade"`
	ExamTypeID  string     `json:"exam_type_id"`
	ExamType    string     `json:"exam_type"`
	JumlahSoal  int        `json:"jumlah_soal"`
	Started     bool       `json:"started"`
	DurasiMenit *int       `json:"durasi_menit"`
	BatchID     *string    `json:"batch_id"`
	CreatedAt   *time.Time `json:"created_at"`
}

func (h *Handlers) handleAdminListQuizzes(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	ctx := r.Context()
	var filter store.AdminQuizFilter
	if v := r.URL.Query().Get("subject_id"); v != "" {
		filter.SubjectID = &v
	} else if v := r.URL.Query().Get("subject"); v != "" {
		sub, errMsg, status := h.resolveSubject(ctx, "", v)
		if errMsg != "" {
			writeError(w, status, errMsg)
			return
		}
		filter.SubjectID = &sub.ID
	}
	if grade, ok := parseIntQuery(r, "grade"); ok {
		filter.Grade = grade
	} else {
		writeError(w, http.StatusUnprocessableEntity, "Parameter grade tidak valid")
		return
	}
	if v := r.URL.Query().Get("exam_type_id"); v != "" {
		filter.ExamTypeID = &v
	}

	quizzes, err := h.Store.ListQuizzesAdmin(ctx, filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Terjadi kesalahan pada server.")
		return
	}
	examNames, err := h.Store.ExamTypeNamesMap(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Terjadi kesalahan pada server.")
		return
	}
	subNames, err := h.Store.SubjectNamesMap(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Terjadi kesalahan pada server.")
		return
	}

	out := make([]quizPackageListItem, len(quizzes))
	for i, q := range quizzes {
		createdAt := q.CreatedAt
		out[i] = quizPackageListItem{
			ID:          q.ID,
			SubjectID:   q.SubjectID,
			Subject:     displaySubjectName(q.SubjectID, subNames),
			Grade:       q.Grade,
			ExamTypeID:  q.ExamTypeID,
			ExamType:    examNames[q.ExamTypeID],
			JumlahSoal:  len(q.Questions),
			Started:     q.Started,
			DurasiMenit: q.DurasiMenit,
			BatchID:     q.BatchID,
			CreatedAt:   &createdAt,
		}
	}
	writeJSON(w, http.StatusOK, out)
}

type adminQuestion struct {
	Nomor      int      `json:"nomor"`
	Tipe       string   `json:"tipe"`
	Pertanyaan string   `json:"pertanyaan"`
	Opsi       []string `json:"opsi,omitempty"`
	Jawaban    any      `json:"jawaban"`
	Pembahasan string   `json:"pembahasan"`
	Gambar     string   `json:"gambar,omitempty"`
}

type quizPackageDetail struct {
	quizPackageListItem
	Questions []adminQuestion `json:"questions"`
}

func (h *Handlers) adminQuizPayload(quiz *store.Quiz, examType string, subject string) quizPackageDetail {
	questions := make([]adminQuestion, len(quiz.Questions))
	for i, q := range quiz.Questions {
		item := adminQuestion{
			Nomor: i + 1, Tipe: q.Tipe, Pertanyaan: q.Pertanyaan,
			Jawaban: q.Jawaban, Pembahasan: q.Pembahasan,
		}
		if q.Tipe == "pilihan_ganda" {
			item.Opsi = q.Opsi
		}
		if q.Gambar != "" {
			item.Gambar = q.Gambar
		}
		questions[i] = item
	}
	createdAt := quiz.CreatedAt
	return quizPackageDetail{
		quizPackageListItem: quizPackageListItem{
			ID: quiz.ID, SubjectID: quiz.SubjectID, Subject: subject, Grade: quiz.Grade,
			ExamTypeID: quiz.ExamTypeID, ExamType: examType, JumlahSoal: len(quiz.Questions),
			Started: quiz.Started, DurasiMenit: quiz.DurasiMenit, BatchID: quiz.BatchID,
			CreatedAt: &createdAt,
		},
		Questions: questions,
	}
}

func (h *Handlers) handleAdminGetQuiz(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	ctx := r.Context()
	quiz, err := h.Store.GetQuiz(ctx, r.PathValue("quiz_id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Terjadi kesalahan pada server.")
		return
	}
	if quiz == nil {
		writeError(w, http.StatusNotFound, "Paket soal tidak ditemukan")
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
	writeJSON(w, http.StatusOK, h.adminQuizPayload(quiz, examType, subject))
}

var validTipeSoalTypes = map[string]bool{"pilihan_ganda": true, "benar_salah": true, "isian": true, "deskripsi": true}

type adminQuestionInput struct {
	Tipe       string   `json:"tipe"`
	Pertanyaan string   `json:"pertanyaan"`
	Opsi       []string `json:"opsi"`
	Jawaban    any      `json:"jawaban"`
	Pembahasan string   `json:"pembahasan"`
	Gambar     string   `json:"gambar"`
}

type quizUpdateRequest struct {
	Questions []adminQuestionInput `json:"questions"`
}

func validateAdminQuestions(items []adminQuestionInput) ([]store.Question, string) {
	if len(items) == 0 {
		return nil, "Paket soal harus berisi minimal 1 soal"
	}
	cleaned := make([]store.Question, 0, len(items))
	for i, item := range items {
		n := i + 1
		if !validTipeSoalTypes[item.Tipe] {
			return nil, sprintfSoal(n, "tipe tidak valid: %s", item.Tipe)
		}
		pertanyaan := strings.TrimSpace(item.Pertanyaan)
		pembahasan := strings.TrimSpace(item.Pembahasan)
		if pertanyaan == "" || pembahasan == "" {
			return nil, sprintfSoal(n, "pertanyaan/pembahasan kosong")
		}
		q := store.Question{Tipe: item.Tipe, Pertanyaan: pertanyaan, Pembahasan: pembahasan}
		switch item.Tipe {
		case "pilihan_ganda":
			if len(item.Opsi) != 4 {
				return nil, sprintfSoal(n, "opsi harus 4 item dan tidak kosong")
			}
			for _, o := range item.Opsi {
				if strings.TrimSpace(o) == "" {
					return nil, sprintfSoal(n, "opsi harus 4 item dan tidak kosong")
				}
			}
			idxFloat, isNum := item.Jawaban.(float64)
			if !isNum || idxFloat != float64(int(idxFloat)) || idxFloat < 0 || idxFloat > 3 {
				return nil, sprintfSoal(n, "jawaban pilihan ganda harus indeks 0-3")
			}
			q.Opsi = item.Opsi
			q.Jawaban = int(idxFloat)
		case "benar_salah":
			s, _ := item.Jawaban.(string)
			if s != "benar" && s != "salah" {
				return nil, sprintfSoal(n, "jawaban harus 'benar' atau 'salah'")
			}
			q.Jawaban = s
		default:
			s, _ := item.Jawaban.(string)
			if strings.TrimSpace(s) == "" {
				return nil, sprintfSoal(n, "jawaban %s kosong", item.Tipe)
			}
			q.Jawaban = strings.TrimSpace(s)
		}
		if item.Gambar != "" {
			q.Gambar = item.Gambar
		}
		cleaned = append(cleaned, q)
	}
	return cleaned, ""
}

func (h *Handlers) handleAdminUpdateQuiz(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	ctx := r.Context()
	quizID := r.PathValue("quiz_id")
	quiz, err := h.Store.GetQuiz(ctx, quizID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Terjadi kesalahan pada server.")
		return
	}
	if quiz == nil {
		writeError(w, http.StatusNotFound, "Paket soal tidak ditemukan")
		return
	}
	var body quizUpdateRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	questions, errMsg := validateAdminQuestions(body.Questions)
	if errMsg != "" {
		writeError(w, http.StatusUnprocessableEntity, errMsg)
		return
	}
	if err := h.Store.UpdateQuizQuestions(ctx, quizID, questions); err != nil {
		writeError(w, http.StatusInternalServerError, "Terjadi kesalahan pada server.")
		return
	}
	quiz.Questions = questions
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
	writeJSON(w, http.StatusOK, h.adminQuizPayload(quiz, examType, subject))
}

func (h *Handlers) handleAdminDeleteQuiz(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	ctx := r.Context()
	quizID := r.PathValue("quiz_id")
	quiz, err := h.Store.GetQuiz(ctx, quizID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Terjadi kesalahan pada server.")
		return
	}
	if quiz == nil {
		writeError(w, http.StatusNotFound, "Paket soal tidak ditemukan")
		return
	}
	// Attempt history for this package cascades away via the
	// attempts.quiz_id foreign key (see schema.sql) — a single delete call,
	// so there's no window where the quiz is gone but its attempts remain.
	if err := h.Store.DeleteQuiz(ctx, quizID); err != nil {
		writeError(w, http.StatusInternalServerError, "Terjadi kesalahan pada server.")
		return
	}
	writeNoContent(w)
}

type bulkDeleteRequest struct {
	IDs []string `json:"ids"`
}

func (h *Handlers) handleAdminBulkDeleteQuizzes(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	var body bulkDeleteRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	if len(body.IDs) == 0 {
		writeError(w, http.StatusUnprocessableEntity, "Daftar id paket soal kosong")
		return
	}
	seen := map[string]bool{}
	ids := make([]string, 0, len(body.IDs))
	for _, id := range body.IDs {
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	deleted, err := h.Store.BulkDeleteQuizzes(r.Context(), ids)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Terjadi kesalahan pada server.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"deleted": deleted})
}

type poolResetRequest struct {
	SubjectID  string `json:"subject_id"`
	Subject    string `json:"subject"`
	Grade      int    `json:"grade"`
	ExamTypeID string `json:"exam_type_id"`
}

func (h *Handlers) handlePoolReset(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	var body poolResetRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	ctx := r.Context()
	if !subjectsdata.ValidGrade(body.Grade) {
		writeError(w, http.StatusUnprocessableEntity, "Kelas tidak valid")
		return
	}
	sub, errMsg, status := h.resolveSubject(ctx, body.SubjectID, body.Subject)
	if errMsg != "" {
		writeError(w, status, errMsg)
		return
	}
	if errMsg, status := h.validateExamType(ctx, body.ExamTypeID); errMsg != "" {
		writeError(w, status, errMsg)
		return
	}
	deleted, err := h.Store.DeletePoolForCombo(ctx, sub.ID, body.Grade, body.ExamTypeID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Terjadi kesalahan pada server.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"deleted": deleted})
}
