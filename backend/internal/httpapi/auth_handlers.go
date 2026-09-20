package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"banksoal/internal/supabaseauth"
)

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *Handlers) handleLogin(w http.ResponseWriter, r *http.Request) {
	var body loginRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	emailKey := strings.ToLower(strings.TrimSpace(body.Email))
	if !h.LoginLimiter.Allow(emailKey) {
		w.Header().Set("Retry-After", "900")
		writeError(w, http.StatusTooManyRequests, "Terlalu banyak percobaan login. Coba lagi dalam beberapa menit.")
		return
	}

	session, err := h.Auth.Login(r.Context(), body.Email, body.Password)
	if err != nil {
		var transportErr *supabaseauth.TransportError
		if errors.As(err, &transportErr) {
			writeError(w, http.StatusInternalServerError, "Terjadi kesalahan pada server.")
			return
		}
		writeError(w, http.StatusUnauthorized, "Email atau password salah")
		return
	}

	role, err := h.Store.ProfileRole(r.Context(), session.User.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Terjadi kesalahan pada server.")
		return
	}
	if role != "admin" {
		writeError(w, http.StatusForbidden, "Akun ini bukan admin")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token": session.AccessToken,
		"email":        session.User.Email,
		"role":         role,
	})
}
