'use client'

import { useState, FormEvent } from 'react'
import Link from 'next/link'
import useSWR from 'swr'
import { fetcher, formatXMR, formatHashrate, formatRelativeTime } from '@/lib/api'
import type { MinerStats, MinerPayment, HashratePoint, SubscriptionStatus, PoolStats, MinerWorker } from '@/lib/api'
import PrivacyNotice from '@/components/PrivacyNotice'
import HashrateChart from '@/components/HashrateChart'
import PaymentsTable from '@/components/PaymentsTable'
import WorkersTable from '@/components/WorkersTable'
import ShareTimeCalculator from '@/components/ShareTimeCalculator'
import UncleRateWarning from '@/components/UncleRateWarning'

const API_BASE = process.env.NEXT_PUBLIC_API_URL || ''

export default function MinerPage() {
  const [address, setAddress] = useState('')
  const [activeAddress, setActiveAddress] = useState<string | null>(null)

  const { data: minerStats, error: statsError, isLoading: statsLoading } = useSWR<MinerStats>(
    activeAddress ? `/api/miner/${activeAddress}` : null,
    fetcher,
    { refreshInterval: 15000 }
  )

  const { data: payments, isLoading: paymentsLoading } = useSWR<MinerPayment[]>(
    activeAddress ? `/api/miner/${activeAddress}/payments?limit=50&offset=0` : null,
    fetcher,
    { refreshInterval: 30000 }
  )

  const { data: hashrate, isLoading: hashrateLoading } = useSWR<HashratePoint[]>(
    activeAddress ? `/api/miner/${activeAddress}/hashrate?hours=24` : null,
    fetcher,
    { refreshInterval: 30000 }
  )

  const { data: subStatus } = useSWR<SubscriptionStatus>(
    activeAddress ? `/api/subscription/status/${activeAddress}` : null,
    fetcher,
  )

  const { data: poolStats } = useSWR<PoolStats>(
    activeAddress ? '/api/pool/stats' : null,
    fetcher,
    { refreshInterval: 30000 }
  )

  const isPaid = subStatus?.tier === 'paid' && subStatus?.active
  const { data: workers, isLoading: workersLoading } = useSWR<MinerWorker[]>(
    activeAddress && isPaid ? `/api/miner/${activeAddress}/workers` : null,
    fetcher,
    { refreshInterval: 30000 }
  )

  function handleSubmit(e: FormEvent) {
    e.preventDefault()
    const trimmed = address.trim()
    if (trimmed.length > 0) {
      setActiveAddress(trimmed)
    }
  }

  function handleTaxExport() {
    if (!activeAddress) return
    window.open(`${API_BASE}/api/miner/${activeAddress}/tax-export`, '_blank')
  }

  return (
    <div>
      <div className="mb-8">
        <h1 className="text-3xl font-bold text-zinc-100 mb-2">Miner Dashboard</h1>
        <p className="text-zinc-400 text-sm mb-6">
          Look up your mining statistics by wallet address.
        </p>
      </div>

      <PrivacyNotice />

      <form onSubmit={handleSubmit} className="mb-8">
        <div className="flex gap-3">
          <input
            type="text"
            value={address}
            onChange={(e) => setAddress(e.target.value)}
            placeholder="Enter your Monero wallet address (4...)"
            className="input-field flex-1"
          />
          <button type="submit" className="btn-primary whitespace-nowrap">
            Look Up
          </button>
        </div>
      </form>

      {statsError && (
        <div className="text-red-400 text-sm p-4 bg-red-900/20 border border-red-800 rounded-lg mb-6">
          {statsError.message === 'API error: 404'
            ? 'Miner address not found. Make sure you entered the correct address.'
            : `Failed to load miner stats: ${statsError.message}`}
        </div>
      )}

      {statsLoading && activeAddress && (
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4 mb-8">
          {Array.from({ length: 4 }).map((_, i) => (
            <div key={i} className="stat-card animate-pulse">
              <div className="h-4 bg-zinc-800 rounded w-24 mb-2" />
              <div className="h-8 bg-zinc-800 rounded w-32" />
            </div>
          ))}
        </div>
      )}

      {minerStats && (
        <>
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-5 gap-4 mb-8">
            <div className="stat-card">
              <p className="text-zinc-400 text-sm mb-1">Current Hashrate</p>
              <p className="text-2xl font-bold text-zinc-100">
                {formatHashrate(minerStats.current_hashrate)}
              </p>
            </div>
            <div className="stat-card">
              <p className="text-zinc-400 text-sm mb-1">24h Average</p>
              <p className="text-2xl font-bold text-zinc-100">
                {formatHashrate(minerStats.average_hashrate)}
              </p>
            </div>
            <div className="stat-card">
              <p className="text-zinc-400 text-sm mb-1">Total Shares</p>
              <p className="text-2xl font-bold text-zinc-100">
                {minerStats.total_shares.toLocaleString()}
              </p>
              <p className="text-zinc-500 text-xs mt-1">
                {minerStats.last_share_at ? `Last: ${formatRelativeTime(minerStats.last_share_at)}` : ''}
              </p>
            </div>
            <div className="stat-card">
              <p className="text-zinc-400 text-sm mb-1">Total Paid</p>
              <p className="text-2xl font-bold text-green-400">
                {formatXMR(minerStats.total_paid)} XMR
              </p>
              <p className="text-zinc-500 text-xs mt-1">
                {minerStats.last_payment_at ? `Last: ${formatRelativeTime(minerStats.last_payment_at)}` : ''}
              </p>
            </div>
            {poolStats && poolStats.sidechain_difficulty > 0 && (
              <ShareTimeCalculator
                minerHashrate={minerStats.current_hashrate}
                sidechainDifficulty={poolStats.sidechain_difficulty}
              />
            )}
          </div>

          {subStatus && subStatus.tier === 'free' && (
            <div className="flex items-center justify-between bg-zinc-900/50 border border-zinc-800 rounded-lg px-4 py-3 mb-6">
              <span className="text-zinc-400 text-sm">
                Free tier — hashrate history limited to 30 days
              </span>
              <Link href="/subscribe" className="text-xmr-orange hover:text-xmr-orange-dark text-sm font-medium transition-colors">
                Upgrade
              </Link>
            </div>
          )}

          {subStatus && subStatus.tier === 'paid' && subStatus.active && (
            <div className="flex items-center gap-2 mb-6">
              <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-green-900/40 text-green-400 border border-green-800">
                Paid
              </span>
              <span className="text-zinc-500 text-xs">Full history unlocked</span>
            </div>
          )}

          {minerStats.uncle_rate_24h != null && minerStats.uncle_rate_24h > 0.10 && (
            <UncleRateWarning uncleRate={minerStats.uncle_rate_24h} />
          )}

          <div className="mb-8">
            {hashrateLoading ? (
              <div className="stat-card animate-pulse h-[340px]" />
            ) : (
              <HashrateChart data={hashrate || []} />
            )}
          </div>

          {isPaid && (
            <div className="mb-8">
              <h2 className="text-xl font-bold text-zinc-100 mb-4">Workers</h2>
              <WorkersTable workers={workers || []} isLoading={workersLoading} />
            </div>
          )}

          <div className="flex items-center justify-between mb-4">
            <h2 className="text-xl font-bold text-zinc-100">Payment History</h2>
            <button onClick={handleTaxExport} className="btn-secondary text-sm">
              Download Tax Export (CSV)
            </button>
          </div>

          <PaymentsTable payments={payments || []} isLoading={paymentsLoading} />
        </>
      )}

      {!activeAddress && (
        <div className="text-center text-zinc-500 py-16">
          Enter your wallet address above to view your mining statistics.
        </div>
      )}
    </div>
  )
}
