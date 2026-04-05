'use client'

import { useState, useEffect, FormEvent } from 'react'
import Link from 'next/link'
import useSWR from 'swr'
import { fetcher, formatXMR, formatHashrate, formatRelativeTime, formatCAD, deleteJSON, authHeaders } from '@/lib/api'
import type { MinerStats, MinerPayment, HashratePoint, SubscriptionStatus, PoolStats, MinerWorker, PaymentYearSummary, LocalWorker, HeldDataStatus } from '@/lib/api'
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
  const [apiKey, setApiKey] = useState('')
  const [showApiKeyInput, setShowApiKeyInput] = useState(false)
  const [nukeStep, setNukeStep] = useState<0 | 1 | 2>(0) // 0=hidden, 1=first confirm, 2=final confirm
  const [nukeLoading, setNukeLoading] = useState(false)
  const [nukeResult, setNukeResult] = useState<string | null>(null)
  const [exportLoading, setExportLoading] = useState(false)

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

  const { data: heldData } = useSWR<HeldDataStatus>(
    activeAddress ? `/api/miner/${activeAddress}/held-data` : null,
    fetcher,
  )

  // Load API key from localStorage when address changes.
  useEffect(() => {
    if (activeAddress) {
      const stored = localStorage.getItem(`sidewatch-apikey-${activeAddress}`)
      if (stored) setApiKey(stored)
      else setApiKey('')
    }
    setNukeStep(0)
    setNukeResult(null)
  }, [activeAddress])

  // Persist API key to localStorage when it changes.
  useEffect(() => {
    if (activeAddress && apiKey) {
      localStorage.setItem(`sidewatch-apikey-${activeAddress}`, apiKey)
    }
  }, [activeAddress, apiKey])

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

  async function handleTaxExport() {
    if (!activeAddress) return
    const hasHeldExport = heldData?.has_held_data && (heldData.exports_remaining ?? 0) > 0
    if (!isPaid && !hasHeldExport) {
      setShowUpgradePrompt(true)
      return
    }
    if (!apiKey) {
      setShowApiKeyInput(true)
      return
    }

    setExportLoading(true)
    try {
      const yearParam = taxYear ? `?year=${taxYear}` : ''
      const res = await fetch(`${API_BASE}/api/miner/${activeAddress}/tax-export${yearParam}`, {
        headers: authHeaders(apiKey),
      })
      if (!res.ok) {
        const err = await res.json().catch(() => ({ error: `HTTP ${res.status}` }))
        alert(err.error || `Export failed: ${res.status}`)
        return
      }
      // Trigger download from response blob.
      const blob = await res.blob()
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = res.headers.get('Content-Disposition')?.match(/filename="?(.+?)"?$/)?.[1]
        || `xmr-payments-${activeAddress}.csv`
      document.body.appendChild(a)
      a.click()
      a.remove()
      URL.revokeObjectURL(url)
    } catch (err) {
      alert(`Export failed: ${err instanceof Error ? err.message : 'unknown error'}`)
    } finally {
      setExportLoading(false)
    }
  }

  async function handleNuke() {
    if (!activeAddress || !apiKey) return
    setNukeLoading(true)
    try {
      await deleteJSON(`/api/miner/${activeAddress}/data`, { confirm: 'DELETE ALL MY DATA' }, authHeaders(apiKey))
      setNukeResult('All your mining data has been permanently deleted.')
      setNukeStep(0)
      localStorage.removeItem(`sidewatch-apikey-${activeAddress}`)
      setApiKey('')
    } catch (err) {
      setNukeResult(`Deletion failed: ${err instanceof Error ? err.message : 'unknown error'}`)
    } finally {
      setNukeLoading(false)
    }
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
          This dashboard shows stats for miners connected to this P2Pool node.
          For global P2Pool mini stats, visit{' '}
          <a href="https://mini.p2pool.observer" target="_blank" rel="noopener noreferrer" className="text-zinc-400 hover:text-zinc-300 underline">
            mini.p2pool.observer
          </a>.
        </p>
      </form>

      {statsError && (
        <div className="text-red-400 text-sm p-4 bg-red-900/20 border border-red-800 rounded-lg mb-6">
          {statsError.message === 'API error: 404'
            ? 'Address not found on this node. SideWatch only tracks miners connected to this P2Pool node. For global stats, check mini.p2pool.observer.'
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
                No activity found for this address on this node. SideWatch only tracks miners
                connected to this P2Pool node — if you&apos;re mining through a different node,
                your stats won&apos;t appear here.
              </p>
              <p className="text-zinc-500 text-xs mt-2">
                If you just started mining on this node, data will appear within a few minutes.
                For global P2Pool mini stats, visit{' '}
                <a href="https://mini.p2pool.observer" target="_blank" rel="noopener noreferrer" className="text-zinc-400 hover:text-zinc-300 underline">
                  mini.p2pool.observer
                </a>.
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

          {heldData?.has_held_data && (heldData.exports_remaining ?? 0) > 0 && (
            <div className="bg-amber-900/20 border border-amber-800 rounded-lg px-4 py-3 mb-6">
              <p className="text-amber-400 text-sm font-medium mb-1">
                Your {heldData.held_year} tax data is ready to export
              </p>
              <p className="text-zinc-400 text-xs">
                Your subscription has lapsed, but your {heldData.held_year} payment history is preserved.
                You have {heldData.exports_remaining} download{heldData.exports_remaining === 1 ? '' : 's'} remaining.
                After that, this data will be deleted. Enter your API key below to export.
              </p>
            </div>
          )}

          {(showApiKeyInput || isPaid || (heldData?.has_held_data && (heldData.exports_remaining ?? 0) > 0)) && (
            <div className="bg-zinc-900/50 border border-zinc-800 rounded-lg px-4 py-3 mb-6">
              <div className="flex items-center gap-3">
                <label className="text-zinc-400 text-sm whitespace-nowrap">API Key</label>
                <input
                  type="password"
                  value={apiKey}
                  onChange={(e) => setApiKey(e.target.value)}
                  placeholder="Paste your API key"
                  className="input-field flex-1 text-sm py-1.5"
                />
                {apiKey && (
                  <span className="text-green-500 text-xs whitespace-nowrap">Saved locally</span>
                )}
              </div>
              <p className="text-zinc-600 text-xs mt-1.5">
                Required for tax exports and data management. Generate one on the{' '}
                <Link href={`/subscribe?address=${encodeURIComponent(activeAddress || '')}`} className="text-zinc-400 hover:text-zinc-300 underline">
                  subscription page
                </Link>.
                Stored in your browser only.
              </p>
            </div>
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
                    extended data retention, worker breakdown, and more.
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
              <button onClick={handleTaxExport} disabled={exportLoading} className="btn-secondary text-sm disabled:opacity-50">
                {exportLoading ? 'Exporting...' : 'Download Tax Export (CSV)'}
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

          {/* Data Management — nuke button */}
          {apiKey && (
            <div className="mt-12 pt-8 border-t border-zinc-800">
              <h2 className="text-lg font-bold text-zinc-100 mb-2">Data Management</h2>
              <p className="text-zinc-500 text-sm mb-4">
                Permanently delete all mining data associated with this address.
                This cannot be undone.
              </p>

              {nukeResult && (
                <div className={`rounded-lg px-4 py-3 mb-4 text-sm ${
                  nukeResult.startsWith('All') ? 'bg-green-900/20 border border-green-800 text-green-400'
                    : 'bg-red-900/20 border border-red-800 text-red-400'
                }`}>
                  {nukeResult}
                </div>
              )}

              {nukeStep === 0 && (
                <button
                  onClick={() => setNukeStep(1)}
                  className="text-red-500 hover:text-red-400 text-sm font-medium border border-red-900 hover:border-red-700 px-4 py-2 rounded-lg transition-colors"
                >
                  Delete All My Data
                </button>
              )}

              {nukeStep === 1 && (
                <div className="bg-red-950/30 border border-red-900 rounded-lg p-4">
                  <p className="text-red-400 text-sm font-medium mb-2">
                    Are you sure? This will permanently delete:
                  </p>
                  <ul className="text-zinc-400 text-xs list-disc list-inside mb-4 space-y-1">
                    <li>All shares, hashrate history, and payment records</li>
                    <li>Subscription payment history</li>
                    <li>Your API key</li>
                    <li>Any held-back tax export data</li>
                  </ul>
                  <div className="flex gap-3">
                    <button
                      onClick={() => setNukeStep(2)}
                      className="bg-red-900 hover:bg-red-800 text-red-200 text-sm font-medium px-4 py-2 rounded-lg transition-colors"
                    >
                      Yes, I understand
                    </button>
                    <button
                      onClick={() => setNukeStep(0)}
                      className="text-zinc-400 hover:text-zinc-300 text-sm px-4 py-2"
                    >
                      Cancel
                    </button>
                  </div>
                </div>
              )}

              {nukeStep === 2 && (
                <div className="bg-red-950/50 border-2 border-red-700 rounded-lg p-4">
                  <p className="text-red-300 text-sm font-bold mb-3">
                    FINAL WARNING — This action is irreversible.
                  </p>
                  <p className="text-zinc-400 text-xs mb-4">
                    All mining data for this address will be permanently deleted from SideWatch servers.
                    Your subscription will be reset to free tier.
                  </p>
                  <div className="flex gap-3">
                    <button
                      onClick={handleNuke}
                      disabled={nukeLoading}
                      className="bg-red-700 hover:bg-red-600 text-white text-sm font-bold px-6 py-2 rounded-lg transition-colors disabled:opacity-50"
                    >
                      {nukeLoading ? 'Deleting...' : 'Permanently Delete Everything'}
                    </button>
                    <button
                      onClick={() => setNukeStep(0)}
                      disabled={nukeLoading}
                      className="text-zinc-400 hover:text-zinc-300 text-sm px-4 py-2"
                    >
                      Cancel
                    </button>
                  </div>
                </div>
              )}
            </div>
          )}
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
