'use client'

import Link from 'next/link'
import { usePathname } from 'next/navigation'

const sidechain = (process.env.NEXT_PUBLIC_SIDECHAIN || 'mini').charAt(0).toUpperCase() + (process.env.NEXT_PUBLIC_SIDECHAIN || 'mini').slice(1)

const navLinks = [
  { href: '/', label: 'Home' },
  { href: '/miner', label: 'Miner' },
  { href: '/blocks', label: 'Blocks' },
  { href: '/sidechain', label: 'Sidechain' },
  { href: '/fund', label: 'Fund' },
  { href: '/connect', label: 'Connect' },
  { href: '/subscribe', label: 'Subscribe' },
]

export default function Navigation() {
  const pathname = usePathname()

  return (
    <nav className="border-b border-zinc-800 bg-zinc-950/80 backdrop-blur-sm sticky top-0 z-50">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <div className="flex items-center justify-between h-16">
          <Link href="/" className="text-xmr-orange font-bold text-lg tracking-tight">
            P2Pool {sidechain}
          </Link>
          <div className="flex items-center gap-1">
            {navLinks.map((link) => {
              const isActive = pathname === link.href
              return (
                <Link
                  key={link.href}
                  href={link.href}
                  className={`px-3 py-2 rounded-md text-sm font-medium transition-colors ${
                    isActive
                      ? 'text-xmr-orange bg-zinc-900'
                      : 'text-zinc-400 hover:text-zinc-100 hover:bg-zinc-900/50'
                  }`}
                >
                  {link.label}
                </Link>
              )
            })}
          </div>
        </div>
      </div>
    </nav>
  )
}
