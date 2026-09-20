// Package supabaseauth talks to Supabase's Auth and Storage REST APIs —
// the two pieces of Supabase that have no direct-Postgres equivalent, so
// the Go backend (unlike its table access, which goes straight to
// Postgres via pgx) still reaches them over HTTP with the service key.
package supabaseauth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// TransportError wraps a network-level failure talking to Supabase (DNS,
// connection refused, timeout, or a 5xx from Supabase itself) — callers
// must map this to a 5xx response, never to "invalid credentials/session"
// (mirrors the Python backend's AuthRetryableError handling).
type TransportError struct{ err error }

func (e *TransportError) Error() string { return fmt.Sprintf("gagal terhubung ke Supabase: %v", e.err) }
func (e *TransportError) Unwrap() error { return e.err }

var ErrInvalidCredentials = errors.New("email atau password salah")
var ErrInvalidSession = errors.New("sesi tidak valid, silakan login ulang")

type Client struct {
	BaseURL     string
	ServiceKey  string
	httpClient  *http.Client
	bucketsDone sync.Map // bucket name -> struct{}, set once ensureBucket has run for it
}

func New(baseURL, serviceKey string) *Client {
	return &Client{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		ServiceKey: serviceKey,
		httpClient: &http.Client{Timeout: 20 * time.Second},
	}
}

type Session struct {
	AccessToken string
	User        User
}

type User struct {
	ID    string
	Email string
}

// Login authenticates an admin via Supabase's password grant.
func (c *Client) Login(ctx context.Context, email, password string) (*Session, error) {
	body, _ := json.Marshal(map[string]string{"email": email, "password": password})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.BaseURL+"/auth/v1/token?grant_type=password", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("apikey", c.ServiceKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, &TransportError{err}
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 500 {
		return nil, &TransportError{fmt.Errorf("status %d", resp.StatusCode)}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, ErrInvalidCredentials
	}

	var parsed struct {
		AccessToken string `json:"access_token"`
		User        struct {
			ID    string `json:"id"`
			Email string `json:"email"`
		} `json:"user"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil || parsed.AccessToken == "" || parsed.User.ID == "" {
		return nil, ErrInvalidCredentials
	}
	return &Session{
		AccessToken: parsed.AccessToken,
		User:        User{ID: parsed.User.ID, Email: parsed.User.Email},
	}, nil
}

// GetUser resolves a bearer access token to its Supabase Auth user,
// exactly like the Python backend's sb.auth.get_user(token) — a real call
// to Supabase so that revoked/expired sessions are caught server-side, not
// just by decoding the JWT locally.
func (c *Client) GetUser(ctx context.Context, accessToken string) (*User, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/auth/v1/user", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("apikey", c.ServiceKey)
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, &TransportError{err}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		return nil, &TransportError{fmt.Errorf("status %d", resp.StatusCode)}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, ErrInvalidSession
	}
	var parsed struct {
		ID    string `json:"id"`
		Email string `json:"email"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil || parsed.ID == "" {
		return nil, ErrInvalidSession
	}
	return &User{ID: parsed.ID, Email: parsed.Email}, nil
}
