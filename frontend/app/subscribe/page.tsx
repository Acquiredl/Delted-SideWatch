'use client'

import { Suspense, useState, useEffect, useRef, FormEvent } from 'react'
import { useSearchParams } from 'next/navigation'
import useSWR from 'swr'
import { fetcher, postJSON, tierIncludes } from '@/lib/api'
import type { SubscriptionStatus as SubStatusType, PaymentAddress, SubPayment } from '@/lib/api'
import SubscriptionStatus from '@/components/SubscriptionStatus'
import SubscriptionPayment from '@/components/SubscriptionPayment'
import TierSelector from '@/components/TierSelector'

export default function SubscribePage() {
  return (
    <Suspense fallback={null}>
      <SubscribePageContent />
    </Suspense>
  )
}

function SubscribePageContent() {
  const searchParams = useSearchParams()
  const [address, setAddress] = useState('')
  const [activeAddress, setActiveAddress] = useState<string | null>(null)
  const [apiKey, setApiKey] = useState<string | null>(null)
  const [apiKeyError, setApiKeyError] = useState<string | null>(null)
  const [existingKeyInput, setExistingKeyInput] = useState('')
  const fromContext = searchParams.get('from')
  const paymentRef = useRef<HTMLDivElement>(null)
  const didAutoFill = useRef(false)

  // Auto-populate address from query param (e.g. redirected from miner page)
  useEffect(() => {
    if (didAutoFill.current) return
    const addrParam = searchParams.get('address')
    if (addrParam && addrParam.trim().length > 0) {
      didAutoFill.current = true
      setAddress(addrParam.trim())
      setActiveAddress(addrParam.trim())
    }
  }, [searchParams])

  // Scroll to payment section when address is auto-filled
  useEffect(() => {
    if (activeAddress && didAutoFill.current && paymentRef.current) {
      const timer = setTimeout(() => {
        paymentRef.current?.scrollIntoView({ behavior: 'smooth', block: 'start' })
      }, 300)
      return () => clearTimeout(timer)
    }
  }, [activeAddress])

  const { data: subStatus, error: statusError, isLoading: statusLoading } = useSWR<SubStatusType>(
    activeAddress ? `/api/subscription/status/${activeAddress}` : null,
    fetcher,
    { refreshInterval: 30000 }
  )

  const { data: paymentAddress, isLoading: addressLoading } = useSWR<PaymentAddress>(
    activeAddress ? `/api/subscription/address/${activeAddress}` : null,
    fetcher,
  )

  const { data: subPayments, isLoading: paymentsLoading } = useSWR<SubPayment[]>(
    activeAddress ? `/api/subscription/payments/${activeAddress}` : null,
    fetcher,
    { refreshInterval: 30000 }
  )

  function handleSubmit(e: FormEvent) {
    e.preventDefault()
    const trimmed = address.trim()
    if (trimmed.length > 0) {
      setActiveAddress(trimmed)
      setApiKey(null)
      setApiKeyError(null)
    }
  }

  async function handleGenerateAPIKey() {
    if (!activeAddress) return
    setApiKeyError(null)
    try {
      const hasKey = subStatus?.has_api_key
      if (hasKey) {
        if (!existingKeyInput.trim()) {
          setApiKeyError('Enter your existing API key to regenerate.')
          return
        }
        const result = await postJSON<{ api_key: string; note: string }>(
          `/api/subscription/api-key/${activeAddress}`,
          undefined,
          { 'X-API-Key': existingKeyInput.trim() },
        )
        setApiKey(result.api_key)
      } else {
        const confirmedTx = subPayments?.find((p) => p.confirmed)
        if (!confirmedTx) {
          setApiKeyError('No confirmed payment found. Wait for your payment to confirm, then try again.')
          return
        }
        const result = await postJSON<{ api_key: string; note: string }>(
          `/api/subscription/api-key/${activeAddress}`,
          { tx_hash: confirmedTx.tx_hash },
        )
        setApiKey(result.api_key)
      }
    } catch {
      setApiKeyError('Failed to generate API key. Check your credentials and try again.')
    }
  }

  const currentTier = subStatus?.tier ?? 'free'

  return (
    <div>
      <div className="mb-8">
        <h1 className="text-3xl font-bold text-zinc-100 mb-2">Support <span className="text-xmr-orange">SideWatch</span></h1>
        <p className="text-zinc-400 text-sm mb-3">
          Fund the shared node infrastructure and unlock dashboard features.
          Pay what you want above the tier minimum &mdash; no account, no email, just XMR.
        </p>
        <div className="cube-divider">
          <span style={{ backgroundColor: 'var(--cube-orange)', animationDelay: '0s' }} />
          <span style={{ backgroundColor: 'var(--cube-blue)', animationDelay: '0.5s' }} />
          <span style={{ backgroundColor: 'var(--cube-green)', animationDelay: '1s' }} />
          <span style={{ backgroundColor: 'var(--cube-yellow)', animationDelay: '1.5s' }} />
        </div>
      </div>

      {/* What you get */}
      <TierSelector currentTier={currentTier} />

      {/* Roadmap */}
      <div className="mt-8 mb-8">
        <h2 className="text-lg font-semibold text-zinc-100 mb-4">Roadmap &amp; Planned Features</h2>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <div className="stat-card stat-card-blue">
            <h3 className="text-cube-blue font-semibold text-sm mb-3">Hosted P2Pool Nodes</h3>
            <p className="text-zinc-400 text-xs mb-3">
              Managed P2Pool nodes for subscribers. No syncing, no maintenance &mdash;
              just point your miner and go.
            </p>
            <ul className="space-y-1.5 text-xs text-zinc-300">
              <li className="flex items-start gap-2">
                <span className="text-green-400 mt-0.5">&#10003;</span>
                <span>P2Pool <strong>mini</strong> &mdash; live now, lower difficulty for smaller miners</span>
              </li>
              <li className="flex items-start gap-2">
                <span className="text-cube-blue mt-0.5">+</span>
                <span>P2Pool <strong>main</strong> &mdash; full difficulty for high-hashrate rigs <span className="text-zinc-500">(coming next)</span></span>
              </li>
              <li className="flex items-start gap-2">
                <span className="text-zinc-500 mt-0.5">?</span>
                <span className="text-zinc-500">P2Pool <strong>nano</strong> &mdash; ultra-low difficulty for solo CPUs <span>(not yet confirmed)</span></span>
              </li>
            </ul>
          </div>

          <div className="stat-card stat-card-green">
            <h3 className="text-cube-green font-semibold text-sm mb-3">Dashboard Enhancements</h3>
            <ul className="space-y-1.5 text-xs text-zinc-300">
              <li className="flex items-start gap-2">
                <span className="text-cube-green mt-0.5">+</span>
                <span>Main sidechain support (data layer already sidechain-agnostic)</span>
              </li>
              <li className="flex items-start gap-2">
                <span className="text-cube-green mt-0.5">+</span>
                <span>Notification alerts (block found, payment received)</span>
              </li>
              <li className="flex items-start gap-2">
                <span className="text-cube-green mt-0.5">+</span>
                <span>Advanced analytics &amp; mining profitability calculator</span>
              </li>
              <li className="flex items-start gap-2">
                <span className="text-cube-green mt-0.5">+</span>
                <span>Mobile-optimized miner view</span>
              </li>
              <li className="flex items-start gap-2">
                <span className="text-cube-green mt-0.5">+</span>
                <span>Multi-sidechain dashboard (mini + main in one view)</span>
              </li>
            </ul>
          </div>

          <div className="stat-card stat-card-yellow">
            <h3 className="text-cube-yellow font-semibold text-sm mb-3">Supporter Perks</h3>
            <ul className="space-y-1.5 text-xs text-zinc-300">
              <li className="flex items-start gap-2">
                <span className="text-cube-yellow mt-0.5">+</span>
                <span>Extended hashrate &amp; payment history (15 months)</span>
              </li>
              <li className="flex items-start gap-2">
                <span className="text-cube-yellow mt-0.5">+</span>
                <span>Tax CSV export with fiat conversion (USD/CAD)</span>
              </li>
              <li className="flex items-start gap-2">
                <span className="text-cube-yellow mt-0.5">+</span>
                <span>Per-worker stats breakdown</span>
              </li>
              <li className="flex items-start gap-2">
                <span className="text-cube-yellow mt-0.5">+</span>
                <span>API key for programmatic access</span>
              </li>
            </ul>
            <p className="text-zinc-600 text-xs mt-3">Available now for Supporter tier and above</p>
          </div>

          <div className="stat-card stat-card-orange">
            <h3 className="text-cube-orange font-semibold text-sm mb-3">Open Source &amp; Transparency</h3>
            <ul className="space-y-1.5 text-xs text-zinc-300">
              <li className="flex items-start gap-2">
                <span className="text-cube-orange mt-0.5">+</span>
                <span>AGPL-3.0 licensed &mdash; verify everything</span>
              </li>
              <li className="flex items-start gap-2">
                <span className="text-cube-orange mt-0.5">+</span>
                <span>Tor hidden service for privacy-first access</span>
              </li>
              <li className="flex items-start gap-2">
                <span className="text-cube-orange mt-0.5">+</span>
                <span>No IP logging tied to wallet addresses</span>
              </li>
              <li className="flex items-start gap-2">
                <span className="text-cube-orange mt-0.5">+</span>
                <span>Coinbase private keys published for trustless verification</span>
              </li>
            </ul>
            <p className="text-zinc-600 text-xs mt-3">These are promises, not features &mdash; built in from day one</p>
          </div>
        </div>
      </div>

      {/* Payment section */}
      <div ref={paymentRef} className="border-t border-zinc-800 pt-8 mt-8">
        {fromContext === 'tax-export' && (
          <div className="bg-zinc-900/80 border border-xmr-orange/30 rounded-lg p-4 mb-6">
            <p className="text-zinc-200 text-sm font-medium">Tax export requires Supporter tier</p>
            <p className="text-zinc-400 text-sm mt-1">
              Send as little as ~$1 in XMR below to unlock tax CSV exports, 15-month data retention, and more.
              Your subscription activates after 10 confirmations (~20 min).
            </p>
          </div>
        )}

        <h2 className="text-lg font-semibold text-zinc-100 mb-2">Pay with XMR</h2>
        <p className="text-zinc-400 text-sm mb-6">
          No account needed. Enter your mining wallet address and we&apos;ll generate a unique payment address for you.
        </p>

        {/* How it works steps — always visible */}
        {!activeAddress && (
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 mb-6">
            <div className="stat-card py-4 px-4">
              <div className="flex items-center gap-2 mb-2">
                <span className="text-cube-orange font-bold text-lg">1</span>
                <span className="text-zinc-200 text-sm font-medium">Enter your wallet address</span>
              </div>
              <p className="text-zinc-500 text-xs">The same address you mine to with XMRig. This links your subscription to your miner.</p>
            </div>
            <div className="stat-card py-4 px-4">
              <div className="flex items-center gap-2 mb-2">
                <span className="text-cube-blue font-bold text-lg">2</span>
                <span className="text-zinc-200 text-sm font-medium">Get your payment address</span>
              </div>
              <p className="text-zinc-500 text-xs">We generate a unique Monero subaddress just for you. Copy it and send XMR from any wallet.</p>
            </div>
            <div className="stat-card py-4 px-4">
              <div className="flex items-center gap-2 mb-2">
                <span className="text-cube-green font-bold text-lg">3</span>
                <span className="text-zinc-200 text-sm font-medium">Subscription activates</span>
              </div>
              <p className="text-zinc-500 text-xs">After 10 confirmations (~20 min), your tier unlocks automatically. No manual steps needed.</p>
            </div>
          </div>
        )}

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
              {activeAddress ? 'Change Address' : 'Get Payment Address'}
            </button>
          </div>
        </form>

        {statusError && (
          <div className="text-red-400 text-sm p-4 bg-red-900/20 border border-red-800 rounded-lg mb-6">
            Failed to load subscription status. Please check your address and try again.
          </div>
        )}

        {activeAddress && (
          <div className="space-y-6">
            <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
              <SubscriptionStatus
                status={subStatus || { miner_address: activeAddress, tier: 'free', active: false, expires_at: null, grace_until: null, has_api_key: false }}
                isLoading={statusLoading}
              />
            </div>

            <SubscriptionPayment
              paymentAddress={paymentAddress}
              payments={subPayments || []}
              isLoading={addressLoading || paymentsLoading}
            />

            {subStatus?.active && tierIncludes(subStatus.tier, 'supporter') && (
              <div className="stat-card">
                <p className="text-zinc-400 text-sm mb-3">API Key</p>
                {apiKey ? (
                  <div>
                    <code className="block bg-zinc-950 border border-zinc-800 rounded-lg p-3 text-xs font-mono text-green-400 break-all select-all mb-2">
                      {apiKey}
                    </code>
                    <p className="text-yellow-400 text-xs">
                      Store this key securely. It cannot be retrieved again.
                    </p>
                  </div>
                ) : (
                  <div>
                    <p className="text-zinc-500 text-xs mb-3">
                      Generate an API key for programmatic access. Pass it as the X-API-Key header.
                      {subStatus.has_api_key && ' Enter your existing key below to regenerate.'}
                    </p>
                    {subStatus.has_api_key && (
                      <input
                        type="text"
                        value={existingKeyInput}
                        onChange={(e) => setExistingKeyInput(e.target.value)}
                        placeholder="Paste your existing API key"
                        className="input-field w-full mb-3 text-xs font-mono"
                      />
                    )}
                    <button onClick={handleGenerateAPIKey} className="btn-secondary text-sm">
                      {subStatus.has_api_key ? 'Regenerate API Key' : 'Generate API Key'}
                    </button>
                  </div>
                )}
                {apiKeyError && (
                  <p className="text-red-400 text-xs mt-2">{apiKeyError}</p>
                )}
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  )
}
