package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

// ShortAnswerItem is one "isian" or "deskripsi" question to be graded by
// the AI. "isian" grading falls back to a local match (see
// internal/textmatch) if the AI call fails, since unlike "deskripsi" it
// appears in nearly every quiz and must stay available even when the AI
// doesn't.
type ShortAnswerItem struct {
	Index        int    `json:"index"`
	Tipe         string `json:"tipe"`
	Pertanyaan   string `json:"pertanyaan"`
	JawabanModel string `json:"jawaban_model"`
	JawabanSiswa string `json:"jawaban_siswa"`
}

type ShortAnswerResult struct {
	Verdict    string
	Skor       float64
	UmpanBalik string
}

// gradeBatchSize is the max number of items sent to the AI in one call. Kept
// small so a failure or slow response grading one batch holds up or
// invalidates as few other questions as possible. Requests run
// CONCURRENTLY (see GradeShortAnswers, bounded by maxConcurrentBatches) so
// wall-clock time tracks the largest wave, not the sum of every batch
// graded one after another.
const gradeBatchSize = 3

// maxConcurrentBatches caps how many requests run at once. "isian" now
// sends one request per question, and a large quiz can have dozens — an
// uncapped burst risks tripping the AI provider's rate limit, which is
// slower overall once retries kick in than a modest, bounded amount of
// parallelism.
const maxConcurrentBatches = 5

// gradeMaxAttempts bounds retries within a single request's grading — both
// on a chat() failure (network error, or a status chat() gave up retrying)
// and on an invalid/incomplete JSON response.
const gradeMaxAttempts = 3

// GradeShortAnswers grades every item concurrently, one request per item
// (bounded by maxConcurrentBatches).
func (c *Client) GradeShortAnswers(ctx context.Context, items []ShortAnswerItem) (map[int]ShortAnswerResult, error) {
	if len(items) == 0 {
		return map[int]ShortAnswerResult{}, nil
	}
	if len(items) <= gradeBatchSize {
		return c.gradeBatch(ctx, items)
	}

	var batches [][]ShortAnswerItem
	for i := 0; i < len(items); i += gradeBatchSize {
		end := i + gradeBatchSize
		if end > len(items) {
			end = len(items)
		}
		batches = append(batches, items[i:end])
	}

	results := make([]map[int]ShortAnswerResult, len(batches))
	errs := make([]error, len(batches))
	var wg sync.WaitGroup
	sem := make(chan struct{}, maxConcurrentBatches)
	// Wait for EVERY batch to finish (success or failure) before deciding,
	// instead of cancelling the rest as soon as one fails — mirrors
	// asyncio.gather(..., return_exceptions=True) in the Python original.
	for i, batch := range batches {
		wg.Add(1)
		go func(i int, batch []ShortAnswerItem) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			res, err := c.gradeBatch(ctx, batch)
			results[i] = res
			errs[i] = err
		}(i, batch)
	}
	wg.Wait()

	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}
	merged := map[int]ShortAnswerResult{}
	for _, r := range results {
		for k, v := range r {
			merged[k] = v
		}
	}
	return merged, nil
}

type gradeHasil struct {
	Index      int     `json:"index"`
	Skor       float64 `json:"skor"`
	UmpanBalik string  `json:"umpan_balik"`
}

type gradeResponse struct {
	Hasil []gradeHasil `json:"hasil"`
}

func (c *Client) gradeBatch(ctx context.Context, items []ShortAnswerItem) (map[int]ShortAnswerResult, error) {
	itemsJSON, err := json.Marshal(items)
	if err != nil {
		return nil, err
	}
	prompt := fmt.Sprintf(`Anda guru yang mengoreksi jawaban siswa sekolah Indonesia. Setiap item punya field "tipe": "isian" (jawaban singkat/faktual) atau "deskripsi" (uraian/paragraf). Aturan penilaian berbeda per tipe:

Untuk tipe "deskripsi": nilai kelengkapan dan ketepatan isi jawaban siswa dibanding jawaban model — jawaban siswa tidak harus sama kata per kata dengan jawaban model. Beri "skor" 0.0-1.0:
- 1.0: isi lengkap dan tepat
- 0.1-0.9: sebagian benar/lengkap
- 0.0: salah, kosong, atau tidak relevan

Untuk tipe "isian": ini isian singkat/faktual, PENILAIAN HARUS BINARY — "skor" HANYA BOLEH 1.0 atau 0.0, tidak ada nilai di antaranya:
- 1.0: secara makna/isi sama dengan jawaban model, meskipun beda kata, sinonim, urutan, ejaan minor, atau format angka
- 0.0: beda makna, tidak relevan, atau kosong

Umpan balik dalam Bahasa Indonesia berisi koreksi AI — sebutkan poin/ide yang sudah tepat dari jawaban siswa dan poin penting dari jawaban model yang belum atau kurang dicantumkan (1-3 kalimat).

Item jawaban:
%s

Balas HANYA JSON valid:
{"hasil": [{"index": 0, "skor": 0.0, "umpan_balik": "..."}]}`, string(itemsJSON))

	messages := []chatMessage{
		{Role: "system", Content: "Anda korektor jawaban siswa. Anda selalu menjawab dengan JSON valid saja."},
		{Role: "user", Content: prompt},
	}

	var lastErr error
	for attempt := 0; attempt < gradeMaxAttempts; attempt++ {
		content, err := c.chat(ctx, messages)
		if err != nil {
			// Retry too on a chat() failure (network error, or a transient
			// status chat() itself already gave up retrying), not just on
			// an invalid response — the whole point of one request per
			// question is that a single failed request shouldn't cost that
			// question its AI grading (see the isian fallback in
			// quiz_submit.go, which only kicks in once every attempt here
			// is exhausted).
			lastErr = err
			continue
		}
		result, verr := parseGradeResponse(content, items)
		if verr == nil {
			return result, nil
		}
		lastErr = verr
		messages = append(messages[:2], chatMessage{Role: "assistant", Content: content}, chatMessage{
			Role:    "user",
			Content: fmt.Sprintf("JSON Anda tidak valid: %s. Balas HANYA JSON valid dengan hasil untuk SEMUA index jawaban.", verr),
		})
	}
	return nil, newError("Gagal mengoreksi jawaban uraian. Coba kumpulkan ulang beberapa saat lagi. (%v)", lastErr)
}

// verdictFromSkor derives the "benar"/"parsial"/"salah" verdict from a
// clamped skor instead of asking the model for both — the two could
// otherwise disagree (e.g. the model saying "benar" with skor 0.7).
func verdictFromSkor(skor float64) string {
	switch {
	case skor >= 1:
		return "benar"
	case skor <= 0:
		return "salah"
	default:
		return "parsial"
	}
}

func parseGradeResponse(content string, items []ShortAnswerItem) (map[int]ShortAnswerResult, error) {
	var parsed gradeResponse
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		return nil, fmt.Errorf("hasil bukan JSON valid: %w", err)
	}
	tipeByIndex := make(map[int]string, len(items))
	for _, it := range items {
		tipeByIndex[it.Index] = it.Tipe
	}
	result := map[int]ShortAnswerResult{}
	for _, h := range parsed.Hasil {
		skor := h.Skor
		if skor < 0 {
			skor = 0
		}
		if skor > 1 {
			skor = 1
		}
		// "isian" is binary only — round to the nearer of 0.0/1.0 rather
		// than trusting the model to only ever emit exactly one of the two,
		// despite being told to.
		if tipeByIndex[h.Index] == "isian" {
			if skor >= 0.5 {
				skor = 1
			} else {
				skor = 0
			}
		}
		result[h.Index] = ShortAnswerResult{Verdict: verdictFromSkor(skor), Skor: skor, UmpanBalik: h.UmpanBalik}
	}
	var missing []int
	for _, it := range items {
		if _, ok := result[it.Index]; !ok {
			missing = append(missing, it.Index)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("hasil kurang untuk index: %v", missing)
	}
	return result, nil
}
