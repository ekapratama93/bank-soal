package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"banksoal/internal/supabaseauth"
)

type AdminInfo struct {
	ID    string
	Email string
}

// requireAdmin resolves the request's bearer token to an admin profile,
// writing the appropriate error response itself (401/403/500) and
// returning ok=false if the caller isn't an authorized admin — mirrors the
// Python backend's require_admin FastAPI dependency.
func (h *Handlers) requireAdmin(w http.ResponseWriter, r *http.Request) (*AdminInfo, bool) {
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		writeError(w, http.StatusUnauthorized, "Harus login sebagai admin")
		return nil, false
	}
	token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))

	user, err := h.Auth.GetUser(r.Context(), token)
	if err != nil {
		var transportErr *supabaseauth.TransportError
		if errors.As(err, &transportErr) {
			// Network/server hiccup talking to Supabase — not an invalid
			// token. Answer 5xx, not "session invalid", so the admin isn't
			// told to pointlessly log in again.
			writeError(w, http.StatusInternalServerError, "Terjadi kesalahan pada server.")
			return nil, false
		}
		writeError(w, http.StatusUnauthorized, "Sesi tidak valid, silakan login ulang")
		return nil, false
	}

	role, err := h.Store.ProfileRole(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Terjadi kesalahan pada server.")
		return nil, false
	}
	if role != "admin" {
		writeError(w, http.StatusForbidden, "Akun ini bukan admin")
		return nil, false
	}
	return &AdminInfo{ID: user.ID, Email: user.Email}, true
}
