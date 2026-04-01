'use client'

import useSWR from 'swr'
import LiveStats from '@/components/LiveStats'
import WindowVsWeeklyToggle from '@/components/WindowVsWeeklyToggle'
import FundProgress from '@/components/FundProgress'
import { fetcher } from '@/lib/api'
import type { PoolStats } from '@/lib/api'

const sidechain = (process.env.NEXT_PUBLIC_SIDECHAIN || 'mini').charAt(0).toUpperCase() + (process.env.NEXT_PUBLIC_SIDECHAIN || 'mini').slice(1)

export default function HomePage() {
  const { data: poolStats } = useSWR<PoolStats>('/api/pool/stats', fetcher, { refreshInterval: 15000 })

  return (
    <div>
      <div className="mb-8">
        <h1 className="text-3xl font-bold text-zinc-100 mb-2">SideWatch — P2Pool {sidechain}</h1>
        <p className="text-zinc-400 text-sm">
          Decentralized Monero mining — no fees, no registration, no custody.
          Your keys, your coins.
        </p>
      </div>
      <LiveStats />
      {poolStats && (
        <WindowVsWeeklyToggle windowMiners={poolStats.total_miners} />
      )}
      <div className="mt-6">
        <FundProgress compact />
      </div>
    </div>
  )
}
