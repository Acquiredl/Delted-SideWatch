'use client'

import useSWR from 'swr'
import { useWebSocket } from '@/lib/useWebSocket'
import { fetcher, formatXMR, formatHashrate, formatRelativeTime, formatDifficulty } from '@/lib/api'
import type { PoolStats, NodeStatusResponse } from '@/lib/api'

interface StatCardProps {
  label: string
  value: string
  subtext?: string
  accent?: string
}

function StatCard({ label, value, subtext, accent }: StatCardProps) {
  return (
    <div className={`stat-card ${accent || ''}`}>
      <p className="text-zinc-400 text-sm mb-1">{label}</p>
      <p className="text-2xl font-bold text-zinc-100">{value}</p>
      {subtext && <p className="text-zinc-500 text-xs mt-1">{subtext}</p>}
    </div>
  )
}

function wsUrl(): string {
  if (typeof window === 'undefined') return ''
  // NEXT_PUBLIC_WS_URL points directly to the gateway (or nginx in prod).
  // Next.js rewrites only proxy HTTP, not browser WebSocket upgrades.
  const wsBase = process.env.NEXT_PUBLIC_WS_URL
  if (wsBase) {
    return `${wsBase}/ws/pool/stats`
  }
  // Fallback: same origin works when nginx fronts everything.
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  return `${protocol}//${window.location.host}/ws/pool/stats`
}

export default function LiveStats() {
  const ws = useWebSocket<PoolStats>(wsUrl())
  const { data: swrData, error, isLoading } = useSWR<PoolStats>('/api/pool/stats', fetcher, {
    refreshInterval: ws.isConnected ? 0 : 15000,
  })
  const { data: nodeData } = useSWR<NodeStatusResponse>('/api/nodes/status', fetcher, {
    refreshInterval: 60000,
  })

  const data = ws.data ?? swrData

  if (isLoading && !data) {
    return (
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        {Array.from({ length: 4 }).map((_, i) => (
          <div key={i} className="stat-card animate-pulse">
            <div className="h-4 bg-zinc-800 rounded w-24 mb-2" />
            <div className="h-8 bg-zinc-800 rounded w-32" />
          </div>
        ))}
      </div>
    )
  }

  if (error && !data) {
    return (
      <div className="text-red-400 text-sm p-4 bg-red-900/20 border border-red-800 rounded-lg">
        Failed to load pool stats: {error.message}
      </div>
    )
  }

  if (!data) return null

  return (
    <div>
      <div className="flex items-center gap-2 mb-4">
        <span
          className={`inline-block w-2 h-2 rounded-full ${
            ws.isConnected ? 'bg-green-500' : 'bg-zinc-600'
          }`}
          title={ws.isConnected ? 'Live updates active' : 'Using polling'}
        />
        <span className="text-zinc-500 text-xs">
          {ws.isConnected ? 'Live' : 'Polling'}
        </span>
      </div>
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard
          label="Pool Hashrate"
          value={formatHashrate(data.total_hashrate)}
          subtext={`${data.sidechain} sidechain`}
          accent="stat-card-orange"
        />
        <StatCard
          label="Active Miners"
          value={data.total_miners.toLocaleString()}
          accent="stat-card-blue"
        />
        <StatCard
          label="Sidechain Height"
          value={data.sidechain_height ? data.sidechain_height.toLocaleString() : '--'}
          subtext={data.sidechain_difficulty ? `Diff: ${formatDifficulty(data.sidechain_difficulty)}` : undefined}
          accent="stat-card-green"
        />
        <StatCard
          label="Blocks Found"
          value={data.blocks_found.toLocaleString()}
          subtext={
            data.blocks_found > 0 && data.last_block_found_at
              ? `Last: ${formatRelativeTime(data.last_block_found_at)}`
              : 'Waiting for next block'
          }
          accent="stat-card-yellow"
        />
      </div>
      {data.total_paid > 0 && (
        <div className="mt-4">
          <StatCard
            label="Total Paid"
            value={`${formatXMR(data.total_paid)} XMR`}
          />
        </div>
      )}
      {nodeData && nodeData.nodes.length > 0 && (
        <div className="flex items-center gap-4 mt-3">
          {nodeData.nodes.map((node) => {
            const dotColor =
              node.status === 'healthy' ? 'bg-green-500'
              : node.status === 'syncing' ? 'bg-yellow-500'
              : 'bg-red-500'
            return (
              <span key={node.name} className="flex items-center gap-1.5 text-xs text-zinc-500">
                <span className={`inline-block w-1.5 h-1.5 rounded-full ${dotColor}`} />
                {node.name}
              </span>
            )
          })}
        </div>
      )}
    </div>
  )
}
