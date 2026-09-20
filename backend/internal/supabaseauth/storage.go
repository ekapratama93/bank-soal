package supabaseauth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"banksoal/internal/idgen"
)

const (
	QuestionImagesBucket = "question-images"
	MaterialsBucket      = "materials"
)

var extByContentType = map[string]string{
	"image/png":  "png",
	"image/jpeg": "jpg",
	"image/jpg":  "jpg",
	"image/webp": "webp",
}

// ensureBucket creates the given bucket once per process — a failure
// almost always just means it already exists, so it's swallowed exactly
// like the Python backend's ensure_bucket().
func (c *Client) ensureBucket(ctx context.Context, bucket string) {
	if _, done := c.bucketsDone.Load(bucket); done {
		return
	}
	body, _ := json.Marshal(map[string]any{"id": bucket, "name": bucket, "public": true})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/storage/v1/bucket", bytes.NewReader(body))
	if err == nil {
		req.Header.Set("apikey", c.ServiceKey)
		req.Header.Set("Authorization", "Bearer "+c.ServiceKey)
		req.Header.Set("Content-Type", "application/json")
		if resp, err := c.httpClient.Do(req); err == nil {
			resp.Body.Close()
		}
	}
	c.bucketsDone.Store(bucket, struct{}{})
}

// uploadBytes uploads data to bucket/<uuid>.<ext> and returns its public URL.
func (c *Client) uploadBytes(ctx context.Context, bucket, ext string, data []byte, contentType string) (string, error) {
	c.ensureBucket(ctx, bucket)
	path := idgen.NewUUID() + "." + ext

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.BaseURL+"/storage/v1/object/"+bucket+"/"+path, bytes.NewReader(data))
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
	return c.BaseURL + "/storage/v1/object/public/" + bucket + "/" + path, nil
}

// UploadImageBytes uploads image bytes to the question-images bucket and
// returns its public URL.
func (c *Client) UploadImageBytes(ctx context.Context, data []byte, contentType string) (string, error) {
	ext := extByContentType[contentType]
	if ext == "" {
		ext = "jpg"
	}
	return c.uploadBytes(ctx, QuestionImagesBucket, ext, data, contentType)
}

// UploadMaterialFile uploads a material's original uploaded file (docx/pdf/txt)
// to the materials bucket and returns its public URL.
func (c *Client) UploadMaterialFile(ctx context.Context, data []byte, ext, contentType string) (string, error) {
	if ext == "" {
		ext = "bin"
	}
	return c.uploadBytes(ctx, MaterialsBucket, ext, data, contentType)
}
