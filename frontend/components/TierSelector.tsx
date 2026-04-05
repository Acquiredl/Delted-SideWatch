'use client'

import { useState } from 'react'
import type { SubscriptionTier } from '@/lib/api'

interface TierSelectorProps {
  currentTier: SubscriptionTier
}

const tiers = [
  {
    slug: 'free' as const,
    name: 'Free',
    min: 0,
    suggested: 0,
    features: [
      'Dashboard with 30-day data',
      'Shared node access',
      'Basic hashrate & payment stats',
    ],
    color: 'zinc',
  },
  {
    slug: 'supporter' as const,
    name: 'Supporter',
    min: 1,
    suggested: 3,
    features: [
      'Up to 15-month data retention (while active)',
      'Tax export (CSV with fiat, API key required)',
      'Unlimited hashrate history',
      'API key access',
      'Name on supporters page',
    ],
    color: 'blue',
  },
  {
    slug: 'champion' as const,
    name: 'Champion',
    min: 5,
    suggested: 7,
    features: [
      'Everything in Supporter',
      'Highlighted on supporters page',
      'Priority support channel',
    ],
    color: 'amber',
  },
]

export default function TierSelector({ currentTier }: TierSelectorProps) {
  const [amount, setAmount] = useState(3)

  const effectiveTier: SubscriptionTier =
    amount >= 5 ? 'champion' : amount >= 1 ? 'supporter' : 'free'

  return (
    <div className="stat-card">
      <h3 className="text-lg font-semibold text-zinc-100 mb-4">Choose Your Tier</h3>
      <p className="text-zinc-400 text-sm mb-4">
        Pay what you want above the minimum. All contributions fund the shared node infrastructure.
      </p>

      {/* Amount slider */}
      <div className="mb-6">
        <div className="flex items-center justify-between mb-2">
          <label className="text-sm text-zinc-400">Monthly contribution</label>
          <span className="text-lg font-mono font-bold text-zinc-100">
            ${amount.toFixed(0)}/mo
          </span>
        </div>
        <input
          type="range"
          min={0}
          max={20}
          step={1}
          value={amount}
          onChange={(e) => setAmount(Number(e.target.value))}
          className="w-full accent-blue-500"
        />
        <div className="flex justify-between text-xs text-zinc-500 mt-1">
          <span>$0 Free</span>
          <span>$1+ Supporter</span>
          <span>$5+ Champion</span>
        </div>
      </div>

      {/* Tier cards */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        {tiers.map((tier) => {
          const isActive = tier.slug === currentTier
          const isSelected = tier.slug === effectiveTier
          const borderColor = isSelected
            ? tier.color === 'amber'
              ? 'border-amber-500'
              : tier.color === 'blue'
                ? 'border-blue-500'
                : 'border-zinc-600'
            : 'border-zinc-800'

          return (
            <div
              key={tier.slug}
              className={`rounded-lg border ${borderColor} p-4 transition-colors ${
                isSelected ? 'bg-zinc-800/50' : 'bg-zinc-900/30'
              }`}
            >
              <div className="flex items-center gap-2 mb-2">
                <h4 className="font-semibold text-zinc-100">{tier.name}</h4>
                {isActive && (
                  <span className="text-xs bg-green-900/50 text-green-400 border border-green-800 rounded px-1.5 py-0.5">
                    Current
                  </span>
                )}
              </div>
              <p className="text-sm text-zinc-400 mb-3">
                {tier.min === 0
                  ? 'Free'
                  : `$${tier.min}+/mo (suggested $${tier.suggested})`}
              </p>
              <ul className="space-y-1.5">
                {tier.features.map((feature) => (
                  <li key={feature} className="flex items-start gap-2 text-xs text-zinc-300">
                    <span className="text-green-400 mt-0.5">+</span>
                    <span>{feature}</span>
                  </li>
                ))}
              </ul>
            </div>
          )
        })}
      </div>

      {effectiveTier !== 'free' && (
        <p className="text-sm text-zinc-400 mt-4">
          At ${amount}/mo you unlock <span className="text-zinc-100 font-medium">{effectiveTier}</span> tier.
          Enter your wallet address below to get a payment address.
        </p>
      )}
    </div>
  )
}
