# Config

Infrastructure configuration files mounted into Docker containers at runtime.

## Directories

| Directory | Mounts to | Purpose |
|---|---|---|
| `nginx/` | nginx container | TLS termination, rate limiting, reverse proxy to gateway and frontend |
| `prometheus/` | Prometheus container | Scrape config (manager metrics on `:9090`) and alert rules |
| `prometheus/alerts/` | Prometheus | Alert rule files (pool hashrate drop, service down, etc.) |
| `alertmanager/` | Alertmanager container | Alert routing and notification config |
| `grafana/provisioning/` | Grafana container | Auto-provisioned datasources (Prometheus, Loki) and dashboards |
| `loki/` | Loki + Promtail containers | Log aggregation config and Promtail scrape targets |

## Traffic Flow

```
Internet -> nginx (443/TLS) -> gateway (8080) -> manager (8081)
                             -> frontend (3000)
```

Nginx handles TLS termination, security headers (HSTS, X-Frame-Options), and a
10 req/s rate limit on `/api/` routes. WebSocket connections at `/ws` are
upgraded and proxied through to the gateway.
