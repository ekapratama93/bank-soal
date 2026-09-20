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

func isianItem(i int) ShortAnswerItem {
	it := item(i)
	it.Tipe = "isian"
	return it
}

// requestItems extracts the ShortAnswerItem list actually sent in a
// gradeBatch prompt, by pulling it back out of the chat request body.
func requestItems(r *http.Request) []ShortAnswerItem {
	var body struct {
		Messages []chatMessage `json:"messages"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	prompt, _ := body.Messages[len(body.Messages)-1].Content.(string)
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
		Skor       float64 `json:"skor"`
		UmpanBalik string  `json:"umpan_balik"`
	}
	hasil := make([]h, len(items))
	for i, it := range items {
		hasil[i] = h{Index: it.Index, Skor: 1.0, UmpanBalik: "ok"}
	}
	b, _ := json.Marshal(map[string]any{"hasil": hasil})
	return b
}

func TestGradeShortAnswersSmallBatchUsesSingleCall(t *testing.T) {
	items := make([]ShortAnswerItem, gradeBatchSize)
	for i := range items {
		items[i] = item(i)
	}
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
	if len(result) != gradeBatchSize {
		t.Errorf("len(result) = %d, want %d", len(result), gradeBatchSize)
	}
}

func TestGradeShortAnswersLargeBatchSplitsIntoMultipleCalls(t *testing.T) {
	n := gradeBatchSize*2 + 3
	wantBatches := (n + gradeBatchSize - 1) / gradeBatchSize
	items := make([]ShortAnswerItem, n)
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
	if len(batchSizes) != wantBatches {
		t.Fatalf("got %d calls, want %d: %v", len(batchSizes), wantBatches, batchSizes)
	}
	if len(result) != n {
		t.Errorf("len(result) = %d, want %d", len(result), n)
	}
	for i := 0; i < n; i++ {
		if _, ok := result[i]; !ok {
			t.Errorf("missing index %d in merged result", i)
		}
	}
}

func TestGradeShortAnswersBatchesRunConcurrently(t *testing.T) {
	items := make([]ShortAnswerItem, gradeBatchSize+1) // -> 2 batches
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

func TestGradeShortAnswersConcurrencyCapped(t *testing.T) {
	n := gradeBatchSize * (maxConcurrentBatches + 2) // -> maxConcurrentBatches+2 batches
	items := make([]ShortAnswerItem, n)
	for i := range items {
		items[i] = item(i)
	}
	var inFlight, maxInFlight int32
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		cur := atomic.AddInt32(&inFlight, 1)
		for {
			old := atomic.LoadInt32(&maxInFlight)
			if cur <= old || atomic.CompareAndSwapInt32(&maxInFlight, old, cur) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
		batch := requestItems(r)
		atomic.AddInt32(&inFlight, -1)
		w.Write(chatOKResponse(string(hasilJSON(batch))))
	})
	if _, err := c.GradeShortAnswers(context.Background(), items); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if maxInFlight > maxConcurrentBatches {
		t.Errorf("maxInFlight = %d, want <= %d (maxConcurrentBatches)", maxInFlight, maxConcurrentBatches)
	}
	if maxInFlight != maxConcurrentBatches {
		t.Errorf("maxInFlight = %d, want exactly %d given %d batches to schedule", maxInFlight, maxConcurrentBatches, maxConcurrentBatches+2)
	}
}

func TestGradeShortAnswersOneFailingBatchReturnsError(t *testing.T) {
	items := make([]ShortAnswerItem, gradeBatchSize+1) // -> 2 batches
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

func TestGradeShortAnswersRetriesOnChatErrorThenSucceeds(t *testing.T) {
	items := []ShortAnswerItem{item(0)}
	var calls int32
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			w.WriteHeader(http.StatusUnauthorized) // non-retryable at the chat() level -> chat() returns an error immediately
			return
		}
		w.Write(chatOKResponse(string(hasilJSON(items))))
	})
	result, err := c.GradeShortAnswers(context.Background(), items)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 2 {
		t.Errorf("calls = %d, want 2 (a chat() error should be retried, not returned immediately)", calls)
	}
	if len(result) != 1 {
		t.Errorf("len(result) = %d, want 1", len(result))
	}
}

func TestGradeShortAnswersGivesUpAfterMaxAttemptsOnSustainedChatError(t *testing.T) {
	items := []ShortAnswerItem{item(0)}
	var calls int32
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusUnauthorized)
	})
	if _, err := c.GradeShortAnswers(context.Background(), items); err == nil {
		t.Fatal("expected an error after sustained chat() failure")
	}
	if calls != gradeMaxAttempts {
		t.Errorf("calls = %d, want %d (gradeMaxAttempts)", calls, gradeMaxAttempts)
	}
}

func TestGradeShortAnswersMergedResultsPreserveScoresPerItem(t *testing.T) {
	n := gradeBatchSize + 2 // -> multiple single-item requests
	items := make([]ShortAnswerItem, n)
	for i := range items {
		items[i] = item(i)
	}
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		batch := requestItems(r)
		type h struct {
			Index      int     `json:"index"`
			Skor       float64 `json:"skor"`
			UmpanBalik string  `json:"umpan_balik"`
		}
		hasil := make([]h, len(batch))
		for i, it := range batch {
			hasil[i] = h{Index: it.Index, Skor: float64(it.Index) / 10, UmpanBalik: "catatan " + strconv.Itoa(it.Index)}
		}
		b, _ := json.Marshal(map[string]any{"hasil": hasil})
		w.Write(chatOKResponse(string(b)))
	})
	result, err := c.GradeShortAnswers(context.Background(), items)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i := 0; i < n; i++ {
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
		w.Write(chatOKResponse(`{"hasil": [{"index": 0, "skor": 1.7, "umpan_balik": "x"}]}`))
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

func TestGradeShortAnswersIsianSkorRoundedToBinary(t *testing.T) {
	items := []ShortAnswerItem{isianItem(0), isianItem(1), item(2)} // isian below threshold, isian above threshold, deskripsi
	skorByIndex := map[int]float64{0: 0.3, 1: 0.6, 2: 0.6}
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		// Model disobeys the "isian skor must be exactly 0.0 or 1.0" instruction.
		batch := requestItems(r)
		type h struct {
			Index      int     `json:"index"`
			Skor       float64 `json:"skor"`
			UmpanBalik string  `json:"umpan_balik"`
		}
		hasil := make([]h, len(batch))
		for i, it := range batch {
			hasil[i] = h{Index: it.Index, Skor: skorByIndex[it.Index], UmpanBalik: "x"}
		}
		b, _ := json.Marshal(map[string]any{"hasil": hasil})
		w.Write(chatOKResponse(string(b)))
	})
	result, err := c.GradeShortAnswers(context.Background(), items)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := result[0]; got.Verdict != "salah" || got.Skor != 0 {
		t.Errorf("isian skor 0.3 = %+v, want rounded down to salah/0.0", got)
	}
	if got := result[1]; got.Verdict != "benar" || got.Skor != 1.0 {
		t.Errorf("isian skor 0.6 = %+v, want rounded up to benar/1.0", got)
	}
	if got := result[2]; got.Verdict != "parsial" || got.Skor != 0.6 {
		t.Errorf("deskripsi skor 0.6 = %+v, want left as parsial/0.6 (not binarized)", got)
	}
}

func TestVerdictFromSkor(t *testing.T) {
	cases := []struct {
		skor float64
		want string
	}{
		{0, "salah"},
		{1, "benar"},
		{0.5, "parsial"},
		{0.01, "parsial"},
		{0.99, "parsial"},
	}
	for _, c := range cases {
		if got := verdictFromSkor(c.skor); got != c.want {
			t.Errorf("verdictFromSkor(%v) = %q, want %q", c.skor, got, c.want)
		}
	}
}
