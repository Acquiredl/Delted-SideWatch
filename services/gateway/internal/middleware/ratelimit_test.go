package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExtractIP_XForwardedFor(t *testing.T) {
	tests := []struct {
		name     string
		xff      string
		xri      string
		remote   string
		expected string
	}{
		{
			name:     "single IP in X-Forwarded-For",
			xff:      "203.0.113.50",
			expected: "203.0.113.50",
		},
		{
			name:     "multiple IPs in X-Forwarded-For takes rightmost",
			xff:      "203.0.113.50, 70.41.3.18, 150.172.238.178",
			expected: "150.172.238.178",
		},
		{
			name:     "X-Real-IP preferred",
			xri:      "198.51.100.1",
			expected: "198.51.100.1",
		},
		{
			name:     "RemoteAddr fallback with port",
			remote:   "192.168.1.1:12345",
			expected: "192.168.1.1",
		},
		{
			name:     "RemoteAddr fallback without port",
			remote:   "192.168.1.1",
			expected: "192.168.1.1",
		},
		{
			name:     "X-Real-IP takes precedence over X-Forwarded-For",
			xff:      "203.0.113.50",
			xri:      "198.51.100.1",
			remote:   "192.168.1.1:12345",
			expected: "198.51.100.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.xff != "" {
				req.Header.Set("X-Forwarded-For", tt.xff)
			}
			if tt.xri != "" {
				req.Header.Set("X-Real-IP", tt.xri)
			}
			if tt.remote != "" {
				req.RemoteAddr = tt.remote
			}

			got := extractIP(req)
			if got != tt.expected {
				t.Errorf("extractIP() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestRequestID_Generated(t *testing.T) {
	called := false
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			t.Error("expected X-Request-ID to be set")
		}
		if len(id) != 32 {
			t.Errorf("expected 32-char hex ID, got %d chars: %s", len(id), id)
		}
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("expected handler to be called")
	}

	// Response should also have the ID.
	respID := rec.Header().Get("X-Request-ID")
	if respID == "" {
		t.Error("expected X-Request-ID in response")
	}
}

func TestRequestID_Preserved(t *testing.T) {
	existingID := "my-custom-request-id"

	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id != existingID {
			t.Errorf("expected preserved ID %q, got %q", existingID, id)
		}
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", existingID)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	respID := rec.Header().Get("X-Request-ID")
	if respID != existingID {
		t.Errorf("expected response ID %q, got %q", existingID, respID)
	}
}
