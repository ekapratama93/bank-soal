package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func item(i int) ShortAnswerItem {
	return ShortAnswerItem{
		Index: i, Tipe: "deskripsi", Pertanyaan: fmt.Sprintf("Soal %d", i),
		JawabanModel: fmt.Sprintf("jawaban model %d", i), JawabanSiswa: fmt.Sprintf("jawaban siswa %d", i),
	}
}

// requestItems extracts the ShortAnswerItem list actually sent in a
// gradeBatch prompt, by pulling it back out of the chat request body.
func requestItems(r *http.Request) []ShortAnswerItem {
	var body struct {
		Messages []chatMessage `json:"messages"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	prompt := body.Messages[len(body.Messages)-1].Content
	start := indexOf(prompt, "Item jawaban:\n") + len("Item jawaban:\n")
	end := indexOf(prompt, "\n\nBalas HANYA JSON valid:")
	var items []ShortAnswerItem
	_ = json.Unmarshal([]byte(prompt[start:end]), &items)
	return items
}

func indexOf(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func hasilJSON(items []ShortAnswerItem) []byte {
	type h struct {
		Index      int     `json:"index"`
		Verdict    string  `json:"verdict"`
		Skor       float64 `json:"skor"`
		UmpanBalik string  `json:"umpan_balik"`
	}
	hasil := make([]h, len(items))
	for i, it := range items {
		hasil[i] = h{Index: it.Index, Verdict: "benar", Skor: 1.0, UmpanBalik: "ok"}
	}
	b, _ := json.Marshal(map[string]any{"hasil": hasil})
	return b
}

func TestGradeShortAnswersSmallBatchUsesSingleCall(t *testing.T) {
	items := []ShortAnswerItem{item(0), item(1), item(2), item(3)} // == gradeBatchSize
	var calls int32
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.Write(chatOKResponse(string(hasilJSON(items))))
	})
	result, err := c.GradeShortAnswers(context.Background(), items)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1", calls)
	}
	if len(result) != 4 {
		t.Errorf("len(result) = %d, want 4", len(result))
	}
}

func TestGradeShortAnswersLargeBatchSplitsIntoMultipleCalls(t *testing.T) {
	items := make([]ShortAnswerItem, 10) // > gradeBatchSize (4)
	for i := range items {
		items[i] = item(i)
	}
	var mu sync.Mutex
	var batchSizes []int
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		batch := requestItems(r)
		mu.Lock()
		batchSizes = append(batchSizes, len(batch))
		mu.Unlock()
		w.Write(chatOKResponse(string(hasilJSON(batch))))
	})
	result, err := c.GradeShortAnswers(context.Background(), items)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 10 items / 4 per batch -> 3 calls (4, 4, 2), not one big call.
	if len(batchSizes) != 3 {
		t.Fatalf("got %d calls, want 3: %v", len(batchSizes), batchSizes)
	}
	if len(result) != 10 {
		t.Errorf("len(result) = %d, want 10", len(result))
	}
	for i := 0; i < 10; i++ {
		if _, ok := result[i]; !ok {
			t.Errorf("missing index %d in merged result", i)
		}
	}
}

func TestGradeShortAnswersBatchesRunConcurrently(t *testing.T) {
	items := make([]ShortAnswerItem, 8) // -> 2 batches
	for i := range items {
		items[i] = item(i)
	}
	var inFlight, maxInFlight int32
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&inFlight, 1)
		for {
			old := atomic.LoadInt32(&maxInFlight)
			if n <= old || atomic.CompareAndSwapInt32(&maxInFlight, old, n) {
				break
			}
		}
		time.Sleep(50 * time.Millisecond) // give the other batch time to start
		batch := requestItems(r)
		atomic.AddInt32(&inFlight, -1)
		w.Write(chatOKResponse(string(hasilJSON(batch))))
	})
	if _, err := c.GradeShortAnswers(context.Background(), items); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if maxInFlight != 2 {
		t.Errorf("maxInFlight = %d, want 2 (batches should run concurrently)", maxInFlight)
	}
}

func TestGradeShortAnswersOneFailingBatchReturnsError(t *testing.T) {
	items := make([]ShortAnswerItem, 8) // -> 2 batches
	for i := range items {
		items[i] = item(i)
	}
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		batch := requestItems(r)
		for _, it := range batch {
			if it.Index == 0 {
				w.WriteHeader(http.StatusUnauthorized) // non-retryable -> fails immediately
				return
			}
		}
		w.Write(chatOKResponse(string(hasilJSON(batch))))
	})
	if _, err := c.GradeShortAnswers(context.Background(), items); err == nil {
		t.Fatal("expected an error when one batch fails")
	}
}

func TestGradeShortAnswersMergedResultsPreserveScoresPerItem(t *testing.T) {
	items := make([]ShortAnswerItem, 6) // -> batches of 4, 2
	for i := range items {
		items[i] = item(i)
	}
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		batch := requestItems(r)
		type h struct {
			Index      int     `json:"index"`
			Verdict    string  `json:"verdict"`
			Skor       float64 `json:"skor"`
			UmpanBalik string  `json:"umpan_balik"`
		}
		hasil := make([]h, len(batch))
		for i, it := range batch {
			hasil[i] = h{Index: it.Index, Verdict: "parsial", Skor: float64(it.Index) / 10, UmpanBalik: "catatan " + strconv.Itoa(it.Index)}
		}
		b, _ := json.Marshal(map[string]any{"hasil": hasil})
		w.Write(chatOKResponse(string(b)))
	})
	result, err := c.GradeShortAnswers(context.Background(), items)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i := 0; i < 6; i++ {
		want := float64(i) / 10
		if result[i].Skor != want {
			t.Errorf("result[%d].Skor = %v, want %v", i, result[i].Skor, want)
		}
		if result[i].UmpanBalik != "catatan "+strconv.Itoa(i) {
			t.Errorf("result[%d].UmpanBalik = %q", i, result[i].UmpanBalik)
		}
	}
}

func TestGradeShortAnswersEmptyItemsNoCall(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("must not call the LLM for an empty item list")
	})
	result, err := c.GradeShortAnswers(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("len(result) = %d, want 0", len(result))
	}
}

func TestGradeShortAnswersSkorClamped(t *testing.T) {
	items := []ShortAnswerItem{item(0)}
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write(chatOKResponse(`{"hasil": [{"index": 0, "verdict": "parsial", "skor": 1.7, "umpan_balik": "x"}]}`))
	})
	result, err := c.GradeShortAnswers(context.Background(), items)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result[0].Skor != 1.0 {
		t.Errorf("Skor = %v, want clamped to 1.0", result[0].Skor)
	}
}

func TestGradeShortAnswersMissingIndexFailsAfterRetry(t *testing.T) {
	items := []ShortAnswerItem{item(0)}
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write(chatOKResponse(`{"hasil": []}`))
	})
	if _, err := c.GradeShortAnswers(context.Background(), items); err == nil {
		t.Fatal("expected an error for a response missing the requested index")
	}
}
