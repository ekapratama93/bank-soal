package supabaseauth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"banksoal/internal/idgen"
)

const QuestionImagesBucket = "question-images"

var extByContentType = map[string]string{
	"image/png":  "png",
	"image/jpeg": "jpg",
	"image/jpg":  "jpg",
	"image/webp": "webp",
}

// ensureBucket creates the question-images bucket once per process — a
// failure almost always just means it already exists, so it's swallowed
// exactly like the Python backend's ensure_bucket().
func (c *Client) ensureBucket(ctx context.Context) {
	if c.bucketDone.Load() {
		return
	}
	body, _ := json.Marshal(map[string]any{"id": QuestionImagesBucket, "name": QuestionImagesBucket, "public": true})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/storage/v1/bucket", bytes.NewReader(body))
	if err == nil {
		req.Header.Set("apikey", c.ServiceKey)
		req.Header.Set("Authorization", "Bearer "+c.ServiceKey)
		req.Header.Set("Content-Type", "application/json")
		if resp, err := c.httpClient.Do(req); err == nil {
			resp.Body.Close()
		}
	}
	c.bucketDone.Store(true)
}

// UploadImageBytes uploads image bytes to the question-images bucket and
// returns its public URL.
func (c *Client) UploadImageBytes(ctx context.Context, data []byte, contentType string) (string, error) {
	c.ensureBucket(ctx)
	ext := extByContentType[contentType]
	if ext == "" {
		ext = "jpg"
	}
	path := idgen.NewUUID() + "." + ext

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.BaseURL+"/storage/v1/object/"+QuestionImagesBucket+"/"+path, bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("apikey", c.ServiceKey)
	req.Header.Set("Authorization", "Bearer "+c.ServiceKey)
	req.Header.Set("Content-Type", contentType)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", &TransportError{err}
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", &TransportError{fmt.Errorf("upload status %d", resp.StatusCode)}
	}
	return c.BaseURL + "/storage/v1/object/public/" + QuestionImagesBucket + "/" + path, nil
}
