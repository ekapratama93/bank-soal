package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

// ShortAnswerItem is one "deskripsi" question to be graded by the AI —
// "isian" short answers are graded locally (see internal/textmatch) and
// never reach this path, which is what keeps the AI batch small.
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

// gradeBatchSize is the max number of "deskripsi" items sent to the AI in
// one call; larger packages are split into batches graded CONCURRENTLY (see
// GradeShortAnswers) so wall-clock time tracks the largest batch, not the
// sum of all items graded one after another.
const gradeBatchSize = 4

// GradeShortAnswers grades every item, batching + parallelizing for
// packages with more than gradeBatchSize "deskripsi" questions (rare —
// automatic composition never includes "deskripsi"; this only happens when
// an admin configures a custom tipe_soal).
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
	// Wait for EVERY batch to finish (success or failure) before deciding,
	// instead of cancelling the rest as soon as one fails — mirrors
	// asyncio.gather(..., return_exceptions=True) in the Python original.
	for i, batch := range batches {
		wg.Add(1)
		go func(i int, batch []ShortAnswerItem) {
			defer wg.Done()
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
	Verdict    string  `json:"verdict"`
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
	prompt := fmt.Sprintf(`Anda guru yang mengoreksi jawaban uraian/deskripsi siswa sekolah Indonesia. Untuk setiap item, nilai kelengkapan dan ketepatan isi jawaban siswa dibanding jawaban model — jawaban siswa tidak harus sama kata per kata dengan jawaban model:
- "benar" (skor 1.0): isi lengkap dan tepat
- "parsial" (skor 0.1-0.9): sebagian benar/lengkap
- "salah" (skor 0.0): salah, kosong, atau tidak relevan
Umpan balik dalam Bahasa Indonesia berisi koreksi AI — sebutkan poin/ide yang sudah tepat dari jawaban siswa dan poin penting dari jawaban model yang belum atau kurang dicantumkan (1-3 kalimat).

Item jawaban:
%s

Balas HANYA JSON valid:
{"hasil": [{"index": 0, "verdict": "benar|parsial|salah", "skor": 0.0, "umpan_balik": "..."}]}`, string(itemsJSON))

	messages := []chatMessage{
		{Role: "system", Content: "Anda korektor jawaban siswa. Anda selalu menjawab dengan JSON valid saja."},
		{Role: "user", Content: prompt},
	}

	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		content, err := c.chat(ctx, messages)
		if err != nil {
			return nil, err
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

func parseGradeResponse(content string, items []ShortAnswerItem) (map[int]ShortAnswerResult, error) {
	var parsed gradeResponse
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		return nil, fmt.Errorf("hasil bukan JSON valid: %w", err)
	}
	result := map[int]ShortAnswerResult{}
	for _, h := range parsed.Hasil {
		if h.Verdict != "benar" && h.Verdict != "parsial" && h.Verdict != "salah" {
			return nil, fmt.Errorf("item hasil tidak valid: %+v", h)
		}
		skor := h.Skor
		if skor < 0 {
			skor = 0
		}
		if skor > 1 {
			skor = 1
		}
		result[h.Index] = ShortAnswerResult{Verdict: h.Verdict, Skor: skor, UmpanBalik: h.UmpanBalik}
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
