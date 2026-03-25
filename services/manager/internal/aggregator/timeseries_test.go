package aggregator

import (
	"testing"
	"time"
)

func TestTruncateToBucket(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Time
		expected time.Time
	}{
		{
			name:     "exactly on boundary 14:00",
			input:    time.Date(2025, 6, 15, 14, 0, 0, 0, time.UTC),
			expected: time.Date(2025, 6, 15, 14, 0, 0, 0, time.UTC),
		},
		{
			name:     "14:07 truncates to 14:00",
			input:    time.Date(2025, 6, 15, 14, 7, 30, 0, time.UTC),
			expected: time.Date(2025, 6, 15, 14, 0, 0, 0, time.UTC),
		},
		{
			name:     "14:14:59 truncates to 14:00",
			input:    time.Date(2025, 6, 15, 14, 14, 59, 0, time.UTC),
			expected: time.Date(2025, 6, 15, 14, 0, 0, 0, time.UTC),
		},
		{
			name:     "14:15 truncates to 14:15",
			input:    time.Date(2025, 6, 15, 14, 15, 0, 0, time.UTC),
			expected: time.Date(2025, 6, 15, 14, 15, 0, 0, time.UTC),
		},
		{
			name:     "14:16 truncates to 14:15",
			input:    time.Date(2025, 6, 15, 14, 16, 45, 0, time.UTC),
			expected: time.Date(2025, 6, 15, 14, 15, 0, 0, time.UTC),
		},
		{
			name:     "14:31 truncates to 14:30",
			input:    time.Date(2025, 6, 15, 14, 31, 12, 0, time.UTC),
			expected: time.Date(2025, 6, 15, 14, 30, 0, 0, time.UTC),
		},
		{
			name:     "14:46 truncates to 14:45",
			input:    time.Date(2025, 6, 15, 14, 46, 0, 0, time.UTC),
			expected: time.Date(2025, 6, 15, 14, 45, 0, 0, time.UTC),
		},
		{
			name:     "14:59:59 truncates to 14:45",
			input:    time.Date(2025, 6, 15, 14, 59, 59, 0, time.UTC),
			expected: time.Date(2025, 6, 15, 14, 45, 0, 0, time.UTC),
		},
		{
			name:     "midnight 00:00 stays at 00:00",
			input:    time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC),
			expected: time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "23:59 truncates to 23:45",
			input:    time.Date(2025, 6, 15, 23, 59, 59, 0, time.UTC),
			expected: time.Date(2025, 6, 15, 23, 45, 0, 0, time.UTC),
		},
		{
			name:     "seconds and nanoseconds are zeroed",
			input:    time.Date(2025, 6, 15, 10, 32, 47, 123456789, time.UTC),
			expected: time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC),
		},
		{
			name:     "non-UTC timezone preserved",
			input:    time.Date(2025, 6, 15, 14, 22, 0, 0, time.FixedZone("EST", -5*3600)),
			expected: time.Date(2025, 6, 15, 14, 15, 0, 0, time.FixedZone("EST", -5*3600)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TruncateToBucket(tt.input)
			if !got.Equal(tt.expected) {
				t.Errorf("TruncateToBucket(%v) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestCalculateHashrate(t *testing.T) {
	tests := []struct {
		name            string
		totalDifficulty uint64
		expected        uint64
	}{
		{
			name:            "zero difficulty yields zero hashrate",
			totalDifficulty: 0,
			expected:        0,
		},
		{
			name:            "900000 difficulty over 900s = 1000 H/s",
			totalDifficulty: 900000,
			expected:        1000,
		},
		{
			name:            "900 difficulty over 900s = 1 H/s",
			totalDifficulty: 900,
			expected:        1,
		},
		{
			name:            "450 difficulty rounds down to 0 H/s (integer division)",
			totalDifficulty: 450,
			expected:        0,
		},
		{
			name:            "large difficulty value",
			totalDifficulty: 9000000000, // 9 billion
			expected:        10000000,   // 10 MH/s
		},
		{
			name:            "exact bucket boundary",
			totalDifficulty: 1800,
			expected:        2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateHashrate(tt.totalDifficulty)
			if got != tt.expected {
				t.Errorf("CalculateHashrate(%d) = %d, want %d", tt.totalDifficulty, got, tt.expected)
			}
		})
	}
}
