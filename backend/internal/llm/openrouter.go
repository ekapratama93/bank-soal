// Package llm wraps OpenRouter chat calls for quiz generation and
// short-answer ("deskripsi") grading, ported from the Python backend's
// app/llm.py.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

type Error struct{ msg string }

func (e *Error) Error() string { return e.msg }

func newError(format string, args ...any) error {
	return &Error{msg: fmt.Sprintf(format, args...)}
}

type Client struct {
	APIKey     string
	Model      string
	ImageModel string
	URL        string
	HTTPClient *http.Client
}

func NewClient(apiKey, model, imageModel, url string) *Client {
	return &Client{
		APIKey:     apiKey,
		Model:      model,
		ImageModel: imageModel,
		URL:        url,
		HTTPClient: &http.Client{Timeout: 180 * time.Second},
	}
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// retryableStatus are statuses worth retrying (transient) — not e.g. 400/401
// which will just fail again.
var retryableStatus = map[int]bool{429: true, 500: true, 502: true, 503: true, 504: true}

const (
	chatAttempts   = 2
	chatRetryDelay = time.Second
)

func (c *Client) chat(ctx context.Context, messages []chatMessage) (string, error) {
	if c.APIKey == "" {
		return "", newError("OPENROUTER_API_KEY belum diatur di server")
	}
	payload := map[string]any{
		"model":           c.Model,
		"messages":        messages,
		"temperature":     0.7,
		"response_format": map[string]string{"type": "json_object"},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	var lastErr error
	for attempt := 0; attempt < chatAttempts; attempt++ {
		isLast := attempt == chatAttempts-1
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL, bytes.NewReader(body))
		if err != nil {
			return "", err
		}
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			slog.Warn("Koneksi ke OpenRouter gagal", "attempt", attempt+1, "err", err)
			if isLast {
				return "", newError("Gagal terhubung ke layanan AI. Coba lagi nanti.")
			}
			lastErr = err
			time.Sleep(chatRetryDelay)
			continue
		}

		if resp.StatusCode == http.StatusOK {
			defer resp.Body.Close()
			return readChatContent(resp.Body)
		}

		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 500))
		resp.Body.Close()
		slog.Error("OpenRouter error", "status", resp.StatusCode, "body", string(respBody))
		if retryableStatus[resp.StatusCode] && !isLast {
			time.Sleep(chatRetryDelay)
			continue
		}
		return "", newError("Gagal menghubungi layanan AI. Coba lagi nanti.")
	}
	if lastErr != nil {
		return "", newError("Gagal terhubung ke layanan AI. Coba lagi nanti.")
	}
	return "", newError("Gagal menghubungi layanan AI. Coba lagi nanti.")
}

type chatCompletionResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func readChatContent(r io.Reader) (string, error) {
	var parsed chatCompletionResponse
	if err := json.NewDecoder(r).Decode(&parsed); err != nil {
		return "", newError("Respons AI tidak valid. Coba lagi nanti.")
	}
	if len(parsed.Choices) == 0 {
		return "", newError("Respons AI tidak valid. Coba lagi nanti.")
	}
	content := parsed.Choices[0].Message.Content
	if content == "" {
		return "", newError("Respons AI kosong. Coba lagi nanti.")
	}
	return content, nil
}
