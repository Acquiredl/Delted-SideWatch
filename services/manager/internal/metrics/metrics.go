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

	AlertsReceived = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "p2pool",
		Name:      "alerts_received_total",
		Help:      "Total alerts received from Alertmanager",
	}, []string{"alertname", "status"})

	// Per-miner metrics (exported by aggregator and scanner).
	//
	// Cardinality note: P2Pool mini typically has ~2,000-5,000 active miners.
	// This is manageable for Prometheus, but monitor series count if the pool
	// grows significantly. Stale miner series are naturally pruned by Prometheus
	// staleness rules (5 min without update).
	MinerHashrate = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "p2pool",
		Name:      "miner_hashrate",
		Help:      "Current hashrate per miner in H/s",
	}, []string{"miner_address", "sidechain"})

	MinerShares = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "p2pool",
		Name:      "miner_shares_total",
		Help:      "Total shares submitted per miner in current PPLNS window",
	}, []string{"miner_address", "sidechain"})

	MinerPaymentsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "p2pool",
		Name:      "miner_payments_total",
		Help:      "Total number of payments to a miner",
	}, []string{"miner_address"})

	MinerLastPaymentTimestamp = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "p2pool",
		Name:      "miner_last_payment_timestamp",
		Help:      "Unix timestamp of last payment to miner",
	}, []string{"miner_address"})

	MinerPaidTotal = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "p2pool",
		Name:      "miner_paid_atomic_total",
		Help:      "Total amount paid to miner in atomic units",
	}, []string{"miner_address"})

	// Shared node pool metrics.
	// Namespace: "sidewatch" — new SideWatch-specific metrics use this namespace
	// to distinguish from legacy "p2pool" pool/miner metrics above.
	NodeHealthStatus = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "sidewatch",
		Name:      "node_health_status",
		Help:      "Health status of shared node (1=healthy, 0=unhealthy)",
	}, []string{"name", "sidechain"})

	NodeHashrate = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "sidewatch",
		Name:      "node_hashrate",
		Help:      "Total hashrate reported by shared node in H/s",
	}, []string{"name", "sidechain"})

	NodeMiners = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "sidewatch",
		Name:      "node_miners",
		Help:      "Number of miners connected to shared node",
	}, []string{"name", "sidechain"})

	// Fund metrics.
	FundPercentFunded = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "sidewatch",
		Name:      "fund_percent_funded",
		Help:      "Current month funding percentage (0-100+)",
	})

	FundSupporterCount = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "sidewatch",
		Name:      "fund_supporter_count",
		Help:      "Number of unique contributors this month",
	})
)

// Handler returns the Prometheus metrics HTTP handler.
func Handler() http.Handler {
	return promhttp.Handler()
}
