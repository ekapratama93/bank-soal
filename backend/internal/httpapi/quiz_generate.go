package httpapi

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"sync"

	"banksoal/internal/gradeconfig"
	"banksoal/internal/idgen"
	"banksoal/internal/imagesearch"
	"banksoal/internal/llm"
	"banksoal/internal/store"
	"banksoal/internal/subjectsdata"
)

const imageCapPerPaket = 3

// maxConcurrentPakets caps how many packages are generated at once.
// jumlah_paket is at most 5, but each package is its own full LLM call
// (heavier than a single grading batch), so firing all of them at once
// still risks tripping the AI provider's rate limit.
const maxConcurrentPakets = 3

// maxMaterialImagesForPrompt caps how many material images are sent to the
// LLM as multimodal input per generation call — keeps the request payload
// and per-call cost bounded even when several materials each contributed
// their own handful of extracted images.
const maxMaterialImagesForPrompt = 10

// materialImageMarkerRe matches the "[Gambar N]" position markers
// fileextract leaves in a material's content, where N is 1-based and local
// to that single material's own Images list (see docx.go/pdf.go).
var materialImageMarkerRe = regexp.MustCompile(`\s?\[Gambar (\d+)\]`)

// buildMaterialContext concatenates every material's title+content into one
// prompt string and flattens their images into one list, capped at
// maxMaterialImagesForPrompt, both sent to the LLM as multimodal context
// (see GenerateQuiz). Each material's own "[Gambar N]" markers are local to
// that material, so they're rewritten here into the
// global index its image ends up at in the flattened list; a marker whose
// image didn't make the cut is dropped, since there's no longer an attached
// image left for it to point to.
func buildMaterialContext(materials []store.Material) (string, []store.MaterialImage) {
	var images []store.MaterialImage
	text := ""
	for i, m := range materials {
		offset := len(images)
		included := len(m.Images)
		if remaining := maxMaterialImagesForPrompt - offset; included > remaining {
			included = remaining
		}
		if included > 0 {
			images = append(images, m.Images[:included]...)
		}

		if i > 0 {
			text += "\n\n"
		}
		text += m.Title + ":\n" + renumberMaterialImageMarkers(m.Content, offset, included)
	}
	return text, images
}

func renumberMaterialImageMarkers(content string, offset, included int) string {
	return materialImageMarkerRe.ReplaceAllStringFunc(content, func(m string) string {
		local, err := strconv.Atoi(materialImageMarkerRe.FindStringSubmatch(m)[1])
		if err != nil || local < 1 || local > included {
			return ""
		}
		return fmt.Sprintf(" [Gambar %d]", offset+local)
	})
}

// resolveImages turns gambar_tipe/gambar_prompt/gambar_cari markers from
// the LLM into a real "gambar" URL, up to imageCapPerPaket per package. A
// failure on any single question must not fail the whole batch — that
// question just ends up with no image. Material images are only ever
// offered to the LLM as multimodal context (see GenerateQuiz) and never
// resolved here directly — every image a question ends up with is freshly
// generated or fetched.
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
	materialText, materialImages := buildMaterialContext(materials)

	// Packages are independent, so they're generated CONCURRENTLY (bounded by
	// maxConcurrentPakets) — wall-clock time then tracks the slowest package
	// instead of the sum of every one generated one after another. Each
	// goroutine writes only to its own index, so no lock is needed for that
	// part; genCtx is cancelled on the first internal (non-LLM) error so the
	// rest stop early instead of doing wasted work whose result would be
	// discarded anyway.
	genCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	batchID := idgen.NewUUID()
	quizIDs := make([]string, body.JumlahPaket)
	genErrs := make([]error, body.JumlahPaket)
	var mu sync.Mutex
	var internalErr error

	var wg sync.WaitGroup
	sem := make(chan struct{}, maxConcurrentPakets)
	for i := 0; i < body.JumlahPaket; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			rawQuestions, err := h.LLM.GenerateQuiz(genCtx, sub.Name, body.Grade, counts, materialText, materialImages)
			if err != nil {
				genErrs[i] = err
				return
			}
			h.resolveImages(genCtx, rawQuestions)

			questions := make([]store.Question, len(rawQuestions))
			for j, rq := range rawQuestions {
				sq, err := rq.ToStoreQuestion()
				if err != nil {
					slog.Error("Gagal mengonversi jawaban soal ke bentuk penyimpanan", "paket_index", i, "err", err)
					mu.Lock()
					if internalErr == nil {
						internalErr = err
					}
					mu.Unlock()
					cancel()
					return
				}
				questions[j] = sq
			}

			quiz, err := h.Store.CreateQuiz(genCtx, store.QuizInput{
				SubjectID: sub.ID, Grade: body.Grade, ExamTypeID: body.ExamTypeID,
				Questions: questions, BatchID: batchID, DurasiMenit: durasi,
			})
			if err != nil {
				slog.Error("Gagal menyimpan paket soal ke database", "paket_index", i, "err", err)
				mu.Lock()
				if internalErr == nil {
					internalErr = err
				}
				mu.Unlock()
				cancel()
				return
			}
			quizIDs[i] = quiz.ID
		}(i)
	}
	wg.Wait()

	// A single package's internal (non-LLM) error must not throw away
	// sibling packages that finished successfully — including ones already
	// committed to the DB before that error fired and cancelled genCtx. Only
	// treat the whole batch as failed when NOTHING came out of it.
	var createdIDs []string
	var firstGenErr error
	for i, id := range quizIDs {
		if id != "" {
			createdIDs = append(createdIDs, id)
		} else if genErrs[i] != nil && firstGenErr == nil {
			firstGenErr = genErrs[i]
		}
	}
	if len(createdIDs) == 0 {
		if internalErr != nil {
			writeError(w, http.StatusInternalServerError, "Terjadi kesalahan pada server.")
			return
		}
		writeError(w, http.StatusBadGateway, firstGenErr.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"generated": len(createdIDs),
		"quiz_ids":  createdIDs,
	})
}
