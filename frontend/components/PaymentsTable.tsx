'use client'

import { formatXMR, formatUSD, formatCAD, formatDate } from '@/lib/api'
import type { MinerPayment } from '@/lib/api'

interface PaymentsTableProps {
  payments: MinerPayment[]
  isLoading?: boolean
}

export default function PaymentsTable({ payments, isLoading }: PaymentsTableProps) {
  if (isLoading) {
    return (
      <div className="stat-card">
        <div className="animate-pulse space-y-3">
          {Array.from({ length: 5 }).map((_, i) => (
            <div key={i} className="h-6 bg-zinc-800 rounded" />
          ))}
        </div>
      </div>
    )
  }

  if (payments.length === 0) {
    return (
      <div className="stat-card text-center text-zinc-500 py-8">
        No payments found
      </div>
    )
  }

  return (
    <div className="table-container">
      <table className="data-table">
        <thead>
          <tr>
            <th>Date</th>
            <th>Amount</th>
            <th>USD Value</th>
            <th>CAD Value</th>
            <th>Block Height</th>
          </tr>
        </thead>
        <tbody>
          {payments.map((payment, i) => (
            <tr key={`${payment.main_height}-${i}`}>
              <td className="text-zinc-400">{formatDate(payment.paid_at)}</td>
              <td className="font-mono text-green-400">{formatXMR(payment.amount)} XMR</td>
              <td className="font-mono text-zinc-300">
                {payment.xmr_usd_price ? formatUSD(payment.amount / 1e12 * payment.xmr_usd_price) : '--'}
              </td>
              <td className="font-mono text-zinc-300">
                {payment.xmr_cad_price ? formatCAD(payment.amount / 1e12 * payment.xmr_cad_price) : '--'}
              </td>
              <td className="font-mono text-zinc-400">{payment.main_height.toLocaleString()}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
