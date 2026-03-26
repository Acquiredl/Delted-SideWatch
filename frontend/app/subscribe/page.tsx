'use client'

import { useState, FormEvent } from 'react'
import useSWR from 'swr'
import { fetcher, postJSON } from '@/lib/api'
import type { SubscriptionStatus as SubStatusType, PaymentAddress, SubPayment } from '@/lib/api'
import SubscriptionStatus from '@/components/SubscriptionStatus'
import SubscriptionPayment from '@/components/SubscriptionPayment'

export default function SubscribePage() {
  const [address, setAddress] = useState('')
  const [activeAddress, setActiveAddress] = useState<string | null>(null)
  const [apiKey, setApiKey] = useState<string | null>(null)
  const [apiKeyError, setApiKeyError] = useState<string | null>(null)

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
      const result = await postJSON<{ api_key: string; note: string }>(
        `/api/subscription/api-key/${activeAddress}`
      )
      setApiKey(result.api_key)
    } catch (err) {
      setApiKeyError('Active paid subscription required to generate an API key.')
    }
  }

  return (
    <div>
      <div className="mb-8">
        <h1 className="text-3xl font-bold text-zinc-100 mb-2">Subscribe</h1>
        <p className="text-zinc-400 text-sm mb-2">
          Upgrade to unlock unlimited hashrate history, full payment history, and tax export.
        </p>
        <p className="text-zinc-500 text-xs">
          ~$5/month in XMR. No account, no email required. Pay on-chain.
        </p>
      </div>

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

            <div className="stat-card">
              <p className="text-zinc-400 text-sm mb-2">Paid Tier Benefits</p>
              <ul className="space-y-2 text-sm">
                <li className="flex items-center gap-2">
                  <span className="text-green-400">+</span>
                  <span className="text-zinc-300">Unlimited hashrate history</span>
                </li>
                <li className="flex items-center gap-2">
                  <span className="text-green-400">+</span>
                  <span className="text-zinc-300">Full payment history</span>
                </li>
                <li className="flex items-center gap-2">
                  <span className="text-green-400">+</span>
                  <span className="text-zinc-300">Tax export (CSV with fiat values)</span>
                </li>
                <li className="flex items-center gap-2">
                  <span className="text-green-400">+</span>
                  <span className="text-zinc-300">API key for programmatic access</span>
                </li>
              </ul>
            </div>
          </div>

          <SubscriptionPayment
            paymentAddress={paymentAddress}
            payments={subPayments || []}
            isLoading={addressLoading || paymentsLoading}
          />

          {subStatus?.tier === 'paid' && subStatus.active && (
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
                    {subStatus.has_api_key && ' Generating a new key will replace the existing one.'}
                  </p>
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

      {!activeAddress && (
        <div className="text-center text-zinc-500 py-16">
          Enter your wallet address above to view your subscription status.
        </div>
      )}
    </div>
  )
}
