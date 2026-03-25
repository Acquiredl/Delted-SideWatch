'use client'

import { formatRelativeTime } from '@/lib/api'

interface Worker {
  worker_name: string
  shares: number
  last_share_at: string
}

interface WorkersTableProps {
  workers: Worker[]
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
        No active workers
      </div>
    )
  }

  return (
    <div className="table-container">
      <table className="data-table">
        <thead>
          <tr>
            <th>Worker Name</th>
            <th>Shares</th>
            <th>Last Share</th>
          </tr>
        </thead>
        <tbody>
          {workers.map((worker) => (
            <tr key={worker.worker_name}>
              <td className="font-mono text-zinc-100">{worker.worker_name || 'default'}</td>
              <td className="font-mono text-zinc-300">{worker.shares.toLocaleString()}</td>
              <td className="text-zinc-400">{formatRelativeTime(worker.last_share_at)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
