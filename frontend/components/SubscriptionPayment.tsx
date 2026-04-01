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

          {/* Pay-what-you-want tier guidance */}
          <div className="bg-zinc-900/50 border border-zinc-800 rounded-lg p-3 mb-3">
            <p className="text-zinc-300 text-sm font-medium mb-2">Pay what you want</p>
            <div className="grid grid-cols-3 gap-2 text-xs">
              <div className="text-center p-2 rounded bg-zinc-800/50">
                <p className="text-blue-400 font-medium">$1+</p>
                <p className="text-zinc-500">Supporter</p>
              </div>
              <div className="text-center p-2 rounded bg-zinc-800/50">
                <p className="text-zinc-300 font-medium">$3</p>
                <p className="text-zinc-500">Suggested</p>
              </div>
              <div className="text-center p-2 rounded bg-zinc-800/50">
                <p className="text-amber-400 font-medium">$5+</p>
                <p className="text-zinc-500">Champion</p>
              </div>
            </div>
          </div>

          <div className="flex gap-4 text-sm">
            <span className="text-zinc-400">
              Suggested: <span className="text-zinc-100 font-mono">{paymentAddress.suggested_amount_xmr} XMR</span>
            </span>
            <span className="text-zinc-500">
              ({paymentAddress.amount_usd})
            </span>
          </div>
          <p className="text-zinc-600 text-xs mt-2">
            Send any amount above the tier minimum. Subscription activates after 10 confirmations (~20 min).
            All contributions fund the shared node infrastructure.
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
