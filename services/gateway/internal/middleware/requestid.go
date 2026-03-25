package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

// RequestID adds an X-Request-ID header to the request and response if one is
// not already present. It generates a 16-byte random hex string using
// crypto/rand.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = generateID()
			r.Header.Set("X-Request-ID", id)
		}
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r)
	})
}

// generateID produces a 32-character hex string from 16 random bytes.
func generateID() string {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		// Fallback to a fixed ID if crypto/rand fails (extremely unlikely).
		return "00000000000000000000000000000000"
	}
	return hex.EncodeToString(b)
}
