# Gateway Service

Reverse proxy that sits between nginx and the manager service. Handles JWT
authentication, Redis-backed rate limiting, structured request logging, and
request ID injection. All non-health traffic is forwarded to the manager.

## Key Exports

| Package | Description |
|---|---|
| `internal/auth` | JWT verification middleware — protects `/admin/` routes |
| `internal/middleware` | Request ID, structured logger, and Redis-backed rate limiter |

## Architecture

The gateway builds a middleware chain around a `net/http` ServeMux:

```
RequestID -> Logger -> RateLimiter -> JWT Auth -> ServeMux
```

- `/health` — served directly, not proxied or rate-limited
- `/admin/*` — requires a valid JWT (HS256, secret from Docker secrets)
- Everything else — proxied to `http://manager:8081` via `httputil.ReverseProxy`

## Configuration

All via environment variables with Docker secrets fallback:

- `JWT_SECRET` (required)
- `MANAGER_URL` (default: `http://manager:8081`)
- `REDIS_URL` (default: `redis:6379`)
- `RATE_LIMIT_RPS` (default: `10`)
- `API_PORT` (default: `8080`)

## Running

```bash
go build -o gateway ./cmd/gateway/
./gateway

go test -race ./...
```

## Dependencies

- `golang-jwt/jwt/v5` — JWT parsing and validation
- `go-redis/v9` — Redis client for rate limit state
