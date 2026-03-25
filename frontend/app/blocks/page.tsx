'use client'

import { useState } from 'react'
import useSWR from 'swr'
import { fetcher } from '@/lib/api'
import type { FoundBlock } from '@/lib/api'
import BlocksTable from '@/components/BlocksTable'

const PAGE_SIZE = 50

export default function BlocksPage() {
  const [offset, setOffset] = useState(0)

  const { data: blocks, isLoading } = useSWR<FoundBlock[]>(
    `/api/blocks?limit=${PAGE_SIZE}&offset=${offset}`,
    fetcher,
    { refreshInterval: 30000 }
  )

  const hasPrev = offset > 0
  const hasNext = (blocks?.length ?? 0) === PAGE_SIZE

  return (
    <div>
      <div className="mb-8">
        <h1 className="text-3xl font-bold text-zinc-100 mb-2">Blocks Found</h1>
        <p className="text-zinc-400 text-sm">
          Monero main chain blocks found by P2Pool miners.
        </p>
      </div>

      <BlocksTable blocks={blocks || []} isLoading={isLoading} />

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
    </div>
  )
}
