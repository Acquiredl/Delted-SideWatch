'use client'

import { useState, FormEvent } from 'react'
import Link from 'next/link'
import useSWR from 'swr'
import { fetcher, formatXMR, formatHashrate, formatRelativeTime, formatCAD } from '@/lib/api'
import type { MinerStats, MinerPayment, HashratePoint, SubscriptionStatus, PoolStats, MinerWorker, PaymentYearSummary, LocalWorker } from '@/lib/api'
import PrivacyNotice from '@/components/PrivacyNotice'
import HashrateChart from '@/components/HashrateChart'
import PaymentsTable from '@/components/PaymentsTable'
import WorkersTable from '@/components/WorkersTable'
import LocalWorkersTable from '@/components/LocalWorkersTable'
import ShareTimeCalculator from '@/components/ShareTimeCalculator'
import UncleRateWarning from '@/components/UncleRateWarning'

const API_BASE = process.env.NEXT_PUBLIC_API_URL || ''

export default function MinerPage() {
  const [address, setAddress] = useState('')
  const [activeAddress, setActiveAddress] = useState<string | null>(null)
  const [taxYear, setTaxYear] = useState<number | null>(null)
  const [showUpgradePrompt, setShowUpgradePrompt] = useState(false)

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

  const isPaid = subStatus?.active && (subStatus?.tier === 'supporter' || subStatus?.tier === 'champion')
  const { data: minerWorkers, isLoading: minerWorkersLoading } = useSWR<MinerWorker[]>(
    activeAddress && isPaid ? `/api/miner/${activeAddress}/workers` : null,
    fetcher,
    { refreshInterval: 30000 }
  )

  const { data: paymentSummary } = useSWR<PaymentYearSummary[]>(
    activeAddress ? `/api/miner/${activeAddress}/payment-summary` : null,
    fetcher,
    { refreshInterval: 60000 }
  )

  // Show local workers when no address is entered
  const { data: localWorkers, isLoading: localWorkersLoading } = useSWR<LocalWorker[]>(
    !activeAddress ? '/api/workers' : null,
    fetcher,
    { refreshInterval: 15000 }
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
    if (!isPaid) {
      setShowUpgradePrompt(true)
      return
    }
    const yearParam = taxYear ? `?year=${taxYear}` : ''
    window.open(`${API_BASE}/api/miner/${activeAddress}/tax-export${yearParam}`, '_blank')
  }

  // Check if miner has any real activity
  const hasHashrate = minerStats && (minerStats.current_hashrate > 0 || minerStats.average_hashrate > 0)
  const hasPayments = payments && payments.length > 0

  return (
    <div>
      <div className="mb-8">
        <h1 className="text-3xl font-bold text-zinc-100 mb-2">Miner Dashboard</h1>
        <p className="text-zinc-400 text-sm mb-3">
          Look up your mining statistics by wallet address.
        </p>
        <div className="cube-divider">
          <span style={{ backgroundColor: 'var(--cube-orange)', animationDelay: '0s' }} />
          <span style={{ backgroundColor: 'var(--cube-blue)', animationDelay: '0.4s' }} />
          <span style={{ backgroundColor: 'var(--cube-green)', animationDelay: '0.8s' }} />
          <span style={{ backgroundColor: 'var(--cube-red)', animationDelay: '1.2s' }} />
        </div>
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
        <p className="text-zinc-500 text-xs mt-2">
          P2Pool truncates wallet addresses for privacy. Your full address will be matched against the stored prefix.
        </p>
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
            <div className="stat-card stat-card-orange">
              <p className="text-zinc-400 text-sm mb-1">Current Hashrate</p>
              <p className="text-2xl font-bold text-zinc-100">
                {formatHashrate(minerStats.current_hashrate)}
              </p>
            </div>
            <div className="stat-card stat-card-blue">
              <p className="text-zinc-400 text-sm mb-1">24h Average</p>
              <p className="text-2xl font-bold text-zinc-100">
                {formatHashrate(minerStats.average_hashrate)}
              </p>
            </div>
            <div className="stat-card stat-card-yellow">
              <p className="text-zinc-400 text-sm mb-1">Total Shares</p>
              <p className="text-2xl font-bold text-zinc-100">
                {minerStats.total_shares.toLocaleString()}
              </p>
              <p className="text-zinc-500 text-xs mt-1">
                {minerStats.last_share_at ? `Last: ${formatRelativeTime(minerStats.last_share_at)}` : ''}
              </p>
            </div>
            <div className="stat-card stat-card-green">
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

          {!hasHashrate && !hasPayments && (
            <div className="bg-zinc-900/50 border border-zinc-800 rounded-lg px-4 py-3 mb-6">
              <p className="text-zinc-400 text-sm">
                No activity found for this address. If you just started mining, data will appear
                within a few minutes. Make sure your XMRig is connected to this node&apos;s stratum port.
              </p>
            </div>
          )}

          {subStatus && subStatus.tier === 'free' && (
            <div className="flex items-center justify-between bg-zinc-900/50 border border-zinc-800 rounded-lg px-4 py-3 mb-6">
              <span className="text-zinc-400 text-sm">
                Free tier — hashrate history limited to 30 days
              </span>
              <Link href={`/subscribe?address=${encodeURIComponent(activeAddress || '')}`} className="text-xmr-orange hover:text-xmr-orange-dark text-sm font-medium transition-colors">
                Upgrade
              </Link>
            </div>
          )}

          {subStatus && (subStatus.tier === 'supporter' || subStatus.tier === 'champion') && subStatus.active && (
            <div className="flex items-center gap-2 mb-6">
              <span className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium ${
                subStatus.tier === 'champion'
                  ? 'bg-amber-900/40 text-amber-400 border border-amber-800'
                  : 'bg-green-900/40 text-green-400 border border-green-800'
              }`}>
                {subStatus.tier === 'champion' ? 'Champion' : 'Supporter'}
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
              <WorkersTable workers={minerWorkers || []} isLoading={minerWorkersLoading} />
            </div>
          )}

          {showUpgradePrompt && (
            <div className="bg-zinc-900/80 border border-xmr-orange/30 rounded-lg p-5 mb-6">
              <div className="flex items-start justify-between gap-4">
                <div>
                  <h3 className="text-zinc-100 font-semibold mb-1">Tax export requires Supporter tier</h3>
                  <p className="text-zinc-400 text-sm mb-3">
                    Support SideWatch for as little as ~$1/mo in XMR to unlock tax CSV exports,
                    15-month data retention, worker breakdown, and more.
                  </p>
                  <Link
                    href={`/subscribe?address=${encodeURIComponent(activeAddress || '')}&from=tax-export`}
                    className="inline-flex items-center gap-2 bg-xmr-orange hover:bg-xmr-orange-dark text-zinc-950 font-medium text-sm px-4 py-2 rounded-lg transition-colors"
                  >
                    Subscribe with XMR
                  </Link>
                </div>
                <button
                  onClick={() => setShowUpgradePrompt(false)}
                  className="text-zinc-500 hover:text-zinc-300 text-lg leading-none mt-0.5"
                  aria-label="Dismiss"
                >
                  &times;
                </button>
              </div>
            </div>
          )}

          <div className="flex items-center justify-between mb-4">
            <h2 className="text-xl font-bold text-zinc-100">Payment History</h2>
            <div className="flex items-center gap-3">
              <select
                value={taxYear ?? ''}
                onChange={(e) => setTaxYear(e.target.value ? Number(e.target.value) : null)}
                className="input-field text-sm py-1.5 px-3 w-auto"
              >
                <option value="">All years</option>
                {paymentSummary?.map((s) => (
                  <option key={s.year} value={s.year}>{s.year}</option>
                ))}
              </select>
              <button onClick={handleTaxExport} className="btn-secondary text-sm">
                Download Tax Export (CSV)
              </button>
            </div>
          </div>

          {paymentSummary && paymentSummary.length > 0 && (
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3 mb-4">
              {(taxYear
                ? paymentSummary.filter((s) => s.year === taxYear)
                : paymentSummary
              ).map((s) => (
                <div key={s.year} className="stat-card py-3 px-4">
                  <p className="text-zinc-400 text-xs mb-1">{s.year} Totals</p>
                  <p className="text-lg font-bold text-green-400">
                    {formatXMR(s.total_atomic)} XMR
                  </p>
                  <p className="text-sm text-zinc-300">
                    {s.total_cad != null ? formatCAD(s.total_cad) : '--'}
                    <span className="text-zinc-500 ml-2">({s.payment_count} payments)</span>
                  </p>
                </div>
              ))}
            </div>
          )}

          <PaymentsTable payments={payments || []} isLoading={paymentsLoading} />
        </>
      )}

      {!activeAddress && (
        <div>
          <h2 className="text-xl font-bold text-zinc-100 mb-4">Active Workers on This Node</h2>
          <LocalWorkersTable workers={localWorkers || []} isLoading={localWorkersLoading} />
        </div>
      )}
    </div>
  )
}
