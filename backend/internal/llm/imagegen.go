package llm

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

// GenerateImage calls OpenRouter's Images API (POST /images) for
// c.ImageModel (openai/gpt-image-2) and returns the raw image bytes plus
// its content type. quality/aspect_ratio are pinned to the cheapest,
// smallest options — these are small illustrations shown next to a single
// quiz question, not full-size artwork, so there is no reason to pay for
// (or store) a high-definition image.
func (c *Client) GenerateImage(ctx context.Context, prompt string) ([]byte, string, error) {
	if c.APIKey == "" {
		return nil, "", newError("OPENROUTER_API_KEY belum diatur di server")
	}
	payload := map[string]any{
		"model":        c.ImageModel,
		"prompt":       prompt,
		"n":            1,
		"quality":      "low",
		"aspect_ratio": "1:1",
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, "", err
	}

	reqCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, c.ImagesURL, bytes.NewReader(body))
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		// Without this, a connection failure would fail the ENTIRE batch of
		// generated packages, not just this question's image.
		slog.Warn("Koneksi ke OpenRouter (gambar) gagal", "err", err)
		return nil, "", newError("Gagal terhubung ke layanan AI gambar.")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		slog.Warn("OpenRouter image error", "status", resp.StatusCode)
		return nil, "", newError("Gagal membuat gambar dari layanan AI.")
	}

	var parsed struct {
		Data []struct {
			B64JSON   string `json:"b64_json"`
			MediaType string `json:"media_type"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, "", newError("Respons gambar AI tidak valid.")
	}
	if len(parsed.Data) == 0 || parsed.Data[0].B64JSON == "" {
		return nil, "", newError("Respons gambar AI tidak valid.")
	}
	contentType := parsed.Data[0].MediaType
	if contentType == "" {
		contentType = "image/png"
	}
	data, err := base64.StdEncoding.DecodeString(parsed.Data[0].B64JSON)
	if err != nil {
		return nil, "", newError("Respons gambar AI tidak valid.")
	}
	return data, contentType, nil
}
