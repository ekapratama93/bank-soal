package httpapi

import (
	"net/http"

	"banksoal/internal/idgen"
)

const clientCookieName = "client_id"

// ensureClientID reads the client_id cookie or creates one — the anonymous
// per-device id used to correlate quiz attempts, with no student login at
// all.
func ensureClientID(w http.ResponseWriter, r *http.Request) string {
	clientID := ""
	if cookie, err := r.Cookie(clientCookieName); err == nil {
		clientID = cookie.Value
	}
	if clientID == "" {
		clientID = idgen.NewUUID()
	}
	http.SetCookie(w, &http.Cookie{
		Name:     clientCookieName,
		Value:    clientID,
		Path:     "/",
		MaxAge:   60 * 60 * 24 * 365, // 1 year (browsers commonly cap at ~400 days)
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	return clientID
}
