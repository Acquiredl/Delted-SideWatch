'use client'

import { useState } from 'react'
import { formatXMR, formatDate } from '@/lib/api'
import type { PaymentAddress, SubPayment } from '@/lib/api'

interface SubscriptionPaymentProps {
  paymentAddress: PaymentAddress | undefined
  payments: SubPayment[]
  isLoading?: boolean
}

export default function SubscriptionPayment({ paymentAddress, payments, isLoading }: SubscriptionPaymentProps) {
  const [copied, setCopied] = useState(false)

  function handleCopy() {
    if (!paymentAddress) return
    navigator.clipboard.writeText(paymentAddress.subaddress)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  if (isLoading) {
    return (
      <div className="stat-card animate-pulse">
        <div className="h-4 bg-zinc-800 rounded w-40 mb-3" />
        <div className="h-12 bg-zinc-800 rounded mb-3" />
        <div className="h-4 bg-zinc-800 rounded w-32" />
      </div>
    )
  }

  return (
    <div className="space-y-6">
      {paymentAddress && (
        <div className="stat-card">
          <p className="text-zinc-400 text-sm mb-2">Send Payment To</p>
          <div className="flex items-start gap-2 mb-3">
            <code className="flex-1 bg-zinc-950 border border-zinc-800 rounded-lg p-3 text-xs font-mono text-xmr-orange break-all select-all">
              {paymentAddress.subaddress}
            </code>
            <button
              onClick={handleCopy}
              className="btn-secondary text-xs px-3 py-3 whitespace-nowrap"
            >
              {copied ? 'Copied' : 'Copy'}
            </button>
          </div>
          <div className="flex gap-4 text-sm">
            <span className="text-zinc-400">
              Amount: <span className="text-zinc-100 font-mono">{paymentAddress.suggested_amount_xmr} XMR</span>
            </span>
            <span className="text-zinc-500">
              (~{paymentAddress.amount_usd})
            </span>
          </div>
          <p className="text-zinc-600 text-xs mt-2">
            Price may vary slightly due to market fluctuation. Subscription activates after 10 confirmations (~20 min).
          </p>
        </div>
      )}

      {payments.length > 0 && (
        <div>
          <h3 className="text-lg font-bold text-zinc-100 mb-3">Subscription Payments</h3>
          <div className="table-container">
            <table className="data-table">
              <thead>
                <tr>
                  <th>Date</th>
                  <th>Amount</th>
                  <th>Status</th>
                  <th>TX Hash</th>
                </tr>
              </thead>
              <tbody>
                {payments.map((payment) => (
                  <tr key={payment.tx_hash}>
                    <td className="text-zinc-400">{formatDate(payment.created_at)}</td>
                    <td className="font-mono text-green-400">{formatXMR(payment.amount)} XMR</td>
                    <td>
                      <span className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium ${
                        payment.confirmed
                          ? 'bg-green-900/40 text-green-400'
                          : 'bg-yellow-900/40 text-yellow-400'
                      }`}>
                        {payment.confirmed ? 'Confirmed' : 'Pending'}
                      </span>
                    </td>
                    <td className="font-mono text-xs text-zinc-500">
                      {payment.tx_hash.slice(0, 8)}...{payment.tx_hash.slice(-8)}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  )
}
