'use client'

import { truncateAddress, formatDifficulty, formatRelativeTime } from '@/lib/api'
import type { SidechainShare } from '@/lib/api'

interface SidechainTableProps {
  shares: SidechainShare[]
  isLoading?: boolean
}

export default function SidechainTable({ shares, isLoading }: SidechainTableProps) {
  if (isLoading) {
    return (
      <div className="stat-card">
        <div className="animate-pulse space-y-3">
          {Array.from({ length: 5 }).map((_, i) => (
            <div key={i} className="h-6 bg-zinc-800 rounded" />
          ))}
        </div>
      </div>
    )
  }

  if (shares.length === 0) {
    return (
      <div className="stat-card text-center text-zinc-500 py-8">
        No sidechain shares found
      </div>
    )
  }

  return (
    <div className="table-container">
      <table className="data-table">
        <thead>
          <tr>
            <th>Miner</th>
            <th>Worker</th>
            <th>Height</th>
            <th>Difficulty</th>
            <th>Time</th>
          </tr>
        </thead>
        <tbody>
          {shares.map((share, i) => (
            <tr key={`${share.sidechain_height}-${i}`}>
              <td className="font-mono text-xmr-orange">{truncateAddress(share.miner_address)}</td>
              <td className="font-mono text-zinc-300">{share.worker_name || '--'}</td>
              <td className="font-mono text-zinc-100">{share.sidechain_height.toLocaleString()}</td>
              <td className="font-mono text-zinc-300">{formatDifficulty(share.difficulty)}</td>
              <td className="text-zinc-400">{formatRelativeTime(share.created_at)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
