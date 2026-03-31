'use client'

import { useState, useEffect } from 'react'
import { formatDuration, formatHashrate } from '@/lib/api'

interface ShareTimeCalculatorProps {
  minerHashrate: number       // H/s from miner stats
  sidechainDifficulty: number // from pool stats
}

export default function ShareTimeCalculator({ minerHashrate, sidechainDifficulty }: ShareTimeCalculatorProps) {
  const [customHashrate, setCustomHashrate] = useState('')
  const [effectiveHashrate, setEffectiveHashrate] = useState(minerHashrate)

  useEffect(() => {
    setEffectiveHashrate(minerHashrate)
    setCustomHashrate('')
  }, [minerHashrate])

  function handleHashrateChange(value: string) {
    setCustomHashrate(value)
    const parsed = parseFloat(value)
    if (!isNaN(parsed) && parsed > 0) {
      setEffectiveHashrate(parsed)
    } else if (value === '') {
      setEffectiveHashrate(minerHashrate)
    }
  }

  const expectedSeconds = effectiveHashrate > 0 && sidechainDifficulty > 0
    ? sidechainDifficulty / effectiveHashrate
    : 0

  return (
    <div className="stat-card">
      <p className="text-zinc-400 text-sm mb-1">Expected Share Time</p>
      <p className="text-2xl font-bold text-zinc-100">
        {expectedSeconds > 0 ? formatDuration(expectedSeconds) : '--'}
      </p>
      <p className="text-zinc-500 text-xs mt-1">
        at {formatHashrate(effectiveHashrate)} with current sidechain difficulty
      </p>
      <div className="mt-3">
        <input
          type="number"
          value={customHashrate}
          onChange={(e) => handleHashrateChange(e.target.value)}
          placeholder={`Override (${formatHashrate(minerHashrate)})`}
          className="input-field w-full text-xs"
          min="0"
          step="any"
        />
        <p className="text-zinc-600 text-xs mt-1">Enter H/s to calculate with a different hashrate</p>
      </div>
    </div>
  )
}
