'use client'

import useSWR from 'swr'
import { useWebSocket } from '@/lib/useWebSocket'
import { fetcher, formatXMR, formatHashrate, formatRelativeTime } from '@/lib/api'
import type { PoolStats } from '@/lib/api'

interface StatCardProps {
  label: string
  value: string
  subtext?: string
}

function StatCard({ label, value, subtext }: StatCardProps) {
  return (
    <div className="stat-card">
      <p className="text-zinc-400 text-sm mb-1">{label}</p>
      <p className="text-2xl font-bold text-zinc-100">{value}</p>
      {subtext && <p className="text-zinc-500 text-xs mt-1">{subtext}</p>}
    </div>
  )
}

function wsUrl(): string {
  if (typeof window === 'undefined') return ''
  const base = process.env.NEXT_PUBLIC_API_URL || window.location.origin
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const host = base.replace(/^https?:\/\//, '').replace(/\/$/, '')
  return `${protocol}//${host}/ws/pool/stats`
}

export default function LiveStats() {
  const ws = useWebSocket<PoolStats>(wsUrl())
  const { data: swrData, error, isLoading } = useSWR<PoolStats>('/api/pool/stats', fetcher, {
    refreshInterval: ws.isConnected ? 0 : 15000,
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
          label="Total Hashrate"
          value={formatHashrate(data.total_hashrate)}
          subtext={`${data.sidechain} sidechain`}
        />
        <StatCard
          label="Active Miners"
          value={data.total_miners.toLocaleString()}
        />
        <StatCard
          label="Blocks Found"
          value={data.blocks_found.toLocaleString()}
          subtext={data.last_block_found_at ? `Last: ${formatRelativeTime(data.last_block_found_at)}` : undefined}
        />
        <StatCard
          label="Total Paid"
          value={`${formatXMR(data.total_paid)} XMR`}
        />
      </div>
    </div>
  )
}
