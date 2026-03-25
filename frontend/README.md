# Frontend

Next.js 14 dashboard for P2Pool Monero miners. Dark-themed, Tailwind CSS,
real-time updates via WebSocket with SWR polling fallback.

## Pages

| Route | Description |
|---|---|
| `/` | Pool overview — live hashrate, miner count, recent blocks |
| `/miner` | Miner lookup by address — hashrate chart, shares, payments |
| `/blocks` | Block explorer — found blocks with effort and rewards |
| `/sidechain` | Sidechain share viewer |
| `/admin` | JWT-protected admin panel |

## Key Components

| Component | Description |
|---|---|
| `LiveStats` | Real-time pool stats with WebSocket and SWR fallback |
| `HashrateChart` | Recharts area chart for miner hashrate timeseries |
| `BlocksTable` | Paginated table of found Monero blocks |
| `PaymentsTable` | Miner payment history with XMR and fiat columns |
| `WorkersTable` | Active workers for a miner address |
| `SidechainTable` | Recent P2Pool sidechain shares |
| `Navigation` | Top nav bar with route links |
| `PrivacyNotice` | Coinbase transparency warning |

## Lib

- `lib/api.ts` — Typed interfaces matching Go API responses, SWR fetcher,
  formatting helpers (XMR amounts, hashrates, relative time, difficulty)
- `lib/useWebSocket.ts` — React hook for live data via WebSocket

## Running

```bash
npm install
npm run dev      # http://localhost:3000
npm run build    # production build
npm run lint
```

## Stack

- Next.js 14 (App Router)
- React 18
- TypeScript 5
- Tailwind CSS 3
- Recharts (charting)
- SWR (data fetching with revalidation)
