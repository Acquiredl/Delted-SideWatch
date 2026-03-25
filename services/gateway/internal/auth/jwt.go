package auth

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

// ClaimsKey is the context key used to store validated JWT claims.
const ClaimsKey contextKey = "claims"

// Middleware returns an HTTP middleware that validates JWT tokens.
// It only protects paths matching the given prefixes (e.g., "/admin/").
// Unprotected paths pass through without authentication.
func Middleware(secret string, protectedPrefixes []string, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if the request path matches any protected prefix.
			protected := false
			for _, prefix := range protectedPrefixes {
				if strings.HasPrefix(r.URL.Path, prefix) {
					protected = true
					break
				}
			}

			// If not a protected route, pass through.
			if !protected {
				next.ServeHTTP(w, r)
				return
			}

			// Extract Bearer token from Authorization header.
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeJSONError(w, http.StatusUnauthorized, "missing authorization header")
				logger.Warn("missing authorization header", "path", r.URL.Path)
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				writeJSONError(w, http.StatusUnauthorized, "invalid authorization header format")
				logger.Warn("invalid authorization header format", "path", r.URL.Path)
				return
			}
			tokenStr := parts[1]

			// Validate the token with HS256.
			token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrSignatureInvalid
				}
				return []byte(secret), nil
			})
			if err != nil || !token.Valid {
				writeJSONError(w, http.StatusUnauthorized, "invalid or expired token")
				logger.Warn("invalid or expired token", "path", r.URL.Path, "error", err)
				return
			}

			// Add claims to the request context.
			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				writeJSONError(w, http.StatusUnauthorized, "invalid token claims")
				logger.Warn("invalid token claims", "path", r.URL.Path)
				return
			}

			ctx := context.WithValue(r.Context(), ClaimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// writeJSONError writes a JSON error response with the given status code and message.
func writeJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
