'use client'

import { useState, FormEvent } from 'react'
import useSWR from 'swr'
import { fetcher, postJSON, tierIncludes } from '@/lib/api'
import type { SubscriptionStatus as SubStatusType, PaymentAddress, SubPayment } from '@/lib/api'
import SubscriptionStatus from '@/components/SubscriptionStatus'
import SubscriptionPayment from '@/components/SubscriptionPayment'
import TierSelector from '@/components/TierSelector'

export default function SubscribePage() {
  const [address, setAddress] = useState('')
  const [activeAddress, setActiveAddress] = useState<string | null>(null)
  const [apiKey, setApiKey] = useState<string | null>(null)
  const [apiKeyError, setApiKeyError] = useState<string | null>(null)
  const [existingKeyInput, setExistingKeyInput] = useState('')

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
        // Regeneration: send existing key in header.
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
        // First-time: send the most recent confirmed tx_hash.
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
    } catch (err) {
      setApiKeyError('Failed to generate API key. Check your credentials and try again.')
    }
  }

  const currentTier = subStatus?.tier ?? 'free'

  return (
    <div>
      <div className="mb-8">
        <h1 className="text-3xl font-bold text-zinc-100 mb-2">Support SideWatch</h1>
        <p className="text-zinc-400 text-sm mb-2">
          Fund the shared node infrastructure and unlock dashboard features.
          Pay what you want above the tier minimum.
        </p>
        <p className="text-zinc-500 text-xs">
          $1+/mo Supporter &middot; $5+/mo Champion &middot; No account, no email. Pay on-chain with XMR.
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

            <TierSelector currentTier={currentTier} />
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

      {!activeAddress && (
        <div className="text-center text-zinc-500 py-16">
          Enter your wallet address above to view your subscription status.
        </div>
      )}
    </div>
  )
}
