'use client'

import Link from 'next/link'
import { usePathname } from 'next/navigation'
import CubeLogo from './CubeLogo'

const sidechain = (process.env.NEXT_PUBLIC_SIDECHAIN || 'mini').toLowerCase()

const navLinks = [
  { href: '/', label: 'Home' },
  { href: '/miner', label: 'Miner' },
  { href: '/blocks', label: 'Blocks' },
  { href: '/sidechain', label: 'Sidechain' },
  { href: '/fund', label: 'Fund' },
  { href: '/connect', label: 'Connect' },
  { href: '/subscribe', label: 'Subscribe' },
  { href: '/about', label: 'About' },
]

export default function Navigation() {
  const pathname = usePathname()

  return (
    <nav className="border-b border-zinc-800 bg-zinc-950/90 sticky top-0 z-50 nav-blur">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <div className="flex items-center justify-between h-16">
          <Link href="/" className="flex items-center gap-3 group">
            <CubeLogo />
            <div className="flex items-baseline gap-2">
              <span className="text-xmr-orange font-bold text-lg tracking-tight">
                SideWatch
              </span>
              <span className="text-zinc-500 text-xs font-medium">
                {sidechain}
              </span>
            </div>
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
