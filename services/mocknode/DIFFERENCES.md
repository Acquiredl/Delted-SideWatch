# Mocknode vs Real Node Differences

Known differences between the mock simulator and live P2Pool + monerod nodes.
Use this as a checklist when validating against production.

## P2Pool API

| Endpoint | Mocknode | Real Node | Impact |
|---|---|---|---|
| `/api/pool/stats` | Static PPLNS window (2160), randomized hashrate | Dynamic window size, real hashrate | PPLNS window differs between mini (2160) and main (varies) |
| `/api/shares` | Synthetic shares from 3 fixed addresses | Real shares from actual miners | Address format, share difficulty ranges differ |
| `/api/found_blocks` | Generated every ~90s, guaranteed data | Blocks found at real effort (~hours-days) | May be empty for long periods on live |
| `/api/worker_stats` | 3 hardcoded miners with fake stats | All active miners in PPLNS window | Scale: production may have hundreds of miners |
| `/api/p2p/peers` | 3 static fake peers | Live P2P network peers | Not used by the dashboard, informational only |

## Monerod JSON-RPC

| Method | Mocknode | Real Node | Impact |
|---|---|---|---|
| `get_last_block_header` | Increments every ~15s | Real ~2min Monero block time | Block height advances faster in mock |
| `get_block` | Returns random hashes, fake coinbase tx | Real block data with valid coinbase | Coinbase scanning outputs won't match real addresses |
| `get_transactions` | Generates fake outputs split across 3 miners | Real coinbase with P2Pool payouts to PPLNS miners | Output matching logic untested against real data |

## Key Behavioral Differences

1. **Coinbase scanning**: Mocknode generates fake coinbase transactions that always have outputs for the 3 hardcoded miners. Real coinbase transactions contain outputs for all miners in the P2Pool PPLNS window. The scanner's address-matching logic against real P2Pool output keys has not been validated.

2. **ZMQ events**: Mocknode does not provide ZMQ. The dashboard's ZMQ listener (`internal/events/zmq.go`) falls back to polling when ZMQ is unavailable. Against a real monerod, ZMQ delivers block events with lower latency.

3. **Timing**: Mocknode generates shares every 10s and blocks every 15s to keep the pipeline fed. Real P2Pool mini shares appear every few seconds across all miners; main chain blocks every ~2 minutes. Found blocks depend on pool luck.

4. **PPLNS window size**: Mocknode hardcodes 2160 (mini). Real P2Pool reports the actual window size which varies slightly. Main sidechain uses a much larger window.

5. **Price oracle**: Mocknode serves a fake CoinGecko response on the monerod port. In production, the price oracle hits the real CoinGecko API (or is disabled).

6. **Share difficulty**: Mocknode generates difficulties in the 200-350 MH range. Real mini sidechain target is ~300 MH; main sidechain target is much higher.

## Validation Checklist

Run `infra/scripts/validate-node.sh` against your production deployment, then manually verify:

- [ ] Pool stats show realistic hashrate and miner count
- [ ] Shares are being indexed (check `/api/sidechain/shares`)
- [ ] Blocks are recorded when found (check `/api/blocks` — may take hours)
- [ ] Coinbase scanner correctly matches outputs to miner addresses
- [ ] Payments table populates after a block is found and confirmed (10+ confirmations)
- [ ] Price oracle returns real XMR/USD and XMR/CAD prices
- [ ] WebSocket delivers live updates (open browser, check for "Live" indicator)
- [ ] Prometheus metrics incrementing (`pool_shares_indexed_total`, etc.)
- [ ] No crash loops or repeated error logs in `docker compose logs manager`
