package llm

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// GenerateImage calls OpenRouter's image-output model and returns the raw
// image bytes plus its content type.
func (c *Client) GenerateImage(ctx context.Context, prompt string) ([]byte, string, error) {
	if c.APIKey == "" {
		return nil, "", newError("OPENROUTER_API_KEY belum diatur di server")
	}
	payload := map[string]any{
		"model":      c.ImageModel,
		"messages":   []chatMessage{{Role: "user", Content: prompt}},
		"modalities": []string{"image", "text"},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, "", err
	}

	reqCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, c.URL, bytes.NewReader(body))
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
		Choices []struct {
			Message struct {
				Images []struct {
					ImageURL struct {
						URL string `json:"url"`
					} `json:"image_url"`
				} `json:"images"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, "", newError("Respons gambar AI tidak valid.")
	}
	if len(parsed.Choices) == 0 || len(parsed.Choices[0].Message.Images) == 0 {
		return nil, "", newError("Respons gambar AI tidak valid.")
	}
	imageURL := parsed.Choices[0].Message.Images[0].ImageURL.URL
	header, b64data, ok := strings.Cut(imageURL, ",")
	if !ok {
		return nil, "", newError("Respons gambar AI tidak valid.")
	}
	contentType := "image/jpeg"
	if strings.HasPrefix(header, "data:") {
		mediaType := strings.TrimPrefix(header, "data:")
		mediaType, _, _ = strings.Cut(mediaType, ";")
		if mediaType != "" {
			contentType = mediaType
		}
	}
	data, err := base64.StdEncoding.DecodeString(b64data)
	if err != nil {
		return nil, "", newError("Respons gambar AI tidak valid.")
	}
	return data, contentType, nil
}
