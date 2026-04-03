'use client'

import { truncateAddress, formatHashrate, formatRelativeTime } from '@/lib/api'
import type { LocalWorker } from '@/lib/api'

interface WorkersTableProps {
  workers: LocalWorker[]
  isLoading?: boolean
}

export default function WorkersTable({ workers, isLoading }: WorkersTableProps) {
  if (isLoading) {
    return (
      <div className="stat-card">
        <div className="animate-pulse space-y-3">
          {Array.from({ length: 3 }).map((_, i) => (
            <div key={i} className="h-6 bg-zinc-800 rounded" />
          ))}
        </div>
      </div>
    )
  }

  if (workers.length === 0) {
    return (
      <div className="stat-card text-center text-zinc-500 py-8">
        No active workers connected to this node
      </div>
    )
  }

  return (
    <div className="table-container">
      <table className="data-table">
        <thead>
          <tr>
            <th>Miner Address</th>
            <th>Hashrate</th>
            <th>Last Seen</th>
          </tr>
        </thead>
        <tbody>
          {workers.map((worker) => (
            <tr key={worker.miner_address}>
              <td className="font-mono text-xmr-orange">{truncateAddress(worker.miner_address)}</td>
              <td className="font-mono text-zinc-100">{formatHashrate(worker.current_hashrate)}</td>
              <td className="text-zinc-400">{formatRelativeTime(worker.last_seen)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
