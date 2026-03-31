'use client'

import { formatRelativeTime } from '@/lib/api'
import type { SubscriptionStatus as SubStatus } from '@/lib/api'

interface SubscriptionStatusProps {
  status: SubStatus
  isLoading?: boolean
}

export default function SubscriptionStatus({ status, isLoading }: SubscriptionStatusProps) {
  if (isLoading) {
    return (
      <div className="stat-card animate-pulse">
        <div className="h-4 bg-zinc-800 rounded w-32 mb-3" />
        <div className="h-8 bg-zinc-800 rounded w-24 mb-2" />
        <div className="h-4 bg-zinc-800 rounded w-48" />
      </div>
    )
  }

  const isPaid = status.tier === 'paid' && status.active
  const isGrace = status.tier === 'paid' && !status.active && status.grace_until &&
    new Date(status.grace_until) > new Date()

  return (
    <div className="stat-card">
      <p className="text-zinc-400 text-sm mb-2">Subscription Tier</p>
      <div className="flex items-center gap-3 mb-3">
        <span
          className={`inline-flex items-center px-3 py-1 rounded-full text-sm font-medium ${
            isPaid
              ? 'bg-green-900/40 text-green-400 border border-green-800'
              : isGrace
                ? 'bg-yellow-900/40 text-yellow-400 border border-yellow-800'
                : 'bg-zinc-800 text-zinc-400 border border-zinc-700'
          }`}
        >
          {isPaid ? 'Paid' : isGrace ? 'Grace Period' : 'Free'}
        </span>
        {status.has_api_key && (
          <span className="text-xs text-zinc-500">API key active</span>
        )}
      </div>

      {isPaid && status.expires_at && (
        <>
          <p className="text-zinc-400 text-sm">
            Expires: {new Date(status.expires_at).toLocaleDateString('en-US', {
              year: 'numeric', month: 'short', day: 'numeric'
            })}
            <span className="text-zinc-500 ml-2">({formatRelativeTime(status.expires_at)})</span>
          </p>
          <p className="text-zinc-500 text-xs mt-2">
            Extended history (up to 15 months) is stored from your first payment
            after subscribing. We cannot retroactively retrieve older history.
          </p>
        </>
      )}

      {isGrace && status.grace_until && (
        <p className="text-yellow-400 text-sm">
          Grace period ends: {new Date(status.grace_until).toLocaleDateString('en-US', {
            year: 'numeric', month: 'short', day: 'numeric'
          })}
        </p>
      )}

      {!isPaid && !isGrace && (
        <div className="text-zinc-500 text-sm">
          <p>Free tier limits:</p>
          <ul className="list-disc list-inside mt-1 text-xs">
            <li>30 days hashrate history (paid: 15 months)</li>
            <li>100 payments displayed (paid: unlimited)</li>
            <li>No tax CSV export (paid: full export)</li>
            <li>Data pruned after 30 days (paid: retained 15 months)</li>
          </ul>
        </div>
      )}
    </div>
  )
}
