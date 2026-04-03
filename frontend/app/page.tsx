'use client'

import useSWR from 'swr'
import LiveStats from '@/components/LiveStats'
import WindowVsWeeklyToggle from '@/components/WindowVsWeeklyToggle'
import FundProgress from '@/components/FundProgress'
import { fetcher } from '@/lib/api'
import type { PoolStats } from '@/lib/api'

const sidechain = (process.env.NEXT_PUBLIC_SIDECHAIN || 'mini').toLowerCase()

export default function HomePage() {
  const { data: poolStats } = useSWR<PoolStats>('/api/pool/stats', fetcher, { refreshInterval: 15000 })

  return (
    <div>
      <div className="mb-8">
        <h1 className="text-3xl font-bold text-zinc-100 mb-2">
          Welcome to <span className="text-xmr-orange">SideWatch</span>
        </h1>
        <p className="text-zinc-400 text-sm mb-3">
          Your observability dashboard for P2Pool {sidechain} mining.
          Decentralized, zero-fee, no custody.
        </p>
        <div className="cube-divider">
          <span style={{ backgroundColor: 'var(--cube-orange)', animationDelay: '0s' }} />
          <span style={{ backgroundColor: 'var(--cube-blue)', animationDelay: '0.5s' }} />
          <span style={{ backgroundColor: 'var(--cube-green)', animationDelay: '1s' }} />
          <span style={{ backgroundColor: 'var(--cube-red)', animationDelay: '1.5s' }} />
          <span style={{ backgroundColor: 'var(--cube-yellow)', animationDelay: '2s' }} />
          <span style={{ backgroundColor: 'var(--cube-white)', animationDelay: '2.5s' }} />
        </div>
      </div>

      <LiveStats />

      {poolStats && (
        <WindowVsWeeklyToggle windowMiners={poolStats.total_miners} />
      )}

      <div className="mt-6">
        <FundProgress compact />
      </div>

      <div className="mt-8 grid grid-cols-1 sm:grid-cols-3 gap-4">
        <a href="/connect" className="stat-card stat-card-blue group hover:border-cube-blue/50 transition-colors">
          <p className="text-cube-blue font-semibold mb-1">Start Mining</p>
          <p className="text-zinc-500 text-xs">Point your XMRig at our shared node. No registration needed.</p>
        </a>
        <a href="/miner" className="stat-card stat-card-green group hover:border-cube-green/50 transition-colors">
          <p className="text-cube-green font-semibold mb-1">View Your Stats</p>
          <p className="text-zinc-500 text-xs">Enter your wallet address to see hashrate, shares, and payments.</p>
        </a>
        <a href="/subscribe" className="stat-card stat-card-yellow group hover:border-cube-yellow/50 transition-colors">
          <p className="text-cube-yellow font-semibold mb-1">Support SideWatch</p>
          <p className="text-zinc-500 text-xs">Unlock extended history, tax exports, and fund the infrastructure.</p>
        </a>
      </div>
    </div>
  )
}
