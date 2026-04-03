'use client'

import useSWR from 'swr'
import { fetcher, formatUSD } from '@/lib/api'
import type { FundMonth } from '@/lib/api'

export default function FundHistory() {
  const { data, error, isLoading } = useSWR<FundMonth[]>('/api/fund/history', fetcher)

  if (isLoading) {
    return (
      <div className="stat-card animate-pulse">
        <div className="h-4 bg-zinc-800 rounded w-40 mb-3" />
        <div className="h-32 bg-zinc-800 rounded" />
      </div>
    )
  }

  if (error || !data || data.length === 0) {
    return (
      <div className="stat-card">
        <h3 className="text-sm font-medium text-zinc-400 mb-3">Funding History</h3>
        <p className="text-zinc-500 text-sm">No history yet. Contributions start appearing here after the first month.</p>
      </div>
    )
  }

  // Show oldest first for the chart.
  const months = [...data].reverse()
  const maxVal = Math.max(...months.map(m => Math.max(m.funded_usd, m.goal_usd)), 1)

  return (
    <div className="stat-card">
      <h3 className="text-sm font-medium text-zinc-400 mb-4">Funding History</h3>

      <div className="flex items-end gap-2 h-40">
        {months.map((month) => {
          const fundedH = (month.funded_usd / maxVal) * 100
          const goalH = (month.goal_usd / maxVal) * 100
          const met = month.funded_usd >= month.goal_usd

          return (
            <div key={month.month} className="flex-1 flex flex-col items-center gap-1">
              <div className="w-full relative flex items-end justify-center" style={{ height: '120px' }}>
                {/* Goal line */}
                <div
                  className="absolute w-full border-t border-dashed border-zinc-600"
                  style={{ bottom: `${goalH}%` }}
                />
                {/* Funded bar */}
                <div
                  className={`w-3/4 rounded-t ${met ? 'bg-green-600' : 'bg-blue-600'}`}
                  style={{ height: `${fundedH}%`, minHeight: '2px' }}
                />
              </div>
              <span className="text-[10px] text-zinc-500">{month.month.slice(5)}</span>
            </div>
          )
        })}
      </div>

      {/* Legend */}
      <div className="flex items-center gap-4 mt-3 text-xs text-zinc-500">
        <span className="flex items-center gap-1">
          <span className="inline-block w-2 h-2 bg-green-600 rounded" /> Goal met
        </span>
        <span className="flex items-center gap-1">
          <span className="inline-block w-2 h-2 bg-blue-600 rounded" /> Funded
        </span>
        <span className="flex items-center gap-1">
          <span className="inline-block w-3 border-t border-dashed border-zinc-600" /> Goal
        </span>
      </div>

      {/* Table summary */}
      <div className="mt-4 border-t border-zinc-800 pt-3">
        <table className="w-full text-xs">
          <thead>
            <tr className="text-zinc-500">
              <th className="text-left pb-1">Month</th>
              <th className="text-right pb-1">Funded</th>
              <th className="text-right pb-1">Goal</th>
              <th className="text-right pb-1">Supporters</th>
            </tr>
          </thead>
          <tbody>
            {data.map((m) => (
              <tr key={m.month} className="text-zinc-400">
                <td className="py-0.5">{m.month}</td>
                <td className="text-right font-mono">{formatUSD(m.funded_usd)}</td>
                <td className="text-right font-mono">{formatUSD(m.goal_usd)}</td>
                <td className="text-right">{m.supporter_count}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}
