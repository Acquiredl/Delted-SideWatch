'use client'

import { formatXMR, formatEffort, formatRelativeTime, truncateAddress } from '@/lib/api'
import type { FoundBlock } from '@/lib/api'

interface BlocksTableProps {
  blocks: FoundBlock[]
  isLoading?: boolean
}

export default function BlocksTable({ blocks, isLoading }: BlocksTableProps) {
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

  if (blocks.length === 0) {
    return (
      <div className="stat-card text-center py-12">
        <p className="text-zinc-300 text-lg font-medium mb-2">No blocks found yet</p>
        <p className="text-zinc-500 text-sm max-w-md mx-auto">
          P2Pool mini finds a Monero main chain block roughly every few hours depending on
          the pool&apos;s total hashrate. When a block is found, it will appear here with
          the reward and effort statistics.
        </p>
      </div>
    )
  }

  return (
    <div className="table-container">
      <table className="data-table">
        <thead>
          <tr>
            <th>Height</th>
            <th>Hash</th>
            <th>Reward</th>
            <th>Effort</th>
            <th>CB Priv Key</th>
            <th>Found</th>
          </tr>
        </thead>
        <tbody>
          {blocks.map((block) => (
            <tr key={block.main_height}>
              <td className="font-mono text-xmr-orange">{block.main_height.toLocaleString()}</td>
              <td className="font-mono text-zinc-400">{block.main_hash ? truncateAddress(block.main_hash) : '—'}</td>
              <td className="font-mono text-green-400">
                {block.coinbase_reward > 0 ? `${formatXMR(block.coinbase_reward)} XMR` : '—'}
              </td>
              <td className={`font-mono ${block.effort != null && block.effort > 1 ? 'text-red-400' : 'text-green-400'}`}>
                {block.effort != null ? formatEffort(block.effort) : '—'}
              </td>
              <td className="font-mono text-zinc-500">
                {block.coinbase_private_key ? (
                  <button
                    onClick={() => navigator.clipboard.writeText(block.coinbase_private_key!)}
                    title="Click to copy full key — used for trustless payout verification"
                    className="hover:text-zinc-300 transition-colors cursor-pointer"
                  >
                    {truncateAddress(block.coinbase_private_key)}
                  </button>
                ) : '—'}
              </td>
              <td className="text-zinc-400">{formatRelativeTime(block.found_at)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
