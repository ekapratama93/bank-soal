package httpapi

import (
	"context"
	"log/slog"
	"net/http"

	"banksoal/internal/gradeconfig"
	"banksoal/internal/idgen"
	"banksoal/internal/imagesearch"
	"banksoal/internal/llm"
	"banksoal/internal/store"
	"banksoal/internal/subjectsdata"
)

const imageCapPerPaket = 3

// resolveImages turns gambar_tipe/gambar_prompt/gambar_cari markers from
// the LLM into a real "gambar" URL, up to imageCapPerPaket per package. A
// failure on any single question must not fail the whole batch — that
// question just ends up with no image.
func (h *Handlers) resolveImages(ctx context.Context, questions []llm.RawQuestion) {
	resolved := 0
	for i := range questions {
		q := &questions[i]
		tipe := q.GambarTipe
		if tipe == "" || resolved >= imageCapPerPaket {
			continue
		}
		switch tipe {
		case "generated":
			if q.GambarPrompt == "" {
				continue
			}
			data, contentType, err := h.LLM.GenerateImage(ctx, q.GambarPrompt)
			if err != nil {
				slog.Warn("Gagal membuat gambar soal", "err", err)
				continue
			}
			url, err := h.Auth.UploadImageBytes(ctx, data, contentType)
			if err != nil {
				slog.Warn("Gagal mengunggah gambar soal", "err", err)
				continue
			}
			q.Gambar = url
			resolved++
		case "stock":
			if q.GambarCari == "" {
				continue
			}
			data := imagesearch.SearchStockImage(ctx, q.GambarCari)
			if data == nil {
				continue
			}
			url, err := h.Auth.UploadImageBytes(ctx, data, "image/jpeg")
			if err != nil {
				slog.Warn("Gagal mengunggah gambar soal", "err", err)
				continue
			}
			q.Gambar = url
			resolved++
		}
	}
}

type generateRequest struct {
	SubjectID   string `json:"subject_id"`
	Subject     string `json:"subject"`
	Grade       int    `json:"grade"`
	ExamTypeID  string `json:"exam_type_id"`
	JumlahPaket int    `json:"jumlah_paket"`
}

func (h *Handlers) handleGenerateQuiz(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	var body generateRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.JumlahPaket == 0 {
		body.JumlahPaket = 3
	}
	if body.JumlahPaket < 1 || body.JumlahPaket > 5 {
		writeError(w, http.StatusUnprocessableEntity, "jumlah_paket harus 1-5")
		return
	}
	if !subjectsdata.ValidGrade(body.Grade) {
		writeError(w, http.StatusUnprocessableEntity, "Kelas tidak valid")
		return
	}
	ctx := r.Context()
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

	cfg := gradeconfig.Get(body.Grade)
	total := cfg.JumlahSoal
	if examType.JumlahSoal != nil {
		total = *examType.JumlahSoal
	}
	durasi := cfg.DurasiMenit
	if examType.DurasiMenit != nil {
		durasi = *examType.DurasiMenit
	}

	var counts map[string]int
	if len(examType.TipeSoal) > 0 {
		counts = map[string]int{}
		sum := 0
		for t, n := range examType.TipeSoal {
			if n > 0 {
				counts[t] = n
				sum += n
			}
		}
		if sum > 0 {
			total = sum
		} else {
			counts = llm.SplitCounts(total)
		}
	} else {
		counts = llm.SplitCounts(total)
	}

	materials, err := h.Store.MaterialsForGeneration(ctx, sub.ID, body.Grade, body.ExamTypeID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Terjadi kesalahan pada server.")
		return
	}
	materialText := ""
	for i, m := range materials {
		if i > 0 {
			materialText += "\n\n"
		}
		materialText += m.Title + ":\n" + m.Content
	}

	batchID := idgen.NewUUID()
	var createdIDs []string
	for i := 0; i < body.JumlahPaket; i++ {
		rawQuestions, err := h.LLM.GenerateQuiz(ctx, sub.Name, body.Grade, counts, materialText)
		if err != nil {
			if len(createdIDs) == 0 {
				writeError(w, http.StatusBadGateway, err.Error())
				return
			}
			break // packages already made this call stay saved as pool
		}
		h.resolveImages(ctx, rawQuestions)

		questions := make([]store.Question, len(rawQuestions))
		for j, rq := range rawQuestions {
			sq, err := rq.ToStoreQuestion()
			if err != nil {
				writeError(w, http.StatusInternalServerError, "Terjadi kesalahan pada server.")
				return
			}
			questions[j] = sq
		}

		quiz, err := h.Store.CreateQuiz(ctx, store.QuizInput{
			SubjectID: sub.ID, Grade: body.Grade, ExamTypeID: body.ExamTypeID,
			Questions: questions, BatchID: batchID, DurasiMenit: durasi,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Terjadi kesalahan pada server.")
			return
		}
		createdIDs = append(createdIDs, quiz.ID)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"generated": len(createdIDs),
		"quiz_ids":  createdIDs,
	})
}
