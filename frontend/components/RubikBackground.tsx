'use client'

import { useState, useEffect, useMemo } from 'react'

const CUBE_COLORS = ['#f97316', '#3b82f6', '#22c55e', '#ef4444', '#eab308', '#a1a1aa']

/** Deterministic pseudo-random so positions are stable across renders. */
function seeded(seed: number): number {
  const x = Math.sin(seed * 9301 + 49297) * 49297
  return x - Math.floor(x)
}

interface Square {
  size: number
  x: string
  y: string
  color: string
  opacity: number
  duration: number
  delay: number
  solve: boolean
}

export default function RubikBackground() {
  const [mounted, setMounted] = useState(false)

  useEffect(() => { setMounted(true) }, [])

  const squares: Square[] = useMemo(() =>
    Array.from({ length: 28 }, (_, i) => ({
      size: 6 + seeded(i * 7) * 18,
      x: `${seeded(i * 3) * 100}%`,
      y: `${seeded(i * 5) * 100}%`,
      color: CUBE_COLORS[i % CUBE_COLORS.length],
      opacity: 0.03 + seeded(i * 11) * 0.06,
      duration: 25 + seeded(i * 13) * 45,
      delay: seeded(i * 17) * 15,
      solve: i % 7 === 0, // every 7th square gets a "solve flash"
    })),
  [])

  if (!mounted) return null

  return (
    <div
      className="fixed inset-0 pointer-events-none overflow-hidden z-0"
      aria-hidden="true"
    >
      {squares.map((sq, i) => (
        <div
          key={i}
          className="absolute rounded-sm will-change-transform"
          style={{
            width: sq.size,
            height: sq.size,
            left: sq.x,
            top: sq.y,
            backgroundColor: sq.color,
            color: sq.color,
            '--sq-opacity': sq.opacity,
            animation: sq.solve
              ? `rubik-float ${sq.duration}s ease-in-out ${sq.delay}s infinite, rubik-solve ${sq.duration * 0.6}s ease-in-out ${sq.delay + 5}s infinite`
              : `rubik-float ${sq.duration}s ease-in-out ${sq.delay}s infinite`,
            opacity: sq.opacity,
          } as React.CSSProperties}
        />
      ))}
    </div>
  )
}
