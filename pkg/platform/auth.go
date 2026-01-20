package platform

import (
	"crypto/subtle"
	"net/http"
)

// BasicAuthMiddleware enforces Username/Password check from env vars.
// Default: FAIL if not configured
func BasicAuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := GetEnv("AUTH_USER", "")
		pass := GetEnv("AUTH_PASS", "")

		if user == "" || pass == "" {
			// Unsafe configuration - service should ideally not start, but middleware panicking is harsh.
			// Return 503 or 500.
			http.Error(w, "Service Authentication Not Configured", http.StatusServiceUnavailable)
			return
		}

		u, p, ok := r.BasicAuth()
		if !ok || subtle.ConstantTimeCompare([]byte(u), []byte(user)) != 1 || subtle.ConstantTimeCompare([]byte(p), []byte(pass)) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

// APIKeyMiddleware enforces X-API-Key header.
func APIKeyMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := GetEnv("API_KEY", "")
		if key == "" {
			// If no key configured, skip auth (unsafe defaults logic should warn, but keeping compatible)
			next(w, r)
			return
		}

		if r.Header.Get("X-API-Key") != key {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}
