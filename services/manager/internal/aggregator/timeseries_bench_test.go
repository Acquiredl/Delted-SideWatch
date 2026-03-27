package aggregator

import (
	"testing"
	"time"
)

func BenchmarkTruncateToBucket(b *testing.B) {
	t := time.Date(2026, 3, 27, 14, 37, 42, 123456789, time.UTC)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		TruncateToBucket(t)
	}
}

func BenchmarkCalculateHashrate(b *testing.B) {
	difficulties := []uint64{
		0,
		150_000_000,
		300_000_000_000,
		999_999_999_999,
	}
	for _, d := range difficulties {
		b.Run("", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				CalculateHashrate(d)
			}
		})
	}
}
