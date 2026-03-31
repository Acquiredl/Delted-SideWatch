'use client'

interface UncleRateWarningProps {
  uncleRate: number // 0.0 to 1.0
}

export default function UncleRateWarning({ uncleRate }: UncleRateWarningProps) {
  const pct = (uncleRate * 100).toFixed(1)
  const isElevated = uncleRate > 0.10

  if (isElevated) {
    return (
      <div className="bg-red-900/30 border border-red-700 rounded-lg px-4 py-3 mb-6">
        <p className="text-red-300 text-sm font-medium">
          Uncle rate elevated: {pct}% (last 24h)
        </p>
        <p className="text-red-300/80 text-xs mt-1">
          This reduces your effective contribution to the PPLNS window.
          Check your network latency to the P2Pool node and ensure your miner
          software is up to date.
        </p>
      </div>
    )
  }

  return (
    <div className="stat-card">
      <p className="text-zinc-400 text-sm mb-1">Uncle Rate (24h)</p>
      <p className="text-2xl font-bold text-green-400">{pct}%</p>
      <p className="text-zinc-500 text-xs mt-1">Normal range</p>
    </div>
  )
}
