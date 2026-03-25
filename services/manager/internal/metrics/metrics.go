package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// Pool metrics.
	PoolHashrate = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "p2pool_pool_hashrate",
		Help: "Current pool hashrate in H/s",
	})
	PoolMiners = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "p2pool_pool_miners",
		Help: "Number of active miners",
	})
	BlocksFound = promauto.NewCounter(prometheus.CounterOpts{
		Name: "p2pool_blocks_found_total",
		Help: "Total blocks found by the pool",
	})

	// Indexer metrics.
	IndexerPollDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "p2pool_indexer_poll_duration_seconds",
		Help:    "Duration of indexer poll cycles",
		Buckets: prometheus.DefBuckets,
	})
	IndexerErrors = promauto.NewCounter(prometheus.CounterOpts{
		Name: "p2pool_indexer_errors_total",
		Help: "Total indexer errors",
	})

	// Scanner metrics.
	PaymentsRecorded = promauto.NewCounter(prometheus.CounterOpts{
		Name: "p2pool_payments_recorded_total",
		Help: "Total payments recorded",
	})
	PendingBlocks = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "p2pool_pending_blocks",
		Help: "Blocks awaiting confirmation",
	})

	// API metrics.
	HTTPRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "p2pool_http_request_duration_seconds",
		Help:    "HTTP request duration",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "path", "status"})
	HTTPRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "p2pool_http_requests_total",
		Help: "Total HTTP requests",
	}, []string{"method", "path", "status"})
)

// Handler returns the Prometheus metrics HTTP handler.
func Handler() http.Handler {
	return promhttp.Handler()
}
