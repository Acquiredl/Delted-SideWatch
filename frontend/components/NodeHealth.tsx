'use client'

import useSWR from 'swr'
import { fetcher, formatHashrate, formatRelativeTime } from '@/lib/api'
import type { NodeStatusResponse } from '@/lib/api'

export default function NodeHealth() {
  const { data, isLoading } = useSWR<NodeStatusResponse>('/api/nodes/status', fetcher, {
    refreshInterval: 60000,
  })

  if (isLoading) {
    return (
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
        {Array.from({ length: 2 }).map((_, i) => (
          <div key={i} className="stat-card animate-pulse">
            <div className="h-4 bg-zinc-800 rounded w-32 mb-2" />
            <div className="h-6 bg-zinc-800 rounded w-20" />
          </div>
        ))}
      </div>
    )
  }

  if (!data || data.nodes.length === 0) return null

  return (
    <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
      {data.nodes.map((node) => {
        const statusColor =
          node.status === 'healthy'
            ? 'bg-green-500'
            : node.status === 'syncing'
              ? 'bg-yellow-500'
              : 'bg-red-500'

        return (
          <div key={node.name} className="stat-card">
            <div className="flex items-center gap-2 mb-2">
              <span className={`inline-block w-2 h-2 rounded-full ${statusColor}`} />
              <span className="text-sm font-medium text-zinc-100">{node.name}</span>
              <span className="text-xs text-zinc-500">{node.sidechain}</span>
            </div>
            <div className="grid grid-cols-3 gap-2 text-xs">
              {node.hashrate != null && (
                <div>
                  <p className="text-zinc-500">Hashrate</p>
                  <p className="text-zinc-300 font-mono">{formatHashrate(node.hashrate)}</p>
                </div>
              )}
              {node.miners != null && (
                <div>
                  <p className="text-zinc-500">Miners</p>
                  <p className="text-zinc-300 font-mono">{node.miners}</p>
                </div>
              )}
              {node.peers != null && (
                <div>
                  <p className="text-zinc-500">Peers</p>
                  <p className="text-zinc-300 font-mono">{node.peers}</p>
                </div>
              )}
            </div>
            {node.last_health_at && (
              <p className="text-zinc-600 text-xs mt-2">
                Last check: {formatRelativeTime(node.last_health_at)}
              </p>
            )}
          </div>
        )
      })}
    </div>
  )
}
