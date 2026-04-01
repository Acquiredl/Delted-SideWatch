'use client'

import useSWR from 'swr'
import { fetcher } from '@/lib/api'
import type { FundSupporter } from '@/lib/api'

export default function SupportersPage() {
  const { data, isLoading } = useSWR<FundSupporter[]>('/api/fund/supporters', fetcher, {
    refreshInterval: 60000,
  })

  if (isLoading) {
    return (
      <div className="stat-card animate-pulse">
        <div className="h-4 bg-zinc-800 rounded w-48 mb-3" />
        <div className="space-y-2">
          {Array.from({ length: 5 }).map((_, i) => (
            <div key={i} className="h-4 bg-zinc-800 rounded w-64" />
          ))}
        </div>
      </div>
    )
  }

  if (!data || data.length === 0) {
    return (
      <div className="stat-card">
        <h3 className="text-sm font-medium text-zinc-400 mb-3">This Month&apos;s Supporters</h3>
        <p className="text-zinc-500 text-sm">
          No contributors yet this month. Be the first!
        </p>
      </div>
    )
  }

  const champions = data.filter(s => s.tier === 'champion')
  const supporters = data.filter(s => s.tier === 'supporter')

  return (
    <div className="stat-card">
      <h3 className="text-sm font-medium text-zinc-400 mb-4">
        This Month&apos;s Supporters ({data.length})
      </h3>

      <div className="space-y-1.5">
        {champions.map((s) => (
          <div key={s.address} className="flex items-center gap-2 text-sm">
            <span className="text-amber-400">&#9733;</span>
            <span className="font-mono text-zinc-200">{s.address}</span>
            <span className="text-xs bg-amber-900/40 text-amber-400 border border-amber-800 rounded px-1.5 py-0.5">
              Champion
            </span>
          </div>
        ))}
        {supporters.map((s) => (
          <div key={s.address} className="flex items-center gap-2 text-sm">
            <span className="text-zinc-600">&bull;</span>
            <span className="font-mono text-zinc-400">{s.address}</span>
            <span className="text-xs bg-blue-900/40 text-blue-400 border border-blue-800 rounded px-1.5 py-0.5">
              Supporter
            </span>
          </div>
        ))}
      </div>

      <p className="text-zinc-600 text-xs mt-4">
        Contributors are identified by truncated wallet address. Miner addresses are already public on the P2Pool sidechain.
        Opt out by contacting the operator.
      </p>
    </div>
  )
}
