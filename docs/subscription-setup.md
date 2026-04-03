# Subscription System -- Operator Setup

This guide covers how to set up the XMR subscription payment system for dashboard
operators. Miners pay a small XMR amount to unlock paid-tier features (extended
hashrate history, unlimited payment records, CSV tax export).

## How It Works

1. A miner requests a payment address via the API. The manager assigns a unique
   Monero subaddress per miner by calling `monero-wallet-rpc`.
2. The miner sends XMR to that subaddress.
3. The subscription scanner polls `monero-wallet-rpc` every 60 seconds for new
   incoming transfers. Once a transfer has 10 confirmations and meets the minimum
   USD threshold, the miner's subscription is activated for 30 days (configurable).
4. A 48-hour grace period extends beyond expiry so miners don't lose access the
   instant their subscription lapses.

## Security Model

The subscription wallet is **view-only**. It can detect incoming payments but
cannot spend funds. The operator (you) holds the full spend key offline. This
is fundamentally different from the "no wallet RPC" rule in [SECURITY.md](SECURITY.md), which
prohibits custody of *miner* funds. The subscription wallet holds only *operator*
revenue.

- `monero-wallet-rpc` runs with `--disable-rpc-login` on the internal Docker
  network only. It is never exposed externally.
- The wallet file is mounted read-only into the container.
- No spend operations are ever called by the dashboard code.

## Prerequisites

- A running `monerod` node (already required by the dashboard).
- The Monero CLI tools (`monero-wallet-cli`) installed on a secure machine for
  one-time wallet creation. These do NOT need to be on the server.

## Step 1: Create the Subscription Wallet

On a secure machine with the Monero CLI tools:

```bash
# Generate a new wallet. Save the mnemonic seed securely -- this is your
# spend key backup. You will NEVER put the full wallet on the server.
monero-wallet-cli --generate-new-wallet subscription-wallet

# Once inside the wallet CLI, note the primary address and the secret view key:
#   address
#   viewkey
```

Record these two values:
- **Primary address** -- this is the base address for all subaddresses
- **Secret view key** -- needed to create the view-only wallet

## Step 2: Create the View-Only Wallet

Still on the secure machine:

```bash
monero-wallet-cli \
  --generate-from-view-key subscription-viewonly \
  --address <PRIMARY_ADDRESS> \
  --viewkey <SECRET_VIEW_KEY> \
  --daemon-address <YOUR_MONEROD_HOST>:18081
```

This creates two files:
- `subscription-viewonly` (the wallet file)
- `subscription-viewonly.keys` (the view-only keys file)

You can also create the view-only wallet from the full wallet:

```bash
# Inside the full wallet CLI session:
# (this avoids needing to manually pass address + viewkey)
monero-wallet-cli --wallet-file subscription-wallet
> password: ****
> viewonly subscription-viewonly
```

## Step 3: Deploy the Wallet Files

Copy both files to the server:

```bash
# On the server, create the secrets directory:
mkdir -p secrets/wallet

# Copy the view-only wallet files:
scp subscription-viewonly subscription-viewonly.keys server:~/p2pool-dashboard/secrets/wallet/

# Lock down permissions:
chmod 600 secrets/wallet/*
```

The `docker-compose.yml` mounts this directory read-only into the `wallet-rpc`
container at `/wallet/`.

## Step 4: Configure docker-compose

The `wallet-rpc` service is already defined in `docker-compose.yml`:

```yaml
wallet-rpc:
  image: sethsimmons/simple-monero-wallet-rpc:latest
  command: >
    monero-wallet-rpc
    --daemon-address=monerod:18081
    --wallet-file=/wallet/subscription-viewonly
    --password=""
    --rpc-bind-port=18088
    --rpc-bind-ip=0.0.0.0
    --confirm-external-bind
    --disable-rpc-login
  volumes:
    - ./secrets/wallet:/wallet:ro
  restart: unless-stopped
  networks:
    - backend
```

If your wallet has a password, replace `--password=""` with `--password=<pw>` or
mount a Docker secret. For simplicity, a view-only wallet with an empty password
is reasonable since the wallet cannot spend.

## Step 5: Set Environment Variables

Add to your `.env` file:

```bash
# Subscription payment scanning (optional -- leave empty to disable)
WALLET_RPC_URL=http://wallet-rpc:18088

# Minimum USD value to accept as a valid subscription payment (default: 4.00)
SUBSCRIPTION_MIN_USD=4.0

# Duration of a subscription in days (default: 30)
SUBSCRIPTION_DURATION_DAYS=30

# Grace period in hours after expiry (default: 48)
SUBSCRIPTION_GRACE_HOURS=48
```

If `WALLET_RPC_URL` is empty or unset, the subscription scanner is disabled and
all miners remain on the free tier. The rest of the dashboard works normally.

## Step 6: Verify

After `docker compose up -d`:

```bash
# Check wallet-rpc is responding:
curl -s http://localhost:18088/json_rpc \
  -d '{"jsonrpc":"2.0","id":"0","method":"get_height"}' \
  -H 'Content-Type: application/json'

# Check the manager detected the wallet:
docker compose logs manager | grep "subscription scanner"
# Should show: "subscription scanner starting" with config values

# Request a payment address for a test miner:
curl http://localhost:8081/api/subscription/address/4TESTADDRESS...
```

## Tier Limits

| Feature | Free | Paid |
|---|---|---|
| Hashrate history | 720 hours (30 days) | Unlimited |
| Payment records per request | 100 | Unlimited |
| Tax export (CSV) | Blocked | Full history |
| API key authentication | N/A | Supported |

## Operational Notes

**Wallet sync time:** When first deployed, `monero-wallet-rpc` must sync the
view-only wallet to the current blockchain height. This can take minutes to
hours depending on when the wallet was created. The scanner will log errors
during this period -- this is expected.

**Subaddress persistence:** Each miner's subaddress assignment is stored in
PostgreSQL (`subscription_addresses` table). The wallet-rpc generates subaddresses
deterministically from the view key, so they survive container restarts. However,
if you recreate the wallet from scratch, you must also clear the
`subscription_addresses` table.

**Backup:** The view-only wallet files do not change at runtime (subaddresses are
derived, not written to disk by wallet-rpc). Back up the full spend wallet and
mnemonic seed offline. The view-only files can be regenerated from the spend key.

**Price oracle:** The scanner uses CoinGecko (via the existing price oracle) to
convert XMR amounts to USD for minimum threshold validation. If the price oracle
is unavailable, payments are accepted conservatively (better to grant access than
reject a valid payment).

**Multiple payments:** If a miner sends additional payments while already
subscribed, the subscription is extended from the current expiry date, not from
now. Payments stack.
