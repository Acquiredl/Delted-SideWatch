package auth

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const testSecret = "test-secret-key-for-jwt-tests"

func testLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
}

func generateTestToken(t *testing.T, secret string, claims jwt.MapClaims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("generating test token: %v", err)
	}
	return tokenStr
}

func TestJWTMiddleware_ProtectedRouteNoToken(t *testing.T) {
	logger := testLogger()
	mw := Middleware(testSecret, []string{"/admin/"}, logger)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/admin/settings", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rec.Code)
	}
}

func TestJWTMiddleware_ProtectedRouteValidToken(t *testing.T) {
	logger := testLogger()
	mw := Middleware(testSecret, []string{"/admin/"}, logger)

	called := false
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		claims, ok := r.Context().Value(ClaimsKey).(jwt.MapClaims)
		if !ok {
			t.Error("expected claims in context")
			return
		}
		if claims["sub"] != "admin-user" {
			t.Errorf("expected sub=admin-user, got %v", claims["sub"])
		}
		w.WriteHeader(http.StatusOK)
	}))

	tokenStr := generateTestToken(t, testSecret, jwt.MapClaims{
		"sub":  "admin-user",
		"role": "admin",
		"exp":  time.Now().Add(1 * time.Hour).Unix(),
	})

	req := httptest.NewRequest(http.MethodGet, "/admin/settings", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
	if !called {
		t.Error("expected handler to be called")
	}
}

func TestJWTMiddleware_ProtectedRouteExpiredToken(t *testing.T) {
	logger := testLogger()
	mw := Middleware(testSecret, []string{"/admin/"}, logger)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called for expired token")
	}))

	tokenStr := generateTestToken(t, testSecret, jwt.MapClaims{
		"sub": "admin-user",
		"exp": time.Now().Add(-1 * time.Hour).Unix(),
	})

	req := httptest.NewRequest(http.MethodGet, "/admin/settings", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rec.Code)
	}
}

func TestJWTMiddleware_UnprotectedRouteNoToken(t *testing.T) {
	logger := testLogger()
	mw := Middleware(testSecret, []string{"/admin/"}, logger)

	called := false
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/pool/stats", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
	if !called {
		t.Error("expected handler to be called for unprotected route")
	}
}

func TestJWTMiddleware_WrongSigningMethod(t *testing.T) {
	logger := testLogger()
	mw := Middleware(testSecret, []string{"/admin/"}, logger)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called for wrong signing method")
	}))

	// Create a token with a different secret.
	tokenStr := generateTestToken(t, "wrong-secret", jwt.MapClaims{
		"sub": "admin-user",
		"exp": time.Now().Add(1 * time.Hour).Unix(),
	})

	req := httptest.NewRequest(http.MethodGet, "/admin/settings", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rec.Code)
	}
}

func TestJWTMiddleware_InvalidAuthHeaderFormat(t *testing.T) {
	logger := testLogger()
	mw := Middleware(testSecret, []string{"/admin/"}, logger)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called for invalid auth header")
	}))

	req := httptest.NewRequest(http.MethodGet, "/admin/settings", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rec.Code)
	}
}
