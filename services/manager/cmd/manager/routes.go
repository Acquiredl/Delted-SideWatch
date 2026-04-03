package main

import (
	"crypto/subtle"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/acquiredl/xmr-p2pool-dashboard/services/manager/internal/aggregator"
	"github.com/acquiredl/xmr-p2pool-dashboard/services/manager/internal/cache"
	"github.com/acquiredl/xmr-p2pool-dashboard/services/manager/internal/fund"
	"github.com/acquiredl/xmr-p2pool-dashboard/services/manager/internal/metrics"
	"github.com/acquiredl/xmr-p2pool-dashboard/services/manager/internal/nodepool"
	"github.com/acquiredl/xmr-p2pool-dashboard/services/manager/internal/p2pool"
	"github.com/acquiredl/xmr-p2pool-dashboard/services/manager/internal/scanner"
	"github.com/acquiredl/xmr-p2pool-dashboard/services/manager/internal/subscription"
	"github.com/acquiredl/xmr-p2pool-dashboard/services/manager/internal/ws"
	"github.com/acquiredl/xmr-p2pool-dashboard/services/manager/pkg/p2poolclient"
)

// RegisterRoutes wires all HTTP routes onto the provided ServeMux.
func RegisterRoutes(mux *http.ServeMux, pool *pgxpool.Pool, agg *aggregator.Aggregator, cacheStore *cache.Store, hub *ws.Hub, oracle *scanner.PriceOracle, subSvc *subscription.Service, subScanner *subscription.Scanner, p2poolSvc *p2pool.Service, p2poolClient *p2poolclient.Client, fundSvc *fund.Service, nodePoolSvc *nodepool.Pool, adminToken string) {
	mux.HandleFunc("GET /health", handleHealth(pool, cacheStore, p2poolClient))
	mux.HandleFunc("GET /api/pool/stats", handlePoolStats(agg))
	mux.HandleFunc("GET /api/miner/{address}", handleMinerStats(agg))
	mux.HandleFunc("GET /api/miner/{address}/payments", handleMinerPayments(agg))
	mux.HandleFunc("GET /api/miner/{address}/payment-summary", handleMinerPaymentSummary(agg))
	mux.HandleFunc("GET /api/miner/{address}/hashrate", handleMinerHashrate(agg))
	mux.HandleFunc("GET /api/miners/weekly", handleWeeklyMiners(agg))
	mux.HandleFunc("GET /api/miner/{address}/uncle-rate", handleMinerUncleRate(agg))
	mux.HandleFunc("GET /api/blocks", handleBlocks(agg))
	mux.HandleFunc("GET /api/sidechain/shares", handleSidechainShares(agg))
	mux.HandleFunc("GET /api/workers", handleLocalWorkers(agg))
	mux.HandleFunc("GET /api/sidechain/stats", handleSidechainStats(agg))
	mux.HandleFunc("GET /ws/pool/stats", hub.HandlePoolStats())
	mux.HandleFunc("POST /api/admin/backfill-prices", handleBackfillPrices(pool, oracle, adminToken))
	mux.HandleFunc("GET /api/admin/subscription-income", handleSubscriptionIncome(subSvc, adminToken))
	mux.HandleFunc("POST /api/admin/backfill-sub-prices", handleBackfillSubPrices(pool, oracle, adminToken))
	mux.HandleFunc("POST /api/webhook/alerts", handleAlertWebhook(adminToken))

	// Worker breakdown requires supporter+ subscription.
	mux.Handle("GET /api/miner/{address}/workers",
		subscription.RequireTier(subscription.TierSupporter, slog.Default())(http.HandlerFunc(handleMinerWorkers(agg))))

	// Tax export requires supporter+ subscription.
	mux.Handle("GET /api/miner/{address}/tax-export",
		subscription.RequireTier(subscription.TierSupporter, slog.Default())(http.HandlerFunc(handleTaxExport(agg))))

	// Subscription endpoints.
	mux.HandleFunc("GET /api/subscription/address/{address}", handleSubscriptionAddress(subScanner, oracle))
	mux.HandleFunc("GET /api/subscription/status/{address}", handleSubscriptionStatus(subSvc))
	mux.HandleFunc("GET /api/subscription/payments/{address}", handleSubscriptionPayments(subSvc))
	mux.HandleFunc("POST /api/subscription/api-key/{address}", handleGenerateAPIKey(subSvc))

	// Fund endpoints.
	mux.HandleFunc("GET /api/fund/status", handleFundStatus(fundSvc))
	mux.HandleFunc("GET /api/fund/history", handleFundHistory(fundSvc))
	mux.HandleFunc("GET /api/fund/supporters", handleFundSupporters(fundSvc))

	// Node pool endpoints.
	mux.HandleFunc("GET /api/nodes/status", handleNodesStatus(nodePoolSvc))
	mux.HandleFunc("GET /api/nodes/connection-info", handleNodesConnectionInfo(nodePoolSvc))
}

// writeJSON encodes v as JSON and writes it to the response.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("failed to encode JSON response", "error", err)
	}
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// parsePagination extracts limit and offset query params with defaults and bounds.
func parsePagination(r *http.Request, defaultLimit, maxLimit int) (limit, offset int) {
	limit = defaultLimit
	offset = 0

	if v := r.URL.Query().Get("limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if limit > maxLimit {
		limit = maxLimit
	}

	if v := r.URL.Query().Get("offset"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	return limit, offset
}

// recordMetrics records HTTP request metrics for the given handler.
func recordMetrics(method, path string, status int, duration time.Duration) {
	statusStr := strconv.Itoa(status)
	metrics.HTTPRequestDuration.WithLabelValues(method, path, statusStr).Observe(duration.Seconds())
	metrics.HTTPRequestsTotal.WithLabelValues(method, path, statusStr).Inc()
}

// handleHealth returns 200 if the service, database, Redis, and P2Pool API are reachable.
func handleHealth(pool *pgxpool.Pool, cacheStore *cache.Store, p2pool *p2poolclient.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		result := map[string]string{"status": "ok", "postgres": "ok", "redis": "ok", "p2pool": "ok"}
		status := http.StatusOK

		if err := pool.Ping(r.Context()); err != nil {
			result["postgres"] = fmt.Sprintf("error: %v", err)
			result["status"] = "degraded"
			status = http.StatusServiceUnavailable
		}

		if err := cacheStore.HealthCheck(r.Context()); err != nil {
			result["redis"] = fmt.Sprintf("error: %v", err)
			result["status"] = "degraded"
			status = http.StatusServiceUnavailable
		}

		if _, err := p2pool.GetPoolStats(r.Context()); err != nil {
			result["p2pool"] = fmt.Sprintf("error: %v", err)
			if result["status"] == "ok" {
				result["status"] = "degraded"
			}
		}

		writeJSON(w, status, result)
		recordMetrics(r.Method, "/health", status, time.Since(start))
	}
}

// handlePoolStats returns aggregated pool statistics with caching.
// Sidechain difficulty is fetched live from the P2Pool API and merged in.
func handlePoolStats(agg *aggregator.Aggregator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		stats, err := agg.GetPoolStatsCached(r.Context())
		if err != nil {
			slog.Error("failed to get pool stats", "error", err)
			writeError(w, http.StatusInternalServerError, "failed to retrieve pool stats")
			recordMetrics(r.Method, "/api/pool/stats", http.StatusInternalServerError, time.Since(start))
			return
		}

		metrics.PoolHashrate.Set(float64(stats.TotalHashrate))
		metrics.PoolMiners.Set(float64(stats.TotalMiners))

		writeJSON(w, http.StatusOK, stats)
		recordMetrics(r.Method, "/api/pool/stats", http.StatusOK, time.Since(start))
	}
}

// handleMinerStats returns stats for a specific miner address.
func handleMinerStats(agg *aggregator.Aggregator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		address := r.PathValue("address")

		if address == "" || len(address) > 256 {
			writeError(w, http.StatusBadRequest, "invalid miner address")
			recordMetrics(r.Method, "/api/miner/{address}", http.StatusBadRequest, time.Since(start))
			return
		}

		stats, err := agg.GetMinerStats(r.Context(), address)
		if err != nil {
			slog.Error("failed to get miner stats", "address", address, "error", err)
			writeError(w, http.StatusInternalServerError, "failed to retrieve miner stats")
			recordMetrics(r.Method, "/api/miner/{address}", http.StatusInternalServerError, time.Since(start))
			return
		}

		writeJSON(w, http.StatusOK, stats)
		recordMetrics(r.Method, "/api/miner/{address}", http.StatusOK, time.Since(start))
	}
}

// handleMinerPayments returns paginated payment history for a miner.
func handleMinerPayments(agg *aggregator.Aggregator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		address := r.PathValue("address")

		if address == "" || len(address) > 256 {
			writeError(w, http.StatusBadRequest, "invalid miner address")
			recordMetrics(r.Method, "/api/miner/{address}/payments", http.StatusBadRequest, time.Since(start))
			return
		}

		limit, offset := parsePagination(r, 50, 200)

		// Cap payment count based on subscription tier.
		tier := subscription.TierFromContext(r.Context())
		limit = subscription.EffectivePaymentLimit(tier, limit, subscription.DefaultFreeLimits())

		payments, err := agg.GetMinerPayments(r.Context(), address, limit, offset, 0)
		if err != nil {
			slog.Error("failed to get miner payments", "address", address, "error", err)
			writeError(w, http.StatusInternalServerError, "failed to retrieve miner payments")
			recordMetrics(r.Method, "/api/miner/{address}/payments", http.StatusInternalServerError, time.Since(start))
			return
		}

		if payments == nil {
			payments = []aggregator.MinerPayment{}
		}

		writeJSON(w, http.StatusOK, payments)
		recordMetrics(r.Method, "/api/miner/{address}/payments", http.StatusOK, time.Since(start))
	}
}

// handleMinerHashrate returns hashrate timeseries for a miner.
func handleMinerHashrate(agg *aggregator.Aggregator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		address := r.PathValue("address")

		if address == "" || len(address) > 256 {
			writeError(w, http.StatusBadRequest, "invalid miner address")
			recordMetrics(r.Method, "/api/miner/{address}/hashrate", http.StatusBadRequest, time.Since(start))
			return
		}

		hours := 24
		if v := r.URL.Query().Get("hours"); v != "" {
			if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
				hours = parsed
			}
		}

		// Cap hashrate history window based on subscription tier.
		tier := subscription.TierFromContext(r.Context())
		hours = subscription.EffectiveHashrateHours(tier, hours, subscription.DefaultFreeLimits())

		points, err := agg.GetMinerHashrate(r.Context(), address, hours)
		if err != nil {
			slog.Error("failed to get miner hashrate", "address", address, "error", err)
			writeError(w, http.StatusInternalServerError, "failed to retrieve miner hashrate")
			recordMetrics(r.Method, "/api/miner/{address}/hashrate", http.StatusInternalServerError, time.Since(start))
			return
		}

		if points == nil {
			points = []aggregator.HashratePoint{}
		}

		writeJSON(w, http.StatusOK, points)
		recordMetrics(r.Method, "/api/miner/{address}/hashrate", http.StatusOK, time.Since(start))
	}
}

// handleWeeklyMiners returns miners active in the last 7 days.
func handleWeeklyMiners(agg *aggregator.Aggregator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		miners, err := agg.GetWeeklyMiners(r.Context())
		if err != nil {
			slog.Error("failed to get weekly miners", "error", err)
			writeError(w, http.StatusInternalServerError, "failed to retrieve weekly miners")
			recordMetrics(r.Method, "/api/miners/weekly", http.StatusInternalServerError, time.Since(start))
			return
		}

		if miners == nil {
			miners = []aggregator.WeeklyMiner{}
		}

		writeJSON(w, http.StatusOK, miners)
		recordMetrics(r.Method, "/api/miners/weekly", http.StatusOK, time.Since(start))
	}
}

// handleMinerUncleRate returns uncle rate timeseries for a miner.
func handleMinerUncleRate(agg *aggregator.Aggregator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		address := r.PathValue("address")

		if address == "" || len(address) > 256 {
			writeError(w, http.StatusBadRequest, "invalid miner address")
			recordMetrics(r.Method, "/api/miner/{address}/uncle-rate", http.StatusBadRequest, time.Since(start))
			return
		}

		hours := 24
		if v := r.URL.Query().Get("hours"); v != "" {
			if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
				hours = parsed
			}
		}
		if hours > 720 {
			hours = 720
		}

		points, err := agg.GetMinerUncleRate(r.Context(), address, hours)
		if err != nil {
			slog.Error("failed to get uncle rate", "address", address, "error", err)
			writeError(w, http.StatusInternalServerError, "failed to retrieve uncle rate")
			recordMetrics(r.Method, "/api/miner/{address}/uncle-rate", http.StatusInternalServerError, time.Since(start))
			return
		}

		if points == nil {
			points = []aggregator.UncleRatePoint{}
		}

		writeJSON(w, http.StatusOK, points)
		recordMetrics(r.Method, "/api/miner/{address}/uncle-rate", http.StatusOK, time.Since(start))
	}
}

// handleMinerWorkers returns per-worker share breakdown for a miner (paid tier only).
func handleMinerWorkers(agg *aggregator.Aggregator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		address := r.PathValue("address")

		if address == "" || len(address) > 256 {
			writeError(w, http.StatusBadRequest, "invalid miner address")
			recordMetrics(r.Method, "/api/miner/{address}/workers", http.StatusBadRequest, time.Since(start))
			return
		}

		workers, err := agg.GetMinerWorkers(r.Context(), address)
		if err != nil {
			slog.Error("failed to get miner workers", "address", address, "error", err)
			writeError(w, http.StatusInternalServerError, "failed to retrieve worker breakdown")
			recordMetrics(r.Method, "/api/miner/{address}/workers", http.StatusInternalServerError, time.Since(start))
			return
		}

		if workers == nil {
			workers = []aggregator.MinerWorker{}
		}

		writeJSON(w, http.StatusOK, workers)
		recordMetrics(r.Method, "/api/miner/{address}/workers", http.StatusOK, time.Since(start))
	}
}

// handleTaxExport returns a CSV file of payments for a miner.
// Supports optional ?year=YYYY to filter by calendar year.
// Appends a totals row at the end of the CSV for easy tax reporting.
func handleTaxExport(agg *aggregator.Aggregator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		address := r.PathValue("address")

		if address == "" || len(address) > 256 {
			writeError(w, http.StatusBadRequest, "invalid miner address")
			recordMetrics(r.Method, "/api/miner/{address}/tax-export", http.StatusBadRequest, time.Since(start))
			return
		}

		// Parse optional year filter.
		var year int
		if v := r.URL.Query().Get("year"); v != "" {
			parsed, err := strconv.Atoi(v)
			if err != nil || parsed < 2000 || parsed > 2100 {
				writeError(w, http.StatusBadRequest, "invalid year parameter")
				recordMetrics(r.Method, "/api/miner/{address}/tax-export", http.StatusBadRequest, time.Since(start))
				return
			}
			year = parsed
		}

		payments, err := agg.GetMinerPaymentsForExport(r.Context(), address, year)
		if err != nil {
			slog.Error("failed to get miner payments for export", "address", address, "error", err)
			writeError(w, http.StatusInternalServerError, "failed to retrieve payments for export")
			recordMetrics(r.Method, "/api/miner/{address}/tax-export", http.StatusInternalServerError, time.Since(start))
			return
		}

		filename := fmt.Sprintf("xmr-payments-%s", address)
		if year > 0 {
			filename += fmt.Sprintf("-%d", year)
		}
		filename += ".csv"
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
		w.WriteHeader(http.StatusOK)

		writer := csv.NewWriter(w)
		defer writer.Flush()

		// Write CSV header.
		if err := writer.Write([]string{
			"date", "amount_atomic", "amount_xmr",
			"xmr_usd_price", "xmr_cad_price", "usd_value", "cad_value",
		}); err != nil {
			slog.Error("failed to write CSV header", "error", err)
			return
		}

		var totalAtomic uint64
		var totalUSD, totalCAD float64
		var hasUSD, hasCAD bool

		for _, p := range payments {
			amountXMR := float64(p.Amount) / 1e12
			totalAtomic += p.Amount

			usdPrice := ""
			cadPrice := ""
			usdValue := ""
			cadValue := ""

			if p.XMRUSDPrice != nil {
				usdPrice = fmt.Sprintf("%.4f", *p.XMRUSDPrice)
				v := amountXMR * (*p.XMRUSDPrice)
				usdValue = fmt.Sprintf("%.4f", v)
				totalUSD += v
				hasUSD = true
			}
			if p.XMRCADPrice != nil {
				cadPrice = fmt.Sprintf("%.4f", *p.XMRCADPrice)
				v := amountXMR * (*p.XMRCADPrice)
				cadValue = fmt.Sprintf("%.4f", v)
				totalCAD += v
				hasCAD = true
			}

			row := []string{
				p.PaidAt.UTC().Format(time.RFC3339),
				strconv.FormatUint(p.Amount, 10),
				fmt.Sprintf("%.12f", amountXMR),
				usdPrice,
				cadPrice,
				usdValue,
				cadValue,
			}

			if err := writer.Write(row); err != nil {
				slog.Error("failed to write CSV row", "error", err)
				return
			}
		}

		// Write totals row.
		totalXMR := float64(totalAtomic) / 1e12
		totalUSDStr := ""
		totalCADStr := ""
		if hasUSD {
			totalUSDStr = fmt.Sprintf("%.4f", totalUSD)
		}
		if hasCAD {
			totalCADStr = fmt.Sprintf("%.4f", totalCAD)
		}
		if err := writer.Write([]string{
			"TOTAL", strconv.FormatUint(totalAtomic, 10),
			fmt.Sprintf("%.12f", totalXMR),
			"", "", totalUSDStr, totalCADStr,
		}); err != nil {
			slog.Error("failed to write CSV totals row", "error", err)
		}

		recordMetrics(r.Method, "/api/miner/{address}/tax-export", http.StatusOK, time.Since(start))
	}
}

// handleMinerPaymentSummary returns per-year payment totals for a miner.
func handleMinerPaymentSummary(agg *aggregator.Aggregator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		address := r.PathValue("address")

		if address == "" || len(address) > 256 {
			writeError(w, http.StatusBadRequest, "invalid miner address")
			recordMetrics(r.Method, "/api/miner/{address}/payment-summary", http.StatusBadRequest, time.Since(start))
			return
		}

		summaries, err := agg.GetMinerPaymentSummary(r.Context(), address)
		if err != nil {
			slog.Error("failed to get payment summary", "address", address, "error", err)
			writeError(w, http.StatusInternalServerError, "failed to retrieve payment summary")
			recordMetrics(r.Method, "/api/miner/{address}/payment-summary", http.StatusInternalServerError, time.Since(start))
			return
		}

		if summaries == nil {
			summaries = []aggregator.PaymentYearSummary{}
		}

		writeJSON(w, http.StatusOK, summaries)
		recordMetrics(r.Method, "/api/miner/{address}/payment-summary", http.StatusOK, time.Since(start))
	}
}

// handleBlocks returns paginated found blocks.
func handleBlocks(agg *aggregator.Aggregator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		limit, offset := parsePagination(r, 50, 200)

		blocks, err := agg.GetBlocks(r.Context(), limit, offset)
		if err != nil {
			slog.Error("failed to get blocks", "error", err)
			writeError(w, http.StatusInternalServerError, "failed to retrieve blocks")
			recordMetrics(r.Method, "/api/blocks", http.StatusInternalServerError, time.Since(start))
			return
		}

		if blocks == nil {
			blocks = []aggregator.FoundBlock{}
		}

		writeJSON(w, http.StatusOK, blocks)
		recordMetrics(r.Method, "/api/blocks", http.StatusOK, time.Since(start))
	}
}

// handleSidechainShares returns paginated sidechain shares.
func handleSidechainShares(agg *aggregator.Aggregator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		limit, offset := parsePagination(r, 100, 500)

		shares, err := agg.GetSidechainShares(r.Context(), limit, offset)
		if err != nil {
			slog.Error("failed to get sidechain shares", "error", err)
			writeError(w, http.StatusInternalServerError, "failed to retrieve sidechain shares")
			recordMetrics(r.Method, "/api/sidechain/shares", http.StatusInternalServerError, time.Since(start))
			return
		}

		if shares == nil {
			shares = []aggregator.SidechainShare{}
		}

		writeJSON(w, http.StatusOK, shares)
		recordMetrics(r.Method, "/api/sidechain/shares", http.StatusOK, time.Since(start))
	}
}

// handleLocalWorkers returns miners currently connected to the local stratum.
func handleLocalWorkers(agg *aggregator.Aggregator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		workers, err := agg.GetLocalWorkers(r.Context())
		if err != nil {
			slog.Error("failed to get local workers", "error", err)
			writeError(w, http.StatusInternalServerError, "failed to retrieve local workers")
			recordMetrics(r.Method, "/api/workers", http.StatusInternalServerError, time.Since(start))
			return
		}

		if workers == nil {
			workers = []aggregator.LocalWorker{}
		}

		writeJSON(w, http.StatusOK, workers)
		recordMetrics(r.Method, "/api/workers", http.StatusOK, time.Since(start))
	}
}

// handleSidechainStats returns pool stats timeseries for the sidechain page.
func handleSidechainStats(agg *aggregator.Aggregator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		hours := 24
		if v := r.URL.Query().Get("hours"); v != "" {
			if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 && parsed <= 168 {
				hours = parsed
			}
		}

		points, err := agg.GetPoolStatsHistory(r.Context(), hours)
		if err != nil {
			slog.Error("failed to get sidechain stats", "error", err)
			writeError(w, http.StatusInternalServerError, "failed to retrieve sidechain stats")
			recordMetrics(r.Method, "/api/sidechain/stats", http.StatusInternalServerError, time.Since(start))
			return
		}

		if points == nil {
			points = []aggregator.PoolStatsPoint{}
		}

		writeJSON(w, http.StatusOK, points)
		recordMetrics(r.Method, "/api/sidechain/stats", http.StatusOK, time.Since(start))
	}
}

// handleBackfillPrices fills in NULL xmr_usd_price/xmr_cad_price for historical
// payments by looking up the block timestamp and fetching the historical price
// from CoinGecko. Payments are grouped by date to minimize API calls.
func handleBackfillPrices(pool *pgxpool.Pool, oracle *scanner.PriceOracle, adminToken string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ctx := r.Context()

		// Defense in depth: verify admin token even if gateway JWT already checked.
		token := r.Header.Get("X-Admin-Token")
		if token == "" || subtle.ConstantTimeCompare([]byte(token), []byte(adminToken)) != 1 {
			writeError(w, http.StatusForbidden, "forbidden")
			recordMetrics(r.Method, "/api/admin/backfill-prices", http.StatusForbidden, time.Since(start))
			return
		}

		if oracle == nil {
			writeError(w, http.StatusServiceUnavailable, "price oracle not configured")
			return
		}

		// Find all payments missing prices, grouped by created_at date.
		rows, err := pool.Query(ctx,
			`SELECT id, created_at FROM payments
			 WHERE xmr_usd_price IS NULL
			 ORDER BY created_at ASC`)
		if err != nil {
			slog.Error("backfill: failed to query payments", "error", err)
			writeError(w, http.StatusInternalServerError, "failed to query payments")
			return
		}
		defer rows.Close()

		type pendingPayment struct {
			ID        int64
			CreatedAt time.Time
		}

		var pending []pendingPayment
		for rows.Next() {
			var p pendingPayment
			if err := rows.Scan(&p.ID, &p.CreatedAt); err != nil {
				slog.Error("backfill: failed to scan row", "error", err)
				writeError(w, http.StatusInternalServerError, "failed to scan payment row")
				return
			}
			pending = append(pending, p)
		}
		if err := rows.Err(); err != nil {
			slog.Error("backfill: row iteration error", "error", err)
			writeError(w, http.StatusInternalServerError, "failed to iterate payments")
			return
		}

		if len(pending) == 0 {
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"updated":  0,
				"skipped":  0,
				"duration": time.Since(start).String(),
			})
			return
		}

		// Group by date to avoid redundant CoinGecko calls.
		priceCache := make(map[string]*scanner.Price) // "2006-01-02" -> Price
		updated := 0
		skipped := 0

		for _, p := range pending {
			dateKey := p.CreatedAt.UTC().Format("2006-01-02")

			price, cached := priceCache[dateKey]
			if !cached {
				var fetchErr error
				price, fetchErr = oracle.GetHistoricalPrice(ctx, p.CreatedAt.UTC())
				if fetchErr != nil {
					slog.Warn("backfill: failed to fetch historical price, skipping",
						"date", dateKey, "error", fetchErr)
					priceCache[dateKey] = nil // mark as failed so we don't retry
					skipped++
					continue
				}
				priceCache[dateKey] = price
			}

			if price == nil {
				skipped++
				continue
			}

			_, err := pool.Exec(ctx,
				`UPDATE payments SET xmr_usd_price = $1, xmr_cad_price = $2 WHERE id = $3`,
				price.USD, price.CAD, p.ID)
			if err != nil {
				slog.Error("backfill: failed to update payment", "id", p.ID, "error", err)
				skipped++
				continue
			}
			updated++
		}

		slog.Info("backfill complete", "updated", updated, "skipped", skipped, "duration", time.Since(start))
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"updated":  updated,
			"skipped":  skipped,
			"duration": time.Since(start).String(),
		})
		recordMetrics(r.Method, "/api/admin/backfill-prices", http.StatusOK, time.Since(start))
	}
}

// handleAlertWebhook receives alert notifications from Alertmanager, logs them,
// and increments Prometheus counters. Requires a valid admin token.
func handleAlertWebhook(adminToken string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Validate admin token via X-Admin-Token header or Authorization: Bearer.
		token := r.Header.Get("X-Admin-Token")
		if token == "" {
			if auth := r.Header.Get("Authorization"); len(auth) > 7 && auth[:7] == "Bearer " {
				token = auth[7:]
			}
		}
		if token == "" || subtle.ConstantTimeCompare([]byte(token), []byte(adminToken)) != 1 {
			writeError(w, http.StatusForbidden, "forbidden")
			recordMetrics(r.Method, "/api/webhook/alerts", http.StatusForbidden, time.Since(start))
			return
		}

		// Parse Alertmanager v4 webhook payload (limit body to 1MB).
		type alertmanagerPayload struct {
			Status string `json:"status"`
			Alerts []struct {
				Status      string            `json:"status"`
				Labels      map[string]string `json:"labels"`
				Annotations map[string]string `json:"annotations"`
				StartsAt    string            `json:"startsAt"`
				EndsAt      string            `json:"endsAt"`
			} `json:"alerts"`
		}

		var payload alertmanagerPayload
		body := io.LimitReader(r.Body, 1<<20) // 1 MB
		if err := json.NewDecoder(body).Decode(&payload); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON payload")
			recordMetrics(r.Method, "/api/webhook/alerts", http.StatusBadRequest, time.Since(start))
			return
		}

		for _, alert := range payload.Alerts {
			alertname := alert.Labels["alertname"]
			status := alert.Status
			severity := alert.Labels["severity"]
			summary := alert.Annotations["summary"]

			if status == "firing" {
				slog.Warn("alert received",
					"alertname", alertname,
					"status", status,
					"severity", severity,
					"summary", summary,
				)
			} else {
				slog.Info("alert received",
					"alertname", alertname,
					"status", status,
					"severity", severity,
					"summary", summary,
				)
			}

			metrics.AlertsReceived.WithLabelValues(alertname, status).Inc()
		}

		writeJSON(w, http.StatusOK, map[string]int{"received": len(payload.Alerts)})
		recordMetrics(r.Method, "/api/webhook/alerts", http.StatusOK, time.Since(start))
	}
}

// handleSubscriptionIncome returns all confirmed subscription payments for admin tax reporting.
// Supports ?year=YYYY filter and ?format=csv for CSV export.
func handleSubscriptionIncome(subSvc *subscription.Service, adminToken string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ctx := r.Context()

		token := r.Header.Get("X-Admin-Token")
		if token == "" || subtle.ConstantTimeCompare([]byte(token), []byte(adminToken)) != 1 {
			writeError(w, http.StatusForbidden, "forbidden")
			recordMetrics(r.Method, "/api/admin/subscription-income", http.StatusForbidden, time.Since(start))
			return
		}

		var year int
		if v := r.URL.Query().Get("year"); v != "" {
			parsed, err := strconv.Atoi(v)
			if err != nil || parsed < 2000 || parsed > 2100 {
				writeError(w, http.StatusBadRequest, "invalid year parameter")
				recordMetrics(r.Method, "/api/admin/subscription-income", http.StatusBadRequest, time.Since(start))
				return
			}
			year = parsed
		}

		payments, err := subSvc.GetAllConfirmedPayments(ctx, year)
		if err != nil {
			slog.Error("failed to get subscription income", "error", err)
			writeError(w, http.StatusInternalServerError, "failed to retrieve subscription income")
			recordMetrics(r.Method, "/api/admin/subscription-income", http.StatusInternalServerError, time.Since(start))
			return
		}

		if payments == nil {
			payments = []subscription.SubPayment{}
		}

		if r.URL.Query().Get("format") == "csv" {
			filename := "subscription-income"
			if year > 0 {
				filename += fmt.Sprintf("-%d", year)
			}
			filename += ".csv"
			w.Header().Set("Content-Type", "text/csv")
			w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
			w.WriteHeader(http.StatusOK)

			writer := csv.NewWriter(w)
			defer writer.Flush()

			if err := writer.Write([]string{
				"date", "tx_hash", "miner_address", "amount_atomic", "amount_xmr",
				"xmr_usd_price", "xmr_cad_price", "usd_value", "cad_value",
			}); err != nil {
				slog.Error("failed to write CSV header", "error", err)
				return
			}

			var totalAtomic uint64
			var totalCAD float64
			var hasCAD bool

			for _, p := range payments {
				amountXMR := float64(p.Amount) / 1e12
				totalAtomic += p.Amount

				usdPrice, cadPrice, usdValue, cadValue := "", "", "", ""
				if p.XMRUSDPrice != nil {
					usdPrice = fmt.Sprintf("%.4f", *p.XMRUSDPrice)
					usdValue = fmt.Sprintf("%.4f", amountXMR*(*p.XMRUSDPrice))
				}
				if p.XMRCADPrice != nil {
					cadPrice = fmt.Sprintf("%.4f", *p.XMRCADPrice)
					v := amountXMR * (*p.XMRCADPrice)
					cadValue = fmt.Sprintf("%.4f", v)
					totalCAD += v
					hasCAD = true
				}

				if err := writer.Write([]string{
					p.CreatedAt.UTC().Format(time.RFC3339),
					p.TxHash,
					p.MinerAddress,
					strconv.FormatUint(p.Amount, 10),
					fmt.Sprintf("%.12f", amountXMR),
					usdPrice, cadPrice, usdValue, cadValue,
				}); err != nil {
					slog.Error("failed to write CSV row", "error", err)
					return
				}
			}

			totalXMR := float64(totalAtomic) / 1e12
			totalCADStr := ""
			if hasCAD {
				totalCADStr = fmt.Sprintf("%.4f", totalCAD)
			}
			_ = writer.Write([]string{
				"TOTAL", "", "",
				strconv.FormatUint(totalAtomic, 10),
				fmt.Sprintf("%.12f", totalXMR),
				"", "", "", totalCADStr,
			})

			recordMetrics(r.Method, "/api/admin/subscription-income", http.StatusOK, time.Since(start))
			return
		}

		writeJSON(w, http.StatusOK, payments)
		recordMetrics(r.Method, "/api/admin/subscription-income", http.StatusOK, time.Since(start))
	}
}

// handleBackfillSubPrices fills in NULL xmr_cad_price (and xmr_usd_price) for
// historical subscription payments using CoinGecko historical price data.
func handleBackfillSubPrices(pool *pgxpool.Pool, oracle *scanner.PriceOracle, adminToken string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ctx := r.Context()

		token := r.Header.Get("X-Admin-Token")
		if token == "" || subtle.ConstantTimeCompare([]byte(token), []byte(adminToken)) != 1 {
			writeError(w, http.StatusForbidden, "forbidden")
			recordMetrics(r.Method, "/api/admin/backfill-sub-prices", http.StatusForbidden, time.Since(start))
			return
		}

		if oracle == nil {
			writeError(w, http.StatusServiceUnavailable, "price oracle not configured")
			return
		}

		rows, err := pool.Query(ctx,
			`SELECT id, created_at FROM subscription_payments
			 WHERE xmr_cad_price IS NULL
			 ORDER BY created_at ASC`)
		if err != nil {
			slog.Error("backfill-sub: failed to query payments", "error", err)
			writeError(w, http.StatusInternalServerError, "failed to query subscription payments")
			return
		}
		defer rows.Close()

		type pendingPayment struct {
			ID        int64
			CreatedAt time.Time
		}

		var pending []pendingPayment
		for rows.Next() {
			var p pendingPayment
			if err := rows.Scan(&p.ID, &p.CreatedAt); err != nil {
				slog.Error("backfill-sub: failed to scan row", "error", err)
				writeError(w, http.StatusInternalServerError, "failed to scan payment row")
				return
			}
			pending = append(pending, p)
		}
		if err := rows.Err(); err != nil {
			slog.Error("backfill-sub: row iteration error", "error", err)
			writeError(w, http.StatusInternalServerError, "failed to iterate payments")
			return
		}

		if len(pending) == 0 {
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"updated": 0, "skipped": 0, "duration": time.Since(start).String(),
			})
			return
		}

		priceCache := make(map[string]*scanner.Price)
		updated, skipped := 0, 0

		for _, p := range pending {
			dateKey := p.CreatedAt.UTC().Format("2006-01-02")

			price, cached := priceCache[dateKey]
			if !cached {
				var fetchErr error
				price, fetchErr = oracle.GetHistoricalPrice(ctx, p.CreatedAt.UTC())
				if fetchErr != nil {
					slog.Warn("backfill-sub: failed to fetch historical price",
						"date", dateKey, "error", fetchErr)
					priceCache[dateKey] = nil
					skipped++
					continue
				}
				priceCache[dateKey] = price
			}

			if price == nil {
				skipped++
				continue
			}

			_, err := pool.Exec(ctx,
				`UPDATE subscription_payments
				 SET xmr_usd_price = COALESCE(xmr_usd_price, $1),
				     xmr_cad_price = $2
				 WHERE id = $3`,
				price.USD, price.CAD, p.ID)
			if err != nil {
				slog.Error("backfill-sub: failed to update payment", "id", p.ID, "error", err)
				skipped++
				continue
			}
			updated++
		}

		slog.Info("backfill-sub complete", "updated", updated, "skipped", skipped, "duration", time.Since(start))
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"updated":  updated,
			"skipped":  skipped,
			"duration": time.Since(start).String(),
		})
		recordMetrics(r.Method, "/api/admin/backfill-sub-prices", http.StatusOK, time.Since(start))
	}
}

// handleSubscriptionAddress returns (or assigns) the payment subaddress for a miner.
func handleSubscriptionAddress(subScanner *subscription.Scanner, oracle *scanner.PriceOracle) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		address := r.PathValue("address")

		if address == "" || len(address) > 256 {
			writeError(w, http.StatusBadRequest, "invalid miner address")
			recordMetrics(r.Method, "/api/subscription/address/{address}", http.StatusBadRequest, time.Since(start))
			return
		}

		if subScanner == nil {
			writeError(w, http.StatusServiceUnavailable, "payment address generation is not configured")
			recordMetrics(r.Method, "/api/subscription/address/{address}", http.StatusServiceUnavailable, time.Since(start))
			return
		}

		sa, err := subScanner.AssignSubaddress(r.Context(), address)
		if err != nil {
			slog.Error("failed to assign subaddress", "address", address, "error", err)
			writeError(w, http.StatusInternalServerError, "failed to assign payment address")
			recordMetrics(r.Method, "/api/subscription/address/{address}", http.StatusInternalServerError, time.Since(start))
			return
		}

		resp := subscription.PaymentAddress{
			MinerAddress: sa.MinerAddress,
			Subaddress:   sa.Subaddress,
			AmountUSD:    "$1+ Supporter / $5+ Champion",
		}

		// Show suggested XMR amount for supporter tier based on current price.
		if oracle != nil {
			price, priceErr := oracle.GetPrice(r.Context())
			if priceErr == nil && price.USD > 0 {
				suggestedXMR := subscription.SuggestedSupporterUSD / price.USD
				resp.AmountXMR = fmt.Sprintf("%.6f", suggestedXMR)
			}
		}

		writeJSON(w, http.StatusOK, resp)
		recordMetrics(r.Method, "/api/subscription/address/{address}", http.StatusOK, time.Since(start))
	}
}

// handleSubscriptionStatus returns the subscription tier and expiry for a miner.
func handleSubscriptionStatus(subSvc *subscription.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		address := r.PathValue("address")

		if address == "" || len(address) > 256 {
			writeError(w, http.StatusBadRequest, "invalid miner address")
			recordMetrics(r.Method, "/api/subscription/status/{address}", http.StatusBadRequest, time.Since(start))
			return
		}

		status, err := subSvc.GetStatus(r.Context(), address)
		if err != nil {
			slog.Error("failed to get subscription status", "address", address, "error", err)
			writeError(w, http.StatusInternalServerError, "failed to retrieve subscription status")
			recordMetrics(r.Method, "/api/subscription/status/{address}", http.StatusInternalServerError, time.Since(start))
			return
		}

		writeJSON(w, http.StatusOK, status)
		recordMetrics(r.Method, "/api/subscription/status/{address}", http.StatusOK, time.Since(start))
	}
}

// handleSubscriptionPayments returns subscription payment history for a miner.
func handleSubscriptionPayments(subSvc *subscription.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		address := r.PathValue("address")

		if address == "" || len(address) > 256 {
			writeError(w, http.StatusBadRequest, "invalid miner address")
			recordMetrics(r.Method, "/api/subscription/payments/{address}", http.StatusBadRequest, time.Since(start))
			return
		}

		limit, offset := parsePagination(r, 50, 200)

		payments, err := subSvc.GetPayments(r.Context(), address, limit, offset)
		if err != nil {
			slog.Error("failed to get subscription payments", "address", address, "error", err)
			writeError(w, http.StatusInternalServerError, "failed to retrieve subscription payments")
			recordMetrics(r.Method, "/api/subscription/payments/{address}", http.StatusInternalServerError, time.Since(start))
			return
		}

		if payments == nil {
			payments = []subscription.SubPayment{}
		}

		writeJSON(w, http.StatusOK, payments)
		recordMetrics(r.Method, "/api/subscription/payments/{address}", http.StatusOK, time.Since(start))
	}
}

// handleGenerateAPIKey generates a new API key for a paid subscriber.
// Proof of ownership required:
//   - First-time: POST body {"tx_hash": "..."} with a confirmed subscription payment.
//   - Regeneration: X-API-Key header with the existing key.
func handleGenerateAPIKey(subSvc *subscription.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		address := r.PathValue("address")

		if address == "" || len(address) > 256 {
			writeError(w, http.StatusBadRequest, "invalid miner address")
			recordMetrics(r.Method, "/api/subscription/api-key/{address}", http.StatusBadRequest, time.Since(start))
			return
		}

		// Parse proof of ownership from request.
		existingKey := r.Header.Get("X-API-Key")
		var txHash string
		if existingKey == "" {
			var body struct {
				TxHash string `json:"tx_hash"`
			}
			if err := json.NewDecoder(io.LimitReader(r.Body, 1<<10)).Decode(&body); err == nil {
				txHash = body.TxHash
			}
		}

		key, err := subSvc.GenerateAPIKey(r.Context(), address, txHash, existingKey)
		if err != nil {
			slog.Error("failed to generate API key", "address", address, "error", err)
			writeError(w, http.StatusForbidden, err.Error())
			recordMetrics(r.Method, "/api/subscription/api-key/{address}", http.StatusForbidden, time.Since(start))
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{
			"api_key": key,
			"note":    "Store this key securely. It cannot be retrieved again.",
		})
		recordMetrics(r.Method, "/api/subscription/api-key/{address}", http.StatusOK, time.Since(start))
	}
}

// handleFundStatus returns the current month's funding progress.
func handleFundStatus(fundSvc *fund.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		status, err := fundSvc.GetStatus(r.Context())
		if err != nil {
			slog.Error("failed to get fund status", "error", err)
			writeError(w, http.StatusInternalServerError, "failed to retrieve fund status")
			recordMetrics(r.Method, "/api/fund/status", http.StatusInternalServerError, time.Since(start))
			return
		}

		// Update Prometheus gauges.
		metrics.FundPercentFunded.Set(float64(status.PercentFunded))
		metrics.FundSupporterCount.Set(float64(status.SupporterCount))

		writeJSON(w, http.StatusOK, status)
		recordMetrics(r.Method, "/api/fund/status", http.StatusOK, time.Since(start))
	}
}

// handleFundHistory returns monthly funding history for charts.
func handleFundHistory(fundSvc *fund.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		months, err := fundSvc.GetHistory(r.Context())
		if err != nil {
			slog.Error("failed to get fund history", "error", err)
			writeError(w, http.StatusInternalServerError, "failed to retrieve fund history")
			recordMetrics(r.Method, "/api/fund/history", http.StatusInternalServerError, time.Since(start))
			return
		}

		if months == nil {
			months = []fund.FundMonth{}
		}

		writeJSON(w, http.StatusOK, months)
		recordMetrics(r.Method, "/api/fund/history", http.StatusOK, time.Since(start))
	}
}

// handleFundSupporters returns the opt-in list of current month's contributors.
func handleFundSupporters(fundSvc *fund.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		supporters, err := fundSvc.GetSupporters(r.Context())
		if err != nil {
			slog.Error("failed to get supporters", "error", err)
			writeError(w, http.StatusInternalServerError, "failed to retrieve supporters")
			recordMetrics(r.Method, "/api/fund/supporters", http.StatusInternalServerError, time.Since(start))
			return
		}

		if supporters == nil {
			supporters = []fund.Supporter{}
		}

		writeJSON(w, http.StatusOK, supporters)
		recordMetrics(r.Method, "/api/fund/supporters", http.StatusOK, time.Since(start))
	}
}

// handleNodesStatus returns the health status of all shared nodes.
func handleNodesStatus(nodePoolSvc *nodepool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		status, err := nodePoolSvc.GetNodesStatus(r.Context())
		if err != nil {
			slog.Error("failed to get nodes status", "error", err)
			writeError(w, http.StatusInternalServerError, "failed to retrieve node status")
			recordMetrics(r.Method, "/api/nodes/status", http.StatusInternalServerError, time.Since(start))
			return
		}

		writeJSON(w, http.StatusOK, status)
		recordMetrics(r.Method, "/api/nodes/status", http.StatusOK, time.Since(start))
	}
}

// handleNodesConnectionInfo returns stratum URLs and XMRig configs.
func handleNodesConnectionInfo(nodePoolSvc *nodepool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		info, err := nodePoolSvc.GetConnectionInfo(r.Context())
		if err != nil {
			slog.Error("failed to get connection info", "error", err)
			writeError(w, http.StatusInternalServerError, "failed to retrieve connection info")
			recordMetrics(r.Method, "/api/nodes/connection-info", http.StatusInternalServerError, time.Since(start))
			return
		}

		writeJSON(w, http.StatusOK, info)
		recordMetrics(r.Method, "/api/nodes/connection-info", http.StatusOK, time.Since(start))
	}
}
