package httpapi

import (
	"log/slog"
	"net/http"
)

// withCORS allows only the configured frontend origin, all methods/headers,
// no Allow-Credentials — matches the Python backend's CORSMiddleware
// config (cookies still work because nginx proxies same-origin in prod).
func withCORS(frontendOrigin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", frontendOrigin)
		w.Header().Set("Access-Control-Allow-Methods", "*")
		w.Header().Set("Access-Control-Allow-Headers", "*")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// withRecovery is the last-resort safety net: any unhandled panic is
// logged with its stack and answered with the same generic 500 body the
// Python backend's global exception handler returns, instead of the
// connection dying or leaking an internal error.
func withRecovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("unhandled panic", "method", r.Method, "path", r.URL.Path, "recovered", rec)
				writeError(w, http.StatusInternalServerError, "Terjadi kesalahan pada server.")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
