package subscription

import (
	"testing"
	"time"
)

func TestMeetsMinimum(t *testing.T) {
	s := &Scanner{minUSD: DefaultMinUSD} // $4 minimum

	tests := []struct {
		name         string
		amountAtomic uint64
		usdPrice     *float64
		want         bool
	}{
		{
			name:         "exact $5 payment at $150/XMR",
			amountAtomic: 33333333333, // ~0.0333 XMR * $150 = $5
			usdPrice:     ptrFloat64(150.0),
			want:         true,
		},
		{
			name:         "$4 payment at $150/XMR (20% tolerance)",
			amountAtomic: 26700000000, // ~0.0267 XMR * $150 = $4.005
			usdPrice:     ptrFloat64(150.0),
			want:         true,
		},
		{
			name:         "below minimum $3 at $150/XMR",
			amountAtomic: 20000000000, // 0.02 XMR * $150 = $3
			usdPrice:     ptrFloat64(150.0),
			want:         false,
		},
		{
			name:         "nil price accepts payment",
			amountAtomic: 1000000000, // small amount
			usdPrice:     nil,
			want:         true,
		},
		{
			name:         "zero price accepts payment",
			amountAtomic: 1000000000,
			usdPrice:     ptrFloat64(0),
			want:         true,
		},
		{
			name:         "large payment easily meets minimum",
			amountAtomic: 1000000000000, // 1 XMR * $150 = $150
			usdPrice:     ptrFloat64(150.0),
			want:         true,
		},
		{
			name:         "1 piconero is not enough",
			amountAtomic: 1,
			usdPrice:     ptrFloat64(150.0),
			want:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.meetsMinimum(tt.amountAtomic, tt.usdPrice)
			if got != tt.want {
				t.Errorf("meetsMinimum(%d, %v) = %v, want %v", tt.amountAtomic, tt.usdPrice, got, tt.want)
			}
		})
	}
}

func TestSubscriptionIsActive(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name string
		sub  Subscription
		want bool
	}{
		{
			name: "active paid subscription",
			sub: Subscription{
				Tier:       TierPaid,
				GraceUntil: ptrTime(now.Add(24 * time.Hour)),
			},
			want: true,
		},
		{
			name: "expired paid subscription (past grace)",
			sub: Subscription{
				Tier:       TierPaid,
				GraceUntil: ptrTime(now.Add(-1 * time.Hour)),
			},
			want: false,
		},
		{
			name: "free tier is never active",
			sub: Subscription{
				Tier:       TierFree,
				GraceUntil: ptrTime(now.Add(24 * time.Hour)),
			},
			want: false,
		},
		{
			name: "paid tier with nil grace is not active",
			sub: Subscription{
				Tier:       TierPaid,
				GraceUntil: nil,
			},
			want: false,
		},
		{
			name: "in grace period (expired but within 48h)",
			sub: Subscription{
				Tier:       TierPaid,
				ExpiresAt:  ptrTime(now.Add(-1 * time.Hour)),
				GraceUntil: ptrTime(now.Add(47 * time.Hour)),
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.sub.IsActive()
			if got != tt.want {
				t.Errorf("IsActive() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGracePeriodCalculation(t *testing.T) {
	now := time.Now()
	durationDays := 30
	graceHours := 48

	expiresAt := now.Add(time.Duration(durationDays) * 24 * time.Hour)
	graceUntil := expiresAt.Add(time.Duration(graceHours) * time.Hour)

	// Verify grace is 48h after expiry.
	diff := graceUntil.Sub(expiresAt)
	if diff != 48*time.Hour {
		t.Errorf("grace period = %v, want 48h", diff)
	}

	// Verify subscription is 30 days.
	subDuration := expiresAt.Sub(now)
	if subDuration < 29*24*time.Hour || subDuration > 31*24*time.Hour {
		t.Errorf("subscription duration = %v, want ~30 days", subDuration)
	}
}

func TestTierConstants(t *testing.T) {
	if TierFree != "free" {
		t.Errorf("TierFree = %q, want \"free\"", TierFree)
	}
	if TierPaid != "paid" {
		t.Errorf("TierPaid = %q, want \"paid\"", TierPaid)
	}
	if DefaultMinUSD != 4.0 {
		t.Errorf("DefaultMinUSD = %f, want 4.0", DefaultMinUSD)
	}
	if DefaultSubscriptionDays != 30 {
		t.Errorf("DefaultSubscriptionDays = %d, want 30", DefaultSubscriptionDays)
	}
	if DefaultGraceHours != 48 {
		t.Errorf("DefaultGraceHours = %d, want 48", DefaultGraceHours)
	}
}

func ptrFloat64(v float64) *float64 { return &v }
func ptrTime(v time.Time) *time.Time { return &v }
