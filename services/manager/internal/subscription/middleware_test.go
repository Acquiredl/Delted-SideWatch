package subscription

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTierFromContext(t *testing.T) {
	tests := []struct {
		name string
		ctx  context.Context
		want Tier
	}{
		{
			name: "supporter tier in context",
			ctx:  context.WithValue(context.Background(), tierKey, TierSupporter),
			want: TierSupporter,
		},
		{
			name: "champion tier in context",
			ctx:  context.WithValue(context.Background(), tierKey, TierChampion),
			want: TierChampion,
		},
		{
			name: "free tier in context",
			ctx:  context.WithValue(context.Background(), tierKey, TierFree),
			want: TierFree,
		},
		{
			name: "no tier in context defaults to free",
			ctx:  context.Background(),
			want: TierFree,
		},
		{
			name: "wrong type in context defaults to free",
			ctx:  context.WithValue(context.Background(), tierKey, "not-a-tier"),
			want: TierFree,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TierFromContext(tt.ctx)
			if got != tt.want {
				t.Errorf("TierFromContext() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAddressFromContext(t *testing.T) {
	tests := []struct {
		name string
		ctx  context.Context
		want string
	}{
		{
			name: "address in context",
			ctx:  context.WithValue(context.Background(), addressKey, "4ABC123addr"),
			want: "4ABC123addr",
		},
		{
			name: "no address in context",
			ctx:  context.Background(),
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AddressFromContext(tt.ctx)
			if got != tt.want {
				t.Errorf("AddressFromContext() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRequireTier(t *testing.T) {
	tests := []struct {
		name       string
		minTier    Tier
		actual     Tier
		wantStatus int
	}{
		{
			name:       "supporter passes supporter gate",
			minTier:    TierSupporter,
			actual:     TierSupporter,
			wantStatus: http.StatusOK,
		},
		{
			name:       "champion passes supporter gate",
			minTier:    TierSupporter,
			actual:     TierChampion,
			wantStatus: http.StatusOK,
		},
		{
			name:       "free gets 403 on supporter gate",
			minTier:    TierSupporter,
			actual:     TierFree,
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "champion passes champion gate",
			minTier:    TierChampion,
			actual:     TierChampion,
			wantStatus: http.StatusOK,
		},
		{
			name:       "supporter gets 403 on champion gate",
			minTier:    TierChampion,
			actual:     TierSupporter,
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "free gets 403 on champion gate",
			minTier:    TierChampion,
			actual:     TierFree,
			wantStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			handler := RequireTier(tt.minTier, nil)(inner)

			req := httptest.NewRequest("GET", "/test", http.NoBody)
			req = req.WithContext(context.WithValue(req.Context(), tierKey, tt.actual))
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}

func TestEffectiveHashrateHours(t *testing.T) {
	limits := DefaultFreeLimits()

	tests := []struct {
		name      string
		tier      Tier
		requested int
		want      int
	}{
		{name: "free within limit", tier: TierFree, requested: 24, want: 24},
		{name: "free at limit", tier: TierFree, requested: 720, want: 720},
		{name: "free over limit", tier: TierFree, requested: 2000, want: 720},
		{name: "supporter no cap", tier: TierSupporter, requested: 2000, want: 2000},
		{name: "supporter small request", tier: TierSupporter, requested: 24, want: 24},
		{name: "champion no cap", tier: TierChampion, requested: 2000, want: 2000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EffectiveHashrateHours(tt.tier, tt.requested, limits)
			if got != tt.want {
				t.Errorf("EffectiveHashrateHours(%q, %d) = %d, want %d", tt.tier, tt.requested, got, tt.want)
			}
		})
	}
}

func TestEffectivePaymentLimit(t *testing.T) {
	limits := DefaultFreeLimits()

	tests := []struct {
		name      string
		tier      Tier
		requested int
		want      int
	}{
		{name: "free within limit", tier: TierFree, requested: 50, want: 50},
		{name: "free at limit", tier: TierFree, requested: 100, want: 100},
		{name: "free over limit", tier: TierFree, requested: 500, want: 100},
		{name: "supporter no cap", tier: TierSupporter, requested: 500, want: 500},
		{name: "champion no cap", tier: TierChampion, requested: 500, want: 500},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EffectivePaymentLimit(tt.tier, tt.requested, limits)
			if got != tt.want {
				t.Errorf("EffectivePaymentLimit(%q, %d) = %d, want %d", tt.tier, tt.requested, got, tt.want)
			}
		})
	}
}

func TestDefaultFreeLimits(t *testing.T) {
	limits := DefaultFreeLimits()
	if limits.MaxHashrateHours != 720 {
		t.Errorf("MaxHashrateHours = %d, want 720", limits.MaxHashrateHours)
	}
	if limits.MaxPayments != 100 {
		t.Errorf("MaxPayments = %d, want 100", limits.MaxPayments)
	}
}
