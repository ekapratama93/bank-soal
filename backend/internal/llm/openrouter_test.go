package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// newTestClient points a Client at a local httptest server. Retry tests
// each incur one real chatRetryDelay (1s) sleep — short enough not to
// matter for a handful of tests.
func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return NewClient("fake-key-for-tests", "test-model", "test-image-model", srv.URL)
}

func chatOKResponse(content string) []byte {
	body, _ := json.Marshal(map[string]any{
		"choices": []map[string]any{{"message": map[string]any{"content": content}}},
	})
	return body
}

func TestChatRetriesOnConnectionErrorThenSucceeds(t *testing.T) {
	var calls int32
	// A server that resets the connection on the first call simulates a
	// transient network failure without relying on process orchestration.
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			hj, _ := w.(http.Hijacker)
			conn, _, _ := hj.Hijack()
			conn.Close()
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write(chatOKResponse("hasil"))
	}))
	defer srv.Close()

	c := NewClient("fake-key-for-tests", "test-model", "test-image-model", srv.URL)
	content, err := c.chat(context.Background(), []chatMessage{{Role: "user", Content: "x"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "hasil" {
		t.Errorf("content = %q, want hasil", content)
	}
	if calls != 2 {
		t.Errorf("calls = %d, want 2", calls)
	}
}

func TestChatGivesUpAfterSustainedConnectionFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hj, _ := w.(http.Hijacker)
		conn, _, _ := hj.Hijack()
		conn.Close()
	}))
	defer srv.Close()

	c := NewClient("fake-key-for-tests", "test-model", "test-image-model", srv.URL)
	_, err := c.chat(context.Background(), []chatMessage{{Role: "user", Content: "x"}})
	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestChatRetriesTransient5xxThenSucceeds(t *testing.T) {
	var calls int32
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write(chatOKResponse("hasil"))
	})
	content, err := c.chat(context.Background(), []chatMessage{{Role: "user", Content: "x"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "hasil" {
		t.Errorf("content = %q, want hasil", content)
	}
	if calls != 2 {
		t.Errorf("calls = %d, want 2", calls)
	}
}

func TestChatDoesNotRetryNonTransient4xx(t *testing.T) {
	var calls int32
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusUnauthorized)
	})
	_, err := c.chat(context.Background(), []chatMessage{{Role: "user", Content: "x"}})
	if err == nil {
		t.Fatal("expected an error")
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1 (401 should not be retried)", calls)
	}
}

func TestGenerateQuizRetriesOnBadJSONThenSucceeds(t *testing.T) {
	var calls int32
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusOK)
		if n == 1 {
			w.Write(chatOKResponse("ini bukan JSON"))
			return
		}
		w.Write(chatOKResponse(validQuestionsJSON))
	})
	qs, err := c.GenerateQuiz(context.Background(), "IPA", 5, validCounts, "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(qs) != 3 {
		t.Fatalf("len = %d, want 3", len(qs))
	}
	if calls != 2 {
		t.Errorf("calls = %d, want 2", calls)
	}
}

func TestGenerateQuizFailsAfterMaxAttempts(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(chatOKResponse("bukan JSON"))
	})
	if _, err := c.GenerateQuiz(context.Background(), "IPA", 5, validCounts, "", nil); err == nil {
		t.Fatal("expected an error")
	}
}

// TestGenerateQuizRetriesOnChatErrorThenSucceeds covers the same policy as
// TestGradeShortAnswersRetriesOnChatErrorThenSucceeds in grading_test.go:
// GenerateQuiz must retry on a chat() failure (here, chat()'s own two
// attempts both hitting a transient 503), not just on an invalid response.
func TestGenerateQuizRetriesOnChatErrorThenSucceeds(t *testing.T) {
	var calls int32
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n <= 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write(chatOKResponse(validQuestionsJSON))
	})
	qs, err := c.GenerateQuiz(context.Background(), "IPA", 5, validCounts, "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(qs) != 3 {
		t.Fatalf("len = %d, want 3", len(qs))
	}
	if calls != 3 {
		t.Errorf("calls = %d, want 3 (chat()'s own 2 attempts, then GenerateQuiz retries once more)", calls)
	}
}
