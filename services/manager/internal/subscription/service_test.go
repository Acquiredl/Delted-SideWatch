package subscription

import (
	"testing"
)

func TestHashAPIKey(t *testing.T) {
	tests := []struct {
		name string
		key  string
	}{
		{name: "standard key", key: "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"},
		{name: "short key", key: "abc"},
		{name: "empty key", key: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash := HashAPIKey(tt.key)

			// SHA-256 produces 64 hex characters.
			if len(hash) != 64 {
				t.Errorf("hash length = %d, want 64", len(hash))
			}

			// Same input produces same hash (deterministic).
			hash2 := HashAPIKey(tt.key)
			if hash != hash2 {
				t.Error("same input produced different hashes")
			}
		})
	}

	// Different inputs produce different hashes.
	h1 := HashAPIKey("key1")
	h2 := HashAPIKey("key2")
	if h1 == h2 {
		t.Error("different inputs produced same hash")
	}
}

func TestHashAPIKeyIsHex(t *testing.T) {
	hash := HashAPIKey("test-api-key")
	for i, c := range hash {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			t.Errorf("hash[%d] = %c, not valid hex", i, c)
		}
	}
}
