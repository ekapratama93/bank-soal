// Package httpapi wires up the HTTP router, middleware, and per-domain
// handlers, replacing the Python backend's FastAPI routers.
package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
)

// writeJSON encodes v as the response body. Every success response in this
// API is JSON (or 204 No Content), matching the FastAPI backend.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("gagal menulis respons JSON", "err", err)
	}
}

// writeError writes {"detail": "<msg>"} — the frontend's api client reads
// this exact field for every non-2xx response.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"detail": msg})
}

func writeNoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// decodeJSON reads and JSON-decodes the request body, writing a 422 and
// returning false on failure — callers should return immediately when this
// returns false.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(dst); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "Body permintaan tidak valid: "+err.Error())
		return false
	}
	return true
}

// readAndRestoreBody reads the full request body and replaces r.Body with
// a fresh reader over the same bytes, so the body can be decoded more than
// once — used by PATCH handlers that need both the raw JSON (to detect
// which fields were sent) and the typed decode.
func readAndRestoreBody(r *http.Request) ([]byte, error) {
	defer r.Body.Close()
	data, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	r.Body = io.NopCloser(bytes.NewReader(data))
	return data, nil
}
