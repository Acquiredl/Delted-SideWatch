'use client'

import { useState } from 'react'
import useSWR from 'swr'
import {
  ResponsiveContainer, AreaChart, Area, XAxis, YAxis, Tooltip, CartesianGrid,
  LineChart, Line,
} from 'recharts'
import { fetcher, formatHashrate, formatDifficulty } from '@/lib/api'
import type { PoolStatsPoint } from '@/lib/api'

const HOUR_OPTIONS = [
  { label: '6h', value: 6 },
  { label: '24h', value: 24 },
  { label: '3d', value: 72 },
  { label: '7d', value: 168 },
]

function formatTime(dateStr: string): string {
  const d = new Date(dateStr)
  return d.toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit' })
}

function formatDateTime(dateStr: string): string {
  const d = new Date(dateStr)
  return d.toLocaleString('en-US', {
    month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit',
  })
}

interface TooltipPayloadItem {
  value: number
  dataKey: string
  payload: PoolStatsPoint
}

interface CustomTooltipProps {
  active?: boolean
  payload?: TooltipPayloadItem[]
}

function HashrateTooltip({ active, payload }: CustomTooltipProps) {
  if (!active || !payload || payload.length === 0) return null
  const point = payload[0]
  return (
    <div className="bg-zinc-900 border border-zinc-700 rounded-lg p-3 text-sm">
      <p className="text-zinc-400">{formatDateTime(point.payload.created_at)}</p>
      <p className="text-xmr-orange font-bold">{formatHashrate(point.value)}</p>
      <p className="text-zinc-400">{point.payload.pool_miners} miners</p>
    </div>
  )
}

function DifficultyTooltip({ active, payload }: CustomTooltipProps) {
  if (!active || !payload || payload.length === 0) return null
  const point = payload[0]
  return (
    <div className="bg-zinc-900 border border-zinc-700 rounded-lg p-3 text-sm">
      <p className="text-zinc-400">{formatDateTime(point.payload.created_at)}</p>
      <p className="text-cyan-400 font-bold">Diff: {formatDifficulty(point.payload.sidechain_difficulty)}</p>
      <p className="text-zinc-400">Height: {point.payload.sidechain_height.toLocaleString()}</p>
    </div>
  )
}

export default function SidechainPage() {
  const [hours, setHours] = useState(24)

  const { data: points, isLoading, error } = useSWR<PoolStatsPoint[]>(
    `/api/sidechain/stats?hours=${hours}`,
    fetcher,
    { refreshInterval: 30000 }
  )

  const latest = points && points.length > 0 ? points[points.length - 1] : null

  return (
    <div>
      <div className="mb-8">
        <h1 className="text-3xl font-bold text-zinc-100 mb-2">Sidechain Overview</h1>
        <p className="text-zinc-400 text-sm">
          Live P2Pool sidechain metrics — pool hashrate, difficulty, and miner count over time.
        </p>
      </div>

      {/* Time range selector */}
      <div className="flex gap-2 mb-6">
        {HOUR_OPTIONS.map((opt) => (
          <button
            key={opt.value}
            onClick={() => setHours(opt.value)}
            className={`px-3 py-1.5 rounded text-sm font-medium transition-colors ${
              hours === opt.value
                ? 'bg-xmr-orange text-zinc-900'
                : 'bg-zinc-800 text-zinc-400 hover:text-zinc-200'
            }`}
          >
            {opt.label}
          </button>
        ))}
      </div>

      {/* Current stats summary */}
      {latest && (
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4 mb-8">
          <div className="stat-card">
            <p className="text-zinc-400 text-sm mb-1">Pool Hashrate</p>
            <p className="text-2xl font-bold text-zinc-100">{formatHashrate(latest.pool_hashrate)}</p>
          </div>
          <div className="stat-card">
            <p className="text-zinc-400 text-sm mb-1">Active Miners</p>
            <p className="text-2xl font-bold text-zinc-100">{latest.pool_miners.toLocaleString()}</p>
          </div>
          <div className="stat-card">
            <p className="text-zinc-400 text-sm mb-1">Sidechain Height</p>
            <p className="text-2xl font-bold text-zinc-100">{latest.sidechain_height.toLocaleString()}</p>
          </div>
          <div className="stat-card">
            <p className="text-zinc-400 text-sm mb-1">Sidechain Difficulty</p>
            <p className="text-2xl font-bold text-zinc-100">{formatDifficulty(latest.sidechain_difficulty)}</p>
          </div>
        </div>
      )}

      {error && (
        <div className="text-red-400 text-sm p-4 bg-red-900/20 border border-red-800 rounded-lg mb-6">
          Failed to load sidechain stats: {error.message}
        </div>
      )}

      {isLoading && !points && (
        <div className="space-y-6">
          <div className="stat-card animate-pulse h-[340px]" />
          <div className="stat-card animate-pulse h-[340px]" />
        </div>
      )}

      {points && points.length === 0 && (
        <div className="stat-card text-center text-zinc-500 py-12">
          No sidechain data collected yet. The indexer records snapshots every 30 seconds
          — data will appear here shortly after the pool starts.
        </div>
      )}

      {points && points.length > 0 && (
        <div className="space-y-6">
          {/* Pool Hashrate Chart */}
          <div className="stat-card">
            <h3 className="text-zinc-400 text-sm mb-4">Pool Hashrate</h3>
            <ResponsiveContainer width="100%" height={300}>
              <AreaChart data={points} margin={{ top: 5, right: 5, left: 0, bottom: 5 }}>
                <defs>
                  <linearGradient id="poolHashGradient" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="5%" stopColor="#f97316" stopOpacity={0.3} />
                    <stop offset="95%" stopColor="#f97316" stopOpacity={0} />
                  </linearGradient>
                </defs>
                <CartesianGrid strokeDasharray="3 3" stroke="#27272a" />
                <XAxis
                  dataKey="created_at"
                  tickFormatter={hours <= 24 ? formatTime : formatDateTime}
                  stroke="#71717a"
                  tick={{ fontSize: 12 }}
                />
                <YAxis
                  tickFormatter={(v: number) => formatHashrate(v)}
                  stroke="#71717a"
                  tick={{ fontSize: 12 }}
                  width={80}
                />
                <Tooltip content={<HashrateTooltip />} />
                <Area
                  type="monotone"
                  dataKey="pool_hashrate"
                  stroke="#f97316"
                  strokeWidth={2}
                  fill="url(#poolHashGradient)"
                />
              </AreaChart>
            </ResponsiveContainer>
          </div>

          {/* Sidechain Difficulty Chart */}
          <div className="stat-card">
            <h3 className="text-zinc-400 text-sm mb-4">Sidechain Difficulty</h3>
            <ResponsiveContainer width="100%" height={300}>
              <LineChart data={points} margin={{ top: 5, right: 5, left: 0, bottom: 5 }}>
                <CartesianGrid strokeDasharray="3 3" stroke="#27272a" />
                <XAxis
                  dataKey="created_at"
                  tickFormatter={hours <= 24 ? formatTime : formatDateTime}
                  stroke="#71717a"
                  tick={{ fontSize: 12 }}
                />
                <YAxis
                  tickFormatter={(v: number) => formatDifficulty(v)}
                  stroke="#71717a"
                  tick={{ fontSize: 12 }}
                  width={80}
                />
                <Tooltip content={<DifficultyTooltip />} />
                <Line
                  type="monotone"
                  dataKey="sidechain_difficulty"
                  stroke="#06b6d4"
                  strokeWidth={2}
                  dot={false}
                />
              </LineChart>
            </ResponsiveContainer>
          </div>
        </div>
      )}
    </div>
  )
}
