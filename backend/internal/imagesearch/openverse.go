// Package imagesearch looks up free/openly-licensed stock photos via
// Openverse, as an alternative to AI-generated illustrations for questions
// needing a real-world object/place/creature photo.
package imagesearch

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"
)

const openverseURL = "https://api.openverse.org/v1/images/"

var httpClient = &http.Client{Timeout: 30 * time.Second}

// SearchStockImage returns image bytes for the first Openverse result
// matching query, or nil if there's no match or the lookup fails — a nil
// result is an expected, non-error outcome, not a failure to report.
func SearchStockImage(ctx context.Context, query string) []byte {
	reqURL := openverseURL + "?" + url.Values{"q": {query}, "page_size": {"1"}}.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		slog.Warn("Pencarian gambar stok gagal", "err", err)
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	var parsed struct {
		Results []struct {
			URL string `json:"url"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil || len(parsed.Results) == 0 {
		return nil
	}
	imageURL := parsed.Results[0].URL
	if imageURL == "" {
		return nil
	}

	imgReq, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		return nil
	}
	imgResp, err := httpClient.Do(imgReq)
	if err != nil {
		slog.Warn("Pencarian gambar stok gagal", "err", err)
		return nil
	}
	defer imgResp.Body.Close()
	if imgResp.StatusCode != http.StatusOK {
		return nil
	}
	data, err := io.ReadAll(imgResp.Body)
	if err != nil {
		return nil
	}
	return data
}
