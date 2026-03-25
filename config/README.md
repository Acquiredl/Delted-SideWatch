# Config

Infrastructure configuration files mounted into Docker containers at runtime.

## Directories

| Directory | Mounts to | Purpose |
|---|---|---|
| `nginx/` | nginx container | TLS termination, rate limiting, reverse proxy to gateway and frontend |
| `prometheus/` | Prometheus container | Scrape config (manager metrics on `:9090`) and alert rules |
| `prometheus/alerts/` | Prometheus | Alert rule files (`pool.yml`) |
| `alertmanager/` | Alertmanager container | Alert routing — manager webhook, optional Discord/email |
| `grafana/provisioning/` | Grafana container | Auto-provisioned datasources (Prometheus, Loki) and dashboards |
| `loki/` | Loki + Promtail containers | Log aggregation config and Promtail Docker socket scraper |
| `tor/` | Tor container | Hidden service v3 configuration (`.onion` access) |

## Traffic Flow

```
Internet -> nginx (443/TLS) -> gateway (8080) -> manager (8081)
                             -> frontend (3000)
Tor      -> nginx (80/HTTP)  -> (same routing as above)
```

Nginx handles TLS termination, security headers (HSTS, X-Frame-Options,
X-Content-Type-Options, X-XSS-Protection), and a 10 req/s rate limit on
`/api/` routes with burst of 20. WebSocket connections at `/ws` are upgraded
and proxied through to the gateway. The `/health` endpoint is exempt from
rate limiting. HTTP port 80 redirects to HTTPS and serves ACME challenges
for Let's Encrypt renewal.

## Alert Rules

Five Prometheus alert rules are defined in `prometheus/alerts/pool.yml`:

| Alert | Severity | Condition |
|---|---|---|
| `PoolHashrateDropped` | warning | Hashrate drops >50% vs 1h ago (10m trigger) |
| `NoBlocksFound24h` | warning | No blocks in 24 hours |
| `IndexerHighErrorRate` | critical | Error rate >0.1/s |
| `HighAPILatency` | warning | API p95 latency >2s |
| `ManyPendingBlocks` | warning | >20 blocks awaiting confirmation |

Alerts route to the manager webhook by default. Discord and email receivers
are commented out in `alertmanager/alertmanager.yml` — uncomment and configure
to enable.

## Grafana Dashboards

Two dashboards are auto-provisioned:

- **Pool Overview** (`pool-overview.json`) — pool hashrate, active miners,
  blocks found, payments recorded, API request rate, API latency percentiles,
  indexer poll duration, indexer error rate (9 panels)
- **Miner Detail** (`miner-detail.json`) — per-miner hashrate, current
  hashrate, active shares, total paid, payment count, last payment time,
  pool share percentage, share rate (8 panels, templated by `miner_address`)

Datasources: Prometheus (metrics) and Loki (logs) are auto-provisioned via
`grafana/provisioning/datasources/prometheus.yml`.

## Tor Hidden Service

The `tor/torrc` file configures a v3 hidden service that proxies port 80
through nginx. The `.onion` hostname is generated on first start and persisted
in a Docker volume. Retrieve it with `make tor-hostname`.
