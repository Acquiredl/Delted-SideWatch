# Developer Cheatsheet

Quick reference for common tasks in the XMR P2Pool Dashboard project.

## Common Commands

```bash
# Start full development stack (hot reload)
make dev

# Start production stack
docker compose up -d

# Run all Go tests
make test
# or manually:
cd services/manager && go test -race ./...
cd services/gateway && go test -race ./...

# Run tests with mocknode (no real P2Pool/monerod needed)
docker compose -f docker-compose.yml -f docker-compose.test.yml up --build

# Run linter
make lint

# Check service health
curl http://localhost:8081/health   # manager
curl http://localhost:8080/health   # gateway

# View logs
docker compose logs -f manager
docker compose logs -f gateway

# Database shell
docker compose exec postgres psql -U manager_user -d p2pool_dashboard

# Redis shell
docker compose exec redis redis-cli

# Backup database
bash infra/scripts/pool-backup.sh

# Get Tor .onion hostname
make tor-hostname
```

## API Endpoint Quick Reference

```
GET /health                           -- service health
GET /api/pool/stats                   -- pool overview
GET /api/miner/{address}              -- miner stats
GET /api/miner/{address}/payments     -- payment history (?limit=50&offset=0)
GET /api/miner/{address}/hashrate     -- hashrate chart   (?hours=24)
GET /api/miner/{address}/tax-export   -- CSV payment export (paid tier only)
GET /api/blocks                       -- found blocks     (?limit=50&offset=0)
GET /api/sidechain/shares             -- sidechain shares (?limit=100&offset=0)

Subscription:
GET  /api/subscription/address/{addr}  -- get/assign payment subaddress
GET  /api/subscription/status/{addr}   -- tier status + expiry
GET  /api/subscription/payments/{addr} -- subscription payment history
POST /api/subscription/api-key/{addr}  -- generate API key (paid tier only)
```

Tier limits: free tier caps hashrate history at 720h (30d) and payments at 100
per request. Paid tier is uncapped. Pass `X-API-Key` header to authenticate.

## Adding a New API Endpoint

1. Add the route in `services/manager/cmd/manager/routes.go`:
   ```go
   mux.HandleFunc("GET /api/my-endpoint", handleMyEndpoint(agg))
   ```

2. Implement the handler in the same file or in a relevant `internal/` package.
   Always call `recordMetrics()` for observability.

3. Add TypeScript types in `frontend/lib/api.ts`.

4. Wire up the frontend page or component.

## Adding a New Database Migration

1. Create a new SQL file in `services/manager/pkg/db/migrations/`:
   ```
   003_my_migration.sql
   ```
   Use the next sequential number.

2. Write forward-only SQL (no down migrations). Include indexes for expected
   query patterns.

3. All tables must have:
   - `BIGSERIAL` primary key
   - `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`
   - `snake_case` naming

4. Test with `EXPLAIN ANALYZE` against realistic data before committing.

## Adding a Prometheus Metric

1. Register the metric in `services/manager/internal/metrics/metrics.go`:
   ```go
   MyMetric = promauto.NewGauge(prometheus.GaugeOpts{
       Name: "p2pool_my_metric",
       Help: "Description of the metric",
   })
   ```

2. Instrument the relevant code path:
   ```go
   metrics.MyMetric.Set(value)
   ```

3. Add a Grafana panel in `config/grafana/provisioning/dashboards/pool-overview.json`.

4. Optionally add an alert rule in `config/prometheus/alerts/pool.yml`.

## Debugging Tips

- **Manager not starting?** Check `docker compose logs manager` for missing
  environment variables. Required vars use `mustGetEnv()` and will panic with
  a clear message.

- **Database connection issues?** Verify PostgreSQL is healthy:
  ```bash
  docker compose exec postgres pg_isready -U manager_user
  ```

- **Redis connection issues?** Verify Redis is healthy:
  ```bash
  docker compose exec redis redis-cli ping
  ```

- **P2Pool API unreachable?** The manager polls `P2POOL_API_URL`. Ensure the
  P2Pool node is running and reachable from the Docker network:
  ```bash
  docker compose exec manager wget -qO- http://p2pool:3333/api/pool/stats
  ```

- **Metrics not appearing in Grafana?** Check that Prometheus can scrape the
  manager:
  ```bash
  curl http://localhost:9090/api/v1/targets
  ```

- **Indexer errors?** Check the `p2pool_indexer_errors_total` metric or grep
  the manager logs:
  ```bash
  docker compose logs manager | grep "error"
  ```

- **Stale data?** The indexer polls every 30 seconds. The aggregator caches
  pool stats for 15 seconds. Check both the indexer loop and Redis cache if
  data appears stale.

- **Subscription scanner not starting?** The scanner only starts if
  `WALLET_RPC_URL` is set. Check the logs:
  ```bash
  docker compose logs manager | grep "subscription"
  ```

- **Wallet-RPC sync lag?** On first deploy, the view-only wallet must sync
  to the current blockchain height. Subscription payment detection will fail
  until sync completes:
  ```bash
  curl -s http://localhost:18088/json_rpc \
    -d '{"jsonrpc":"2.0","id":"0","method":"get_height"}' \
    -H 'Content-Type: application/json'
  ```

- **Tor not working?** Check the Tor container logs and verify the hidden
  service hostname was generated:
  ```bash
  docker compose logs tor
  make tor-hostname
  ```
