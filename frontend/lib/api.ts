const API_BASE = process.env.NEXT_PUBLIC_API_URL || ''

// --- Type definitions matching Go API responses ---

export interface PoolStats {
  total_miners: number
  total_hashrate: number
  blocks_found: number
  last_block_found_at: string
  total_paid: number
  sidechain: string
}

export interface MinerStats {
  address: string
  current_hashrate: number
  average_hashrate: number
  total_shares: number
  total_paid: number
  last_share_at: string
  last_payment_at: string
}

export interface MinerPayment {
  amount: number
  main_height: number
  xmr_usd_price: number
  xmr_cad_price: number
  paid_at: string
}

export interface HashratePoint {
  hashrate: number
  bucket_time: string
}

export interface FoundBlock {
  main_height: number
  main_hash: string
  sidechain_height: number
  coinbase_reward: number
  effort: number
  found_at: string
}

export interface SidechainShare {
  miner_address: string
  worker_name: string
  sidechain: string
  sidechain_height: number
  difficulty: number
  created_at: string
}

export interface HealthStatus {
  status: string
  postgres: string
  redis: string
}

// --- SWR fetcher ---

export const fetcher = async (url: string): Promise<unknown> => {
  const res = await fetch(`${API_BASE}${url}`)
  if (!res.ok) {
    throw new Error(`API error: ${res.status}`)
  }
  return res.json()
}

// --- Formatting helpers ---

export function formatXMR(atomic: number): string {
  return (atomic / 1e12).toFixed(4)
}

export function formatHashrate(hs: number): string {
  if (hs >= 1e9) return (hs / 1e9).toFixed(2) + ' GH/s'
  if (hs >= 1e6) return (hs / 1e6).toFixed(2) + ' MH/s'
  if (hs >= 1e3) return (hs / 1e3).toFixed(2) + ' KH/s'
  return hs.toFixed(0) + ' H/s'
}

export function truncateAddress(addr: string): string {
  if (addr.length <= 16) return addr
  return addr.slice(0, 8) + '...' + addr.slice(-8)
}

export function formatRelativeTime(dateStr: string): string {
  const date = new Date(dateStr)
  const now = new Date()
  const diffMs = now.getTime() - date.getTime()
  const diffSec = Math.floor(diffMs / 1000)
  const diffMin = Math.floor(diffSec / 60)
  const diffHr = Math.floor(diffMin / 60)
  const diffDay = Math.floor(diffHr / 24)

  if (diffSec < 60) return `${diffSec}s ago`
  if (diffMin < 60) return `${diffMin}m ago`
  if (diffHr < 24) return `${diffHr}h ${diffMin % 60}m ago`
  return `${diffDay}d ${diffHr % 24}h ago`
}

export function formatDate(dateStr: string): string {
  const date = new Date(dateStr)
  return date.toLocaleDateString('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
}

export function formatEffort(effort: number): string {
  return (effort * 100).toFixed(1) + '%'
}

export function formatDifficulty(diff: number): string {
  if (diff >= 1e12) return (diff / 1e12).toFixed(2) + ' T'
  if (diff >= 1e9) return (diff / 1e9).toFixed(2) + ' G'
  if (diff >= 1e6) return (diff / 1e6).toFixed(2) + ' M'
  if (diff >= 1e3) return (diff / 1e3).toFixed(2) + ' K'
  return diff.toString()
}

export function formatUSD(value: number): string {
  return '$' + value.toFixed(2)
}

export function formatCAD(value: number): string {
  return 'C$' + value.toFixed(2)
}
