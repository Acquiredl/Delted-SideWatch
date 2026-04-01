package subscription

import (
	"testing"
	"time"
)

func TestPaymentUSDValue(t *testing.T) {
	s := &Scanner{minUSD: DefaultMinUSD}

	tests := []struct {
		name         string
		amountAtomic uint64
		usdPrice     *float64
		wantApprox   float64
	}{
		{
			name:         "$5 payment at $150/XMR",
			amountAtomic: 33333333333, // ~0.0333 XMR * $150 = $5
			usdPrice:     ptrFloat64(150.0),
			wantApprox:   5.0,
		},
		{
			name:         "nil price returns supporter minimum",
			amountAtomic: 1000000000,
			usdPrice:     nil,
			wantApprox:   SupporterMinUSD,
		},
		{
			name:         "zero price returns supporter minimum",
			amountAtomic: 1000000000,
			usdPrice:     ptrFloat64(0),
			wantApprox:   SupporterMinUSD,
		},
		{
			name:         "1 XMR at $150",
			amountAtomic: 1000000000000,
			usdPrice:     ptrFloat64(150.0),
			wantApprox:   150.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.paymentUSDValue(tt.amountAtomic, tt.usdPrice)
			if got < tt.wantApprox*0.99 || got > tt.wantApprox*1.01 {
				t.Errorf("paymentUSDValue(%d, %v) = %.4f, want ~%.4f", tt.amountAtomic, tt.usdPrice, got, tt.wantApprox)
			}
		})
	}
}

func TestTierForAmount(t *testing.T) {
	tests := []struct {
		name     string
		usdValue float64
		want     Tier
	}{
		{name: "zero gets free", usdValue: 0, want: TierFree},
		{name: "below supporter min gets free", usdValue: 0.50, want: TierFree},
		{name: "at supporter min gets supporter", usdValue: SupporterMinUSD, want: TierSupporter},
		{name: "$1 gets supporter", usdValue: 1.0, want: TierSupporter},
		{name: "$3 gets supporter", usdValue: 3.0, want: TierSupporter},
		{name: "$3.99 gets supporter", usdValue: 3.99, want: TierSupporter},
		{name: "at champion min gets champion", usdValue: ChampionMinUSD, want: TierChampion},
		{name: "$5 gets champion", usdValue: 5.0, want: TierChampion},
		{name: "$20 gets champion", usdValue: 20.0, want: TierChampion},
		{name: "$150 gets champion", usdValue: 150.0, want: TierChampion},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TierForAmount(tt.usdValue)
			if got != tt.want {
				t.Errorf("TierForAmount(%.2f) = %q, want %q", tt.usdValue, got, tt.want)
			}
		})
	}
}

func TestTierIncludes(t *testing.T) {
	tests := []struct {
		name     string
		actual   Tier
		required Tier
		want     bool
	}{
		{name: "free includes free", actual: TierFree, required: TierFree, want: true},
		{name: "free does not include supporter", actual: TierFree, required: TierSupporter, want: false},
		{name: "free does not include champion", actual: TierFree, required: TierChampion, want: false},
		{name: "supporter includes free", actual: TierSupporter, required: TierFree, want: true},
		{name: "supporter includes supporter", actual: TierSupporter, required: TierSupporter, want: true},
		{name: "supporter does not include champion", actual: TierSupporter, required: TierChampion, want: false},
		{name: "champion includes free", actual: TierChampion, required: TierFree, want: true},
		{name: "champion includes supporter", actual: TierChampion, required: TierSupporter, want: true},
		{name: "champion includes champion", actual: TierChampion, required: TierChampion, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TierIncludes(tt.actual, tt.required)
			if got != tt.want {
				t.Errorf("TierIncludes(%q, %q) = %v, want %v", tt.actual, tt.required, got, tt.want)
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
			name: "active supporter subscription",
			sub: Subscription{
				Tier:       TierSupporter,
				GraceUntil: ptrTime(now.Add(24 * time.Hour)),
			},
			want: true,
		},
		{
			name: "active champion subscription",
			sub: Subscription{
				Tier:       TierChampion,
				GraceUntil: ptrTime(now.Add(24 * time.Hour)),
			},
			want: true,
		},
		{
			name: "expired supporter subscription (past grace)",
			sub: Subscription{
				Tier:       TierSupporter,
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
			name: "supporter tier with nil grace is not active",
			sub: Subscription{
				Tier:       TierSupporter,
				GraceUntil: nil,
			},
			want: false,
		},
		{
			name: "in grace period (expired but within 48h)",
			sub: Subscription{
				Tier:       TierSupporter,
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
	if TierSupporter != "supporter" {
		t.Errorf("TierSupporter = %q, want \"supporter\"", TierSupporter)
	}
	if TierChampion != "champion" {
		t.Errorf("TierChampion = %q, want \"champion\"", TierChampion)
	}
	if SupporterMinUSD != 0.80 {
		t.Errorf("SupporterMinUSD = %f, want 0.80", SupporterMinUSD)
	}
	if ChampionMinUSD != 4.00 {
		t.Errorf("ChampionMinUSD = %f, want 4.00", ChampionMinUSD)
	}
	if DefaultMinUSD != SupporterMinUSD {
		t.Errorf("DefaultMinUSD = %f, want %f (SupporterMinUSD)", DefaultMinUSD, SupporterMinUSD)
	}
	if DefaultSubscriptionDays != 30 {
		t.Errorf("DefaultSubscriptionDays = %d, want 30", DefaultSubscriptionDays)
	}
	if DefaultGraceHours != 48 {
		t.Errorf("DefaultGraceHours = %d, want 48", DefaultGraceHours)
	}
}

func ptrFloat64(v float64) *float64  { return &v }
func ptrTime(v time.Time) *time.Time { return &v }
