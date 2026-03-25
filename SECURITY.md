# Security Policy

## Design Principles

The XMR P2Pool Dashboard is a **read-only monitoring service**. It indexes
publicly available blockchain and sidechain data and presents it to miners.
The following constraints are fundamental to the architecture:

### No Wallet RPC

This project does not integrate with any Monero wallet RPC. We never hold,
transfer, or have access to miner funds. All payments are handled natively by
the P2Pool protocol directly on the Monero blockchain.

### Docker Secrets

All sensitive configuration values (database passwords, JWT signing keys) are
read from Docker secrets at `/run/secrets/<name>` with an environment variable
fallback. Secrets are never logged or included in error messages.

### Non-Root Containers

Every Dockerfile specifies a non-root `USER`. No service runs as root inside
its container.

### Least-Privilege Database Access

PostgreSQL is configured with a dedicated `manager_user` role that only has
permissions on the tables it owns. No superuser access is used at runtime.

### No IP Logging for Address Lookups

Miner address lookups (`/api/miner/{address}`) do not log the requesting IP
address alongside the Monero address. This prevents the service from building
a mapping between IP addresses and wallet addresses.

### Rate Limiting

Rate limiting is applied at two layers:

1. **Nginx** -- `limit_req_zone` at 10 requests/second per IP with a burst of 20
2. **Go Gateway** -- application-level rate limiting middleware

### TLS

All external traffic is TLS-encrypted via nginx. Internal Docker network
communication uses plain HTTP, as it is isolated within the Docker bridge
network.

## Supported Versions

Only the latest release is supported with security updates.

## Reporting a Vulnerability

If you discover a security vulnerability, please report it responsibly:

1. **Do not** open a public GitHub issue
2. Email: delted@delted.dev
3. Include a description of the vulnerability and steps to reproduce
4. We will acknowledge receipt within 48 hours and provide a timeline for a fix

We appreciate responsible disclosure and will credit reporters (with permission)
in the release notes.

## Scope

The following are considered in-scope for security reports:

- Authentication bypass in the gateway or admin endpoints
- SQL injection or other database attacks
- Information disclosure (IP/address correlation, secret leakage)
- Container escape or privilege escalation
- Denial of service via resource exhaustion

The following are **out of scope**:

- Vulnerabilities in upstream dependencies (report to the upstream project)
- Social engineering attacks
- Issues that require physical access to the server
