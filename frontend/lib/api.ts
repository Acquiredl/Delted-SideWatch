const API_BASE = process.env.NEXT_PUBLIC_API_URL || ''

// --- Type definitions matching Go API responses ---

export interface PoolStats {
  total_miners: number
  total_hashrate: number
  blocks_found: number
  last_block_found_at: string
  total_paid: number
  sidechain: string
  sidechain_height: number
  sidechain_difficulty: number
}

export interface MinerStats {
  address: string
  current_hashrate: number
  average_hashrate: number
  total_shares: number
  total_paid: number
  last_share_at: string
  last_payment_at: string
  uncle_rate_24h: number | null
}

export interface MinerPayment {
  amount: number
  main_height: number
  xmr_usd_price: number | null
  xmr_cad_price: number | null
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
  coinbase_private_key: string | null
  found_at: string
}

export interface SidechainShare {
  miner_address: string
  worker_name: string
  sidechain: string
  sidechain_height: number
  difficulty: number
  is_uncle: boolean
  software_id: number | null
  software_version: string | null
  created_at: string
}

export interface PoolStatsPoint {
  pool_hashrate: number
  pool_miners: number
  sidechain_height: number
  sidechain_difficulty: number
  created_at: string
}

export interface LocalWorker {
  miner_address: string
  current_hashrate: number
  last_seen: string
}

export interface HealthStatus {
  status: string
  postgres: string
  redis: string
}

// --- Subscription types (matching Go subscription.types) ---

export type SubscriptionTier = 'free' | 'supporter' | 'champion'

export interface SubscriptionStatus {
  miner_address: string
  tier: SubscriptionTier
  active: boolean
  expires_at: string | null
  grace_until: string | null
  has_api_key: boolean
}

/** Returns true if actual tier meets or exceeds the required tier. */
export function tierIncludes(actual: SubscriptionTier, required: SubscriptionTier): boolean {
  const hierarchy: Record<SubscriptionTier, number> = { free: 0, supporter: 1, champion: 2 }
  return hierarchy[actual] >= hierarchy[required]
}

export interface PaymentAddress {
  miner_address: string
  subaddress: string
  suggested_amount_xmr: string
  amount_usd: string
}

export interface SubPayment {
  id: number
  miner_address: string
  tx_hash: string
  amount: number
  xmr_usd_price: number | null
  xmr_cad_price: number | null
  confirmed: boolean
  main_height: number | null
  created_at: string
}

// --- Fund types (matching Go fund.types) ---

export interface FundStatus {
  month: string
  goal_usd: number
  funded_usd: number
  percent_funded: number
  infra_cost_usd: number
  supporter_count: number
  node_count: number
  nodes: FundNode[]
}

export interface FundNode {
  name: string
  sidechain: string
  status: string
  miners: number | null
}

export interface FundMonth {
  month: string
  goal_usd: number
  funded_usd: number
  supporter_count: number
}

export interface FundSupporter {
  address: string
  tier: string
  since: string
}

// --- Node pool types (matching Go nodepool.types) ---

export interface NodeStatusResponse {
  nodes: NodeSummary[]
}

export interface NodeSummary {
  name: string
  sidechain: string
  status: string
  miners: number | null
  hashrate: number | null
  peers: number | null
  last_health_at: string | null
}

export interface ConnectionInfoResponse {
  nodes: NodeConnectionInfo[]
  onion_url: string
}

export interface NodeConnectionInfo {
  name: string
  sidechain: string
  status: string
  stratum_url: string
  xmrig_config: {
    url: string
    user: string
    pass: string
  }
}

export interface PaymentYearSummary {
  year: number
  payment_count: number
  total_atomic: number
  total_cad: number | null
  total_usd: number | null
}

export interface MinerWorker {
  worker_name: string
  shares: number
  last_share_at: string
}

export interface WeeklyMiner {
  address: string
  share_count: number
  last_share_at: string
}

// --- SWR fetcher ---

export const fetcher = async <T = unknown>(url: string): Promise<T> => {
  const res = await fetch(`${API_BASE}${url}`)
  if (!res.ok) {
    throw new Error(`API error: ${res.status}`)
  }
  return res.json() as Promise<T>
}

// --- POST helper ---

export async function postJSON<T = unknown>(url: string, body?: Record<string, unknown>, headers?: Record<string, string>): Promise<T> {
  const opts: RequestInit = { method: 'POST', headers: { ...headers } }
  if (body) {
    (opts.headers as Record<string, string>)['Content-Type'] = 'application/json'
    opts.body = JSON.stringify(body)
  }
  const res = await fetch(`${API_BASE}${url}`, opts)
  if (!res.ok) {
    throw new Error(`API error: ${res.status}`)
  }
  return res.json() as Promise<T>
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

export function formatDuration(seconds: number): string {
  if (seconds < 60) return `~${Math.round(seconds)}s`
  const mins = Math.floor(seconds / 60)
  if (mins < 60) return `~${mins}m`
  const hrs = Math.floor(mins / 60)
  const remMins = mins % 60
  if (hrs < 24) return `~${hrs}h ${remMins}m`
  const days = Math.floor(hrs / 24)
  const remHrs = hrs % 24
  return `~${days}d ${remHrs}h`
}

export function formatUSD(value: number): string {
  return '$' + value.toFixed(2)
}

export function formatCAD(value: number): string {
  return 'C$' + value.toFixed(2)
}

const softwareNames: Record<number, string> = {
  0: 'P2Pool',
  1: 'XMRig',
  2: 'XMRig-Mo',
}

export function formatSoftware(id: number | null, version: string | null): string {
  if (id == null) return '—'
  const name = softwareNames[id] ?? `Unknown(${id})`
  return version ? `${name} ${version}` : name
}
