'use client'

import { ResponsiveContainer, AreaChart, Area, XAxis, YAxis, Tooltip, CartesianGrid } from 'recharts'
import { formatHashrate } from '@/lib/api'
import type { HashratePoint } from '@/lib/api'

interface HashrateChartProps {
  data: HashratePoint[]
}

function formatTime(dateStr: string): string {
  const d = new Date(dateStr)
  return d.toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit' })
}

interface TooltipPayloadItem {
  value: number
  payload: HashratePoint
}

interface CustomTooltipProps {
  active?: boolean
  payload?: TooltipPayloadItem[]
  label?: string
}

function CustomTooltip({ active, payload }: CustomTooltipProps) {
  if (!active || !payload || payload.length === 0) return null

  const point = payload[0]
  return (
    <div className="bg-zinc-900 border border-zinc-700 rounded-lg p-3 text-sm">
      <p className="text-zinc-400">{new Date(point.payload.bucket_time).toLocaleString()}</p>
      <p className="text-xmr-orange font-bold">{formatHashrate(point.value)}</p>
    </div>
  )
}

export default function HashrateChart({ data }: HashrateChartProps) {
  if (data.length === 0) {
    return (
      <div className="stat-card text-center text-zinc-500 py-12">
        No hashrate data available
      </div>
    )
  }

  return (
    <div className="stat-card">
      <h3 className="text-zinc-400 text-sm mb-4">Hashrate (24h)</h3>
      <ResponsiveContainer width="100%" height={300}>
        <AreaChart data={data} margin={{ top: 5, right: 5, left: 0, bottom: 5 }}>
          <defs>
            <linearGradient id="hashrateGradient" x1="0" y1="0" x2="0" y2="1">
              <stop offset="5%" stopColor="#f97316" stopOpacity={0.3} />
              <stop offset="95%" stopColor="#f97316" stopOpacity={0} />
            </linearGradient>
          </defs>
          <CartesianGrid strokeDasharray="3 3" stroke="#27272a" />
          <XAxis
            dataKey="bucket_time"
            tickFormatter={formatTime}
            stroke="#71717a"
            tick={{ fontSize: 12 }}
          />
          <YAxis
            tickFormatter={(v: number) => formatHashrate(v)}
            stroke="#71717a"
            tick={{ fontSize: 12 }}
            width={80}
          />
          <Tooltip content={<CustomTooltip />} />
          <Area
            type="monotone"
            dataKey="hashrate"
            stroke="#f97316"
            strokeWidth={2}
            fill="url(#hashrateGradient)"
          />
        </AreaChart>
      </ResponsiveContainer>
    </div>
  )
}
