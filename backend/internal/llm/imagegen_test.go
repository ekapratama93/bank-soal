package llm

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestGenerateImageRequestsLowQualitySmallResolution locks in that
// GenerateImage always asks OpenRouter's Images API for the cheapest,
// smallest output — these illustrations sit beside one quiz question and
// have no reason to cost more or store more than that.
func TestGenerateImageRequestsLowQualitySmallResolution(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		resp := map[string]any{
			"data": []map[string]any{
				{"b64_json": base64.StdEncoding.EncodeToString([]byte("fake-image-bytes")), "media_type": "image/png"},
			},
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := NewClient("fake-key-for-tests", "test-model", "openai/gpt-image-2", srv.URL, srv.URL)
	data, contentType, err := c.GenerateImage(context.Background(), "diagram segitiga siku-siku")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != "fake-image-bytes" {
		t.Errorf("data = %q, want %q", data, "fake-image-bytes")
	}
	if contentType != "image/png" {
		t.Errorf("contentType = %q, want image/png", contentType)
	}

	if gotBody["model"] != "openai/gpt-image-2" {
		t.Errorf("model = %v", gotBody["model"])
	}
	if gotBody["prompt"] != "diagram segitiga siku-siku" {
		t.Errorf("prompt = %v", gotBody["prompt"])
	}
	if gotBody["quality"] != "low" {
		t.Errorf("quality = %v, want low", gotBody["quality"])
	}
	if gotBody["aspect_ratio"] != "1:1" {
		t.Errorf("aspect_ratio = %v, want 1:1", gotBody["aspect_ratio"])
	}
}

func TestGenerateImageErrorsOnEmptyData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}})
	}))
	defer srv.Close()

	c := NewClient("fake-key-for-tests", "test-model", "test-image-model", srv.URL, srv.URL)
	if _, _, err := c.GenerateImage(context.Background(), "x"); err == nil {
		t.Fatal("expected an error for an empty data array")
	}
}
