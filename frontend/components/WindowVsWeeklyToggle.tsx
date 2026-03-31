'use client'

import { useState } from 'react'
import useSWR from 'swr'
import { fetcher, truncateAddress, formatRelativeTime } from '@/lib/api'
import type { WeeklyMiner } from '@/lib/api'

type View = 'window' | 'weekly'

interface MinerRow {
  address: string
  detail: string
}

interface WindowVsWeeklyToggleProps {
  /** Current-window miner count from pool stats */
  windowMiners: number
}

export default function WindowVsWeeklyToggle({ windowMiners }: WindowVsWeeklyToggleProps) {
  const [view, setView] = useState<View>('window')

  const { data: weeklyMiners, isLoading } = useSWR<WeeklyMiner[]>(
    view === 'weekly' ? '/api/miners/weekly' : null,
    fetcher,
  )

  return (
    <div className="mt-8">
      <div className="flex items-center gap-4 mb-4">
        <h2 className="text-xl font-bold text-zinc-100">Miners</h2>
        <div className="flex bg-zinc-800 rounded-lg p-0.5">
          <button
            onClick={() => setView('window')}
            className={`px-3 py-1 text-sm rounded-md transition-colors ${
              view === 'window'
                ? 'bg-zinc-700 text-zinc-100'
                : 'text-zinc-400 hover:text-zinc-300'
            }`}
          >
            Current Window
          </button>
          <button
            onClick={() => setView('weekly')}
            className={`px-3 py-1 text-sm rounded-md transition-colors ${
              view === 'weekly'
                ? 'bg-zinc-700 text-zinc-100'
                : 'text-zinc-400 hover:text-zinc-300'
            }`}
          >
            Weekly Active
          </button>
        </div>
      </div>

      {view === 'window' && (
        <div className="stat-card">
          <p className="text-zinc-400 text-sm">
            {windowMiners} miner{windowMiners !== 1 ? 's' : ''} currently in the PPLNS window.
          </p>
          <p className="text-zinc-500 text-xs mt-2">
            The PPLNS window shows miners with active shares right now.
            Miners who go offline will drop out of this view but may still
            appear in the weekly active list.
          </p>
        </div>
      )}

      {view === 'weekly' && isLoading && (
        <div className="stat-card animate-pulse">
          <div className="h-6 bg-zinc-800 rounded w-48 mb-3" />
          <div className="space-y-2">
            {Array.from({ length: 3 }).map((_, i) => (
              <div key={i} className="h-5 bg-zinc-800 rounded" />
            ))}
          </div>
        </div>
      )}

      {view === 'weekly' && weeklyMiners && (
        <div className="table-container">
          <table className="data-table">
            <thead>
              <tr>
                <th>Address</th>
                <th>Shares (7d)</th>
                <th>Last Share</th>
              </tr>
            </thead>
            <tbody>
              {weeklyMiners.map((m) => (
                <tr key={m.address}>
                  <td className="font-mono text-xmr-orange">{truncateAddress(m.address)}</td>
                  <td className="font-mono text-zinc-100">{m.share_count.toLocaleString()}</td>
                  <td className="text-zinc-400">{formatRelativeTime(m.last_share_at)}</td>
                </tr>
              ))}
              {weeklyMiners.length === 0 && (
                <tr>
                  <td colSpan={3} className="text-center text-zinc-500 py-4">
                    No miners active in the last 7 days
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
