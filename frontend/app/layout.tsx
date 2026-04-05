import type { Metadata } from 'next'
import './globals.css'
import Navigation from '@/components/Navigation'
import RubikBackground from '@/components/RubikBackground'

const sidechain = (process.env.NEXT_PUBLIC_SIDECHAIN || 'mini').charAt(0).toUpperCase() + (process.env.NEXT_PUBLIC_SIDECHAIN || 'mini').slice(1)

export const metadata: Metadata = {
  title: `SideWatch — P2Pool ${sidechain} Dashboard`,
  description: `SideWatch: Monero P2Pool ${sidechain} mining dashboard. Decentralized, zero-fee mining stats. Your keys, your coins.`,
}

export default function RootLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return (
    <html lang="en" className="dark">
      <body className="min-h-screen bg-zinc-950 text-zinc-100 flex flex-col">
        <RubikBackground />
        <Navigation />
        <main className="flex-1 max-w-7xl mx-auto w-full px-4 sm:px-6 lg:px-8 py-8 relative z-10">
          {children}
        </main>
        <footer className="border-t border-zinc-800 py-6 mt-auto relative z-10">
          <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 flex flex-col sm:flex-row justify-between items-center gap-4 text-sm text-zinc-500">
            <div className="flex items-center gap-3">
              <span className="text-xmr-orange font-semibold">SideWatch</span>
              <span className="text-zinc-600">|</span>
              <span>Decentralized P2Pool {sidechain} mining</span>
            </div>
            <div className="flex items-center gap-4">
              <a
                href="/about"
                className="hover:text-zinc-300 transition-colors"
              >
                About
              </a>
              <span className="text-zinc-700">|</span>
              <a
                href="/privacy"
                className="hover:text-zinc-300 transition-colors"
              >
                Privacy
              </a>
              <span className="text-zinc-700">|</span>
              <span className="text-zinc-600 text-xs">
                No accounts. No custody. Your keys, your coins.
              </span>
            </div>
          </div>
        </footer>
      </body>
    </html>
  )
}
