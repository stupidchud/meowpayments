// Package auth provides API key authentication for the meowpayments operator API.
package auth

import (
	"net/http"
	"strings"
)

const headerName = "X-API-Key"

// Middleware returns an http.Handler middleware that validates the operator API key.
// The key may be provided as:
//   - X-API-Key: <key>  (header)
//   - Authorization: Bearer <key>  (header)
//   - ?token=<key>  (query param, for WebSocket browser clients)
func Middleware(apiKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get(headerName)
			if key == "" {
				if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
					key = strings.TrimPrefix(auth, "Bearer ")
				}
			}
			if key == "" {
				key = r.URL.Query().Get("token")
			}
			if key != apiKey {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
