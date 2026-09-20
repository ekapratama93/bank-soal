package httpapi

import (
	"math/rand"
	"net/http"

	"banksoal/internal/subjectsdata"
)

type quizRequestBody struct {
	SubjectID  string   `json:"subject_id"`
	Subject    string   `json:"subject"`
	Grade      int      `json:"grade"`
	ExamTypeID string   `json:"exam_type_id"`
	ServedIDs  []string `json:"served_ids"`
}

func (h *Handlers) handleRequestQuiz(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	clientID := ensureClientID(w, r)
	var body quizRequestBody
	if !decodeJSON(w, r, &body) {
		return
	}
	if !subjectsdata.ValidGrade(body.Grade) {
		writeError(w, http.StatusUnprocessableEntity, "Kelas tidak valid")
		return
	}
	sub, errMsg, status := h.resolveSubject(ctx, body.SubjectID, body.Subject)
	if errMsg != "" {
		writeError(w, status, errMsg)
		return
	}
	examType, err := h.Store.GetExamType(ctx, body.ExamTypeID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Terjadi kesalahan pada server.")
		return
	}
	if examType == nil {
		writeError(w, http.StatusUnprocessableEntity, "Tipe ujian tidak valid")
		return
	}

	pool, err := h.Store.PoolForCombo(ctx, sub.ID, body.Grade, body.ExamTypeID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Terjadi kesalahan pada server.")
		return
	}
	if len(pool) == 0 {
		writeError(w, http.StatusNotFound, "Belum ada paket soal tersedia untuk kombinasi ini. Hubungi guru/admin.")
		return
	}

	served := map[string]bool{}
	for _, id := range body.ServedIDs {
		served[id] = true
	}
	ownAttemptIDs, err := h.Store.AttemptQuizIDsForClient(ctx, clientID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Terjadi kesalahan pada server.")
		return
	}
	taken := map[string]bool{}
	for k := range served {
		taken[k] = true
	}
	for _, id := range ownAttemptIDs {
		taken[id] = true
	}

	// Packages already opened by OTHER students may still be handed out —
	// only packages THIS student has already opened are excluded.
	var available []int
	for i, q := range pool {
		if !taken[q.ID] {
			available = append(available, i)
		}
	}
	repeat := len(available) == 0
	var chosenIdx int
	if repeat {
		chosenIdx = rand.Intn(len(pool))
	} else {
		chosenIdx = available[rand.Intn(len(available))]
	}
	chosen := pool[chosenIdx]

	attempt, err := h.getOrCreateAttempt(ctx, &chosen, clientID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Terjadi kesalahan pada server.")
		return
	}
	public, err := h.quizPublic(ctx, &chosen, attempt)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Terjadi kesalahan pada server.")
		return
	}
	repeatVal := repeat
	public.Repeat = &repeatVal
	writeJSON(w, http.StatusOK, public)
}

type availableCombo struct {
	SubjectID  *string `json:"subject_id"`
	Subject    string  `json:"subject"`
	Grade      int     `json:"grade"`
	ExamTypeID string  `json:"exam_type_id"`
	ExamType   string  `json:"exam_type"`
	Unstarted  int     `json:"unstarted"`
	Total      int     `json:"total"`
}

func (h *Handlers) handleAvailable(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rows, err := h.Store.ListQuizComboRows(ctx)
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

	type key struct {
		subjectID  string
		grade      int
		examTypeID string
	}
	agg := map[key]*availableCombo{}
	var order []key
	for _, row := range rows {
		sid := ""
		if row.SubjectID != nil {
			sid = *row.SubjectID
		}
		k := key{sid, row.Grade, row.ExamTypeID}
		entry, ok := agg[k]
		if !ok {
			var subjectIDPtr *string
			if row.SubjectID != nil {
				v := *row.SubjectID
				subjectIDPtr = &v
			}
			entry = &availableCombo{
				SubjectID:  subjectIDPtr,
				Subject:    subNames[sid],
				Grade:      row.Grade,
				ExamTypeID: row.ExamTypeID,
				ExamType:   examNames[row.ExamTypeID],
			}
			agg[k] = entry
			order = append(order, k)
		}
		entry.Total++
		if !row.Started {
			entry.Unstarted++
		}
	}

	out := make([]availableCombo, 0, len(order))
	for _, k := range order {
		out = append(out, *agg[k])
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handlers) handleGetQuiz(w http.ResponseWriter, r *http.Request) {
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
	attempt, err := h.getOrCreateAttempt(ctx, quiz, clientID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Terjadi kesalahan pada server.")
		return
	}
	public, err := h.quizPublic(ctx, quiz, attempt)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Terjadi kesalahan pada server.")
		return
	}
	writeJSON(w, http.StatusOK, public)
}

type attemptResult struct {
	QuizID      string  `json:"quiz_id"`
	SubjectID   *string `json:"subject_id"`
	Subject     string  `json:"subject"`
	Grade       int     `json:"grade"`
	ExamType    string  `json:"exam_type"`
	Nilai       int     `json:"nilai"`
	Poin        float64 `json:"poin"`
	PoinMaks    int     `json:"poin_maks"`
	PerQuestion any     `json:"per_question"`
	Expired     bool    `json:"expired"`
	SubmittedAt any     `json:"submitted_at"`
}

func (h *Handlers) handleListAttempts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	clientID := ensureClientID(w, r)
	attempts, err := h.Store.SubmittedAttemptsForClient(ctx, clientID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Terjadi kesalahan pada server.")
		return
	}
	quizMeta, err := h.Store.QuizzesMeta(ctx)
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

	out := make([]attemptResult, 0, len(attempts))
	for _, a := range attempts {
		quiz := quizMeta[a.QuizID]
		nilai, poin, poinMaks := 0, 0.0, 0
		var perQuestion any = []any{}
		if a.Score != nil {
			nilai = a.Score.Nilai
			poin = a.Score.Poin
			poinMaks = a.Score.PoinMaks
			perQuestion = a.Score.PerQuestion
		}
		out = append(out, attemptResult{
			QuizID:      a.QuizID,
			SubjectID:   quiz.SubjectID,
			Subject:     displaySubjectName(quiz.SubjectID, subNames),
			Grade:       quiz.Grade,
			ExamType:    examNames[quiz.ExamTypeID],
			Nilai:       nilai,
			Poin:        poin,
			PoinMaks:    poinMaks,
			PerQuestion: perQuestion,
			Expired:     a.Expired,
			SubmittedAt: a.SubmittedAt,
		})
	}
	writeJSON(w, http.StatusOK, out)
}
