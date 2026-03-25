'use client'

import { useState } from 'react'
import useSWR from 'swr'
import { fetcher } from '@/lib/api'
import type { SidechainShare } from '@/lib/api'
import SidechainTable from '@/components/SidechainTable'

const PAGE_SIZE = 100

export default function SidechainPage() {
  const [offset, setOffset] = useState(0)

  const { data: shares, isLoading } = useSWR<SidechainShare[]>(
    `/api/sidechain/shares?limit=${PAGE_SIZE}&offset=${offset}`,
    fetcher,
    { refreshInterval: 15000 }
  )

  const hasPrev = offset > 0
  const hasNext = (shares?.length ?? 0) === PAGE_SIZE

  return (
    <div>
      <div className="mb-8">
        <h1 className="text-3xl font-bold text-zinc-100 mb-2">Sidechain Shares</h1>
        <p className="text-zinc-400 text-sm">
          Recent shares submitted to the P2Pool sidechain. Shares are the proof that miners
          are contributing hashrate to the decentralized pool.
        </p>
      </div>

      <SidechainTable shares={shares || []} isLoading={isLoading} />

      <div className="flex items-center justify-between mt-6">
        <button
          onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}
          disabled={!hasPrev}
          className={`btn-secondary text-sm ${!hasPrev ? 'opacity-50 cursor-not-allowed' : ''}`}
        >
          Previous
        </button>
        <span className="text-zinc-500 text-sm">
          Showing {offset + 1} - {offset + (shares?.length ?? 0)}
        </span>
        <button
          onClick={() => setOffset(offset + PAGE_SIZE)}
          disabled={!hasNext}
          className={`btn-secondary text-sm ${!hasNext ? 'opacity-50 cursor-not-allowed' : ''}`}
        >
          Next
        </button>
      </div>
    </div>
  )
}
