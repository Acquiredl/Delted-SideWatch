'use client'

import useSWR from 'swr'
import { fetcher, formatUSD } from '@/lib/api'
import type { FundStatus } from '@/lib/api'

interface FundProgressProps {
  compact?: boolean
}

export default function FundProgress({ compact }: FundProgressProps) {
  const { data, error, isLoading } = useSWR<FundStatus>('/api/fund/status', fetcher, {
    refreshInterval: 60000,
  })

  if (isLoading) {
    return (
      <div className="stat-card animate-pulse">
        <div className="h-4 bg-zinc-800 rounded w-40 mb-3" />
        <div className="h-6 bg-zinc-800 rounded mb-2" />
        <div className="h-3 bg-zinc-800 rounded w-32" />
      </div>
    )
  }

  if (error || !data) return null

  const pct = Math.min(data.percent_funded, 100)
  const barColor = pct >= 100 ? 'bg-green-500' : pct >= 50 ? 'bg-blue-500' : 'bg-amber-500'

  return (
    <div className="stat-card">
      <div className="flex items-center justify-between mb-2">
        <h3 className="text-sm font-medium text-zinc-400">
          Node Fund &mdash; {data.month}
        </h3>
        <span className="text-sm text-zinc-300 font-mono">
          {data.percent_funded}% funded
        </span>
      </div>

      {/* Progress bar */}
      <div className="w-full bg-zinc-800 rounded-full h-3 mb-3">
        <div
          className={`${barColor} h-3 rounded-full transition-all duration-500`}
          style={{ width: `${pct}%` }}
        />
      </div>

      <div className="flex items-center justify-between text-sm mb-2">
        <span className="text-zinc-300 font-mono">
          {formatUSD(data.funded_usd)} / {formatUSD(data.goal_usd)}
        </span>
        <span className="text-zinc-500">
          {data.supporter_count} supporter{data.supporter_count !== 1 ? 's' : ''}
        </span>
      </div>

      {!compact && (
        <div className="border-t border-zinc-800 pt-3 mt-3 text-xs text-zinc-500 space-y-1">
          <div className="flex justify-between">
            <span>Infrastructure</span>
            <span className="font-mono">{formatUSD(data.infra_cost_usd)}/mo</span>
          </div>
          <div className="flex justify-between">
            <span>Operator + maintenance</span>
            <span className="font-mono">{formatUSD(data.goal_usd - data.infra_cost_usd)}/mo</span>
          </div>
          {data.nodes.length > 0 && (
            <div className="flex justify-between mt-1">
              <span>Shared nodes</span>
              <span>{data.nodes.map(n => n.name).join(', ')}</span>
            </div>
          )}
        </div>
      )}
    </div>
  )
}
