# Security Policy

## Design Principles

SideWatch (XMR P2Pool Dashboard) is a **read-only monitoring service**. It indexes
publicly available blockchain and sidechain data and presents it to miners.
The following constraints are fundamental to the architecture:

### No Miner Wallet RPC

This project never holds, transfers, or has access to **miner funds**. All mining
payments are handled natively by the P2Pool protocol directly on the Monero
blockchain.

The optional subscription system uses a **view-only** `monero-wallet-rpc` instance
to detect incoming operator subscription payments. This wallet cannot spend funds --
it only watches for incoming transfers. The full spend key is kept offline by the
operator. See `docs/subscription-setup.md` for details.

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

### Data Retention Policy

SideWatch enforces tier-based data retention to limit the amount of per-address
data stored:

- **Free tier:** 30-day rolling window. Shares, hashrate buckets, and payment
  records older than 30 days are pruned daily. This limits the exposure window
  if the database is compromised.
- **Paid tier:** 15-month extended retention. Miners explicitly opt in to longer
  storage when they subscribe. Retention starts from the first payment after
  subscription activation — history cannot be retroactively recovered.
- **Pruning job:** Runs daily as part of the timeseries builder. Free-tier data
  is deleted first, then paid-tier data older than 15 months.

### Data Collected

SideWatch stores the following data per miner:

| Data | Source | Retention |
|------|--------|-----------|
| Share timestamps + difficulty | P2Pool sidechain API | Tier-dependent |
| Uncle share status | P2Pool sidechain API | Tier-dependent |
| Miner software ID + version | P2Pool sidechain API | Tier-dependent |
| Hashrate timeseries (15-min buckets) | Computed from shares | Tier-dependent |
| Coinbase payment amounts + fiat prices | Monero blockchain + CoinGecko | Tier-dependent |
| Coinbase private keys (per block) | P2Pool API (already public) | Indefinite |
| Subscription status + payment history | Operator wallet (view-only) | Indefinite |

**Not collected:** IP addresses, connection logs, browser fingerprints, email
addresses (unless voluntarily provided), or any data that correlates wallet
addresses to real-world identities.

### Coinbase Transparency

SideWatch publishes the coinbase private key for every P2Pool-found block.
This key is already available via the P2Pool API and allows anyone to
independently verify that coinbase outputs match the PPLNS share distribution.
This is P2Pool's built-in trustless audit mechanism — not exposing it would
make the dashboard less auditable than the underlying protocol.

### VPN Recommendation

While SideWatch does not log IP addresses, miners connecting to the P2Pool node
can be observed at the network level. For maximum privacy, miners should connect
to the P2Pool node through a VPN so their IP cannot be correlated with their
mining address by any network observer.

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

## Subscription Wallet

The optional XMR subscription system uses a **view-only** `monero-wallet-rpc`
instance to detect incoming operator revenue. This is distinct from the "no
wallet RPC" rule above, which prohibits custody of *miner* funds.

- The view-only wallet **cannot spend** — it only watches for incoming transfers
- The full spend key is kept offline by the operator
- `monero-wallet-rpc` runs on the internal Docker network only, with
  `--disable-rpc-login`, and is never exposed externally
- The wallet file is mounted read-only into the container
- If `WALLET_RPC_URL` is not set, the subscription system is entirely disabled

See [docs/subscription-setup.md](docs/subscription-setup.md) for setup details.

## Scope

### Sweep Transaction Guard

The coinbase scanner validates that every transaction it processes is a genuine
coinbase (generation) transaction by checking:
1. Exactly one input with a `gen` field
2. The gen height matches the expected block height

This prevents the scanner from accidentally misclassifying miner sweep/consolidation
transactions as new coinbase payments.

## Scope

The following are considered in-scope for security reports:

- Authentication bypass in the gateway or admin endpoints
- SQL injection or other database attacks
- Information disclosure (IP/address correlation, secret leakage)
- Container escape or privilege escalation
- Denial of service via resource exhaustion
- Subscription wallet credential exposure or unauthorized RPC access
- Sweep transaction misclassification bypassing the coinbase guard
- Data retention policy bypass (free-tier data surviving pruning)

The following are **out of scope**:

- Vulnerabilities in upstream dependencies (report to the upstream project)
- Social engineering attacks
- Issues that require physical access to the server
