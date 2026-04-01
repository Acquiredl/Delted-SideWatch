package fund

import "testing"

func TestPercentFunded(t *testing.T) {
	tests := []struct {
		name   string
		funded float64
		goal   float64
		want   int
	}{
		{name: "zero of 150", funded: 0, goal: 150, want: 0},
		{name: "75 of 150", funded: 75, goal: 150, want: 50},
		{name: "109.50 of 150", funded: 109.50, goal: 150, want: 73},
		{name: "150 of 150", funded: 150, goal: 150, want: 100},
		{name: "200 of 150 (over-funded)", funded: 200, goal: 150, want: 133},
		{name: "zero goal returns 0", funded: 50, goal: 0, want: 0},
		{name: "negative goal returns 0", funded: 50, goal: -10, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PercentFunded(tt.funded, tt.goal)
			if got != tt.want {
				t.Errorf("PercentFunded(%f, %f) = %d, want %d", tt.funded, tt.goal, got, tt.want)
			}
		})
	}
}

func TestTruncateAddress(t *testing.T) {
	tests := []struct {
		name string
		addr string
		want string
	}{
		{
			name: "standard monero address",
			addr: "4A8xN3q5pR7sKfWm2bC9jLd6eVnZ1yQ8tGhF",
			want: "4A8x...tGhF",
		},
		{
			name: "short address returned as-is",
			addr: "4A8x3fKq",
			want: "4A8x3fKq",
		},
		{
			name: "exactly 12 chars returned as-is",
			addr: "4A8x3fKq7gVp",
			want: "4A8x3fKq7gVp",
		},
		{
			name: "13 chars gets truncated",
			addr: "4A8x3fKq7gVpX",
			want: "4A8x...gVpX",
		},
		{
			name: "empty string",
			addr: "",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TruncateAddress(tt.addr)
			if got != tt.want {
				t.Errorf("TruncateAddress(%q) = %q, want %q", tt.addr, got, tt.want)
			}
		})
	}
}

func TestFundStatusFields(t *testing.T) {
	status := FundStatus{
		Month:          "2026-04",
		GoalUSD:        150.00,
		FundedUSD:      109.50,
		PercentFunded:  73,
		InfraCostUSD:   89.00,
		SupporterCount: 37,
		NodeCount:      2,
	}

	if status.Month != "2026-04" {
		t.Errorf("Month = %q, want \"2026-04\"", status.Month)
	}
	if status.PercentFunded != 73 {
		t.Errorf("PercentFunded = %d, want 73", status.PercentFunded)
	}
	if status.NodeCount != 2 {
		t.Errorf("NodeCount = %d, want 2", status.NodeCount)
	}
}
