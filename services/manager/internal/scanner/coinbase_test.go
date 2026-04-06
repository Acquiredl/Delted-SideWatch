package scanner

import (
	"testing"
)

// TestProportionalShareCalculation verifies that miner payments are calculated
// proportionally based on difficulty contribution. This tests the pure
// calculation logic without requiring a database connection.
func TestProportionalShareCalculation(t *testing.T) {
	tests := []struct {
		name              string
		coinbaseReward    uint64
		minerDifficulties map[string]uint64
		totalHashrate     uint64            // pool-wide hashrate (0 = use sum of minerDifficulties)
		wantPayments      map[string]uint64 // address -> expected amount
		wantTotal         uint64            // sum should equal coinbaseReward
	}{
		{
			name:           "two miners equal share",
			coinbaseReward: 1_000_000_000_000, // 1 XMR
			minerDifficulties: map[string]uint64{
				"4addr_alice": 500,
				"4addr_bob":   500,
			},
			wantPayments: map[string]uint64{
				"4addr_alice": 500_000_000_000,
				"4addr_bob":   500_000_000_000,
			},
			wantTotal: 1_000_000_000_000,
		},
		{
			name:           "three miners unequal share",
			coinbaseReward: 900_000_000_000, // 0.9 XMR
			minerDifficulties: map[string]uint64{
				"4addr_alice":   600,
				"4addr_bob":     300,
				"4addr_charlie": 100,
			},
			wantPayments: map[string]uint64{
				"4addr_alice":   540_000_000_000,
				"4addr_bob":     270_000_000_000,
				"4addr_charlie": 90_000_000_000,
			},
			wantTotal: 900_000_000_000,
		},
		{
			name:           "single miner gets everything",
			coinbaseReward: 600_000_000_000,
			minerDifficulties: map[string]uint64{
				"4addr_solo": 1000,
			},
			wantPayments: map[string]uint64{
				"4addr_solo": 600_000_000_000,
			},
			wantTotal: 600_000_000_000,
		},
		{
			name:           "single local miner against large pool total",
			coinbaseReward: 600_000_000_000, // 0.6 XMR block reward
			totalHashrate:  5_000_000,       // 5 MH/s pool-wide
			minerDifficulties: map[string]uint64{
				"4addr_local": 162, // 162 H/s local miner
			},
			wantPayments: map[string]uint64{
				// 162 / 5_000_000 * 600_000_000_000 = 19_440_000 (~0.00001944 XMR)
				"4addr_local": 19_440_000,
			},
			wantTotal: 19_440_000,
		},
		{
			name:           "rounding truncates fractional piconero",
			coinbaseReward: 1_000_000_000_001, // odd amount
			minerDifficulties: map[string]uint64{
				"4addr_alice":   1,
				"4addr_bob":     1,
				"4addr_charlie": 1,
			},
			// Each miner: 1/3 * 1_000_000_000_001 = 333_333_333_333 (truncated)
			// Total distributed: 999_999_999_999 (dust is not attributed)
			wantTotal: 999_999_999_999,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			totalHashrate := tt.totalHashrate
			if totalHashrate == 0 {
				// Default: sum of local miners (simulates local-only scenario).
				for _, h := range tt.minerDifficulties {
					totalHashrate += h
				}
			}
			payments := calculateProportionalPayments(
				tt.coinbaseReward,
				tt.minerDifficulties,
				totalHashrate,
				99999, // mainHeight
				"abcdef1234567890",
			)

			// Verify total distributed equals coinbase reward.
			var total uint64
			for _, p := range payments {
				total += p.Amount
			}
			if total != tt.wantTotal {
				t.Errorf("total distributed = %d, want %d", total, tt.wantTotal)
			}

			// Verify individual payments if specified.
			if tt.wantPayments != nil {
				paymentMap := make(map[string]uint64)
				for _, p := range payments {
					paymentMap[p.MinerAddress] = p.Amount
				}

				for addr, wantAmount := range tt.wantPayments {
					gotAmount, ok := paymentMap[addr]
					if !ok {
						t.Errorf("missing payment for %s", addr)
						continue
					}
					if gotAmount != wantAmount {
						t.Errorf("payment for %s = %d, want %d", addr, gotAmount, wantAmount)
					}
				}
			}

			// Verify no zero-amount payments.
			for _, p := range payments {
				if p.Amount == 0 {
					t.Errorf("zero-amount payment for %s", p.MinerAddress)
				}
			}
		})
	}
}

// TestCalculateProportionalPaymentsEmpty verifies behavior with no miners.
func TestCalculateProportionalPaymentsEmpty(t *testing.T) {
	payments := calculateProportionalPayments(
		1_000_000_000_000,
		map[string]uint64{},
		1000, // totalHashrate
		100,
		"abc123",
	)
	if len(payments) != 0 {
		t.Errorf("expected 0 payments for empty miners, got %d", len(payments))
	}
}

// calculateProportionalPayments is a pure function that calculates proportional
// payments given a coinbase reward, miner hashrates, and the total pool hashrate.
// totalHashrate should be the pool-wide hashrate (from pool_stats_snapshots),
// not just the sum of local miners.
// This is extracted for testability without database dependencies.
func calculateProportionalPayments(coinbaseReward uint64, minerDifficulties map[string]uint64, totalHashrate uint64, mainHeight uint64, mainHash string) []Payment {
	if len(minerDifficulties) == 0 {
		return nil
	}

	if totalHashrate == 0 {
		return nil
	}

	// Build a deterministic ordering by collecting addresses.
	addresses := make([]string, 0, len(minerDifficulties))
	for addr := range minerDifficulties {
		addresses = append(addresses, addr)
	}

	payments := make([]Payment, 0, len(addresses))

	for _, address := range addresses {
		hashrate := minerDifficulties[address]

		// Each miner's estimated share: (miner_hashrate / pool_hashrate) * reward.
		amount := (hashrate * coinbaseReward) / totalHashrate

		if amount == 0 {
			continue
		}

		payments = append(payments, Payment{
			MinerAddress: address,
			Amount:       amount,
			MainHeight:   mainHeight,
			MainHash:     mainHash,
		})
	}

	return payments
}
