'use client'

import { useState } from 'react'
import useSWR from 'swr'
import { fetcher, formatHashrate } from '@/lib/api'
import type { FoundBlock, PoolStats } from '@/lib/api'
import BlocksTable from '@/components/BlocksTable'

const PAGE_SIZE = 50

export default function BlocksPage() {
  const [offset, setOffset] = useState(0)

  const { data: blocks, isLoading } = useSWR<FoundBlock[]>(
    `/api/blocks?limit=${PAGE_SIZE}&offset=${offset}`,
    fetcher,
    { refreshInterval: 30000 }
  )

  const { data: poolStats } = useSWR<PoolStats>(
    '/api/pool/stats',
    fetcher,
    { refreshInterval: 15000 }
  )

  const hasPrev = offset > 0
  const hasNext = (blocks?.length ?? 0) === PAGE_SIZE

  return (
    <div>
      <div className="mb-8">
        <h1 className="text-3xl font-bold text-zinc-100 mb-2">Blocks Found</h1>
        <p className="text-zinc-400 text-sm mb-3">
          Monero main chain blocks found by P2Pool miners. Each block includes the
          reward, mining effort, and coinbase private key for trustless verification.
          {poolStats && poolStats.total_hashrate > 0 && (
            <> Currently hashing at {formatHashrate(poolStats.total_hashrate)} with {poolStats.total_miners} miners.</>
          )}
        </p>
        <div className="cube-divider">
          <span style={{ backgroundColor: 'var(--cube-orange)', animationDelay: '0s' }} />
          <span style={{ backgroundColor: 'var(--cube-yellow)', animationDelay: '0.5s' }} />
          <span style={{ backgroundColor: 'var(--cube-green)', animationDelay: '1s' }} />
        </div>
      </div>

      <BlocksTable blocks={blocks || []} isLoading={isLoading} />

      {blocks && blocks.length > 0 && (
        <div className="flex items-center justify-between mt-6">
          <button
            onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}
            disabled={!hasPrev}
            className={`btn-secondary text-sm ${!hasPrev ? 'opacity-50 cursor-not-allowed' : ''}`}
          >
            Previous
          </button>
          <span className="text-zinc-500 text-sm">
            Showing {offset + 1} - {offset + (blocks?.length ?? 0)}
          </span>
          <button
            onClick={() => setOffset(offset + PAGE_SIZE)}
            disabled={!hasNext}
            className={`btn-secondary text-sm ${!hasNext ? 'opacity-50 cursor-not-allowed' : ''}`}
          >
            Next
          </button>
        </div>
      )}
    </div>
  )
}
