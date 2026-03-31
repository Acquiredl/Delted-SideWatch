import type { Metadata } from 'next'
import './globals.css'
import Navigation from '@/components/Navigation'

const sidechain = (process.env.NEXT_PUBLIC_SIDECHAIN || 'mini').charAt(0).toUpperCase() + (process.env.NEXT_PUBLIC_SIDECHAIN || 'mini').slice(1)

export const metadata: Metadata = {
  title: `SideWatch — P2Pool ${sidechain}`,
  description: `SideWatch: Monero P2Pool ${sidechain} mining dashboard — decentralized, zero-fee mining statistics`,
}

export default function RootLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return (
    <html lang="en" className="dark">
      <body className="min-h-screen bg-zinc-950 text-zinc-100 flex flex-col">
        <Navigation />
        <main className="flex-1 max-w-7xl mx-auto w-full px-4 sm:px-6 lg:px-8 py-8">
          {children}
        </main>
        <footer className="border-t border-zinc-800 py-6 mt-auto">
          <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 flex flex-col sm:flex-row justify-between items-center gap-2 text-sm text-zinc-500">
            <span>P2Pool {sidechain} Dashboard</span>
            <a
              href="/privacy"
              className="hover:text-zinc-300 transition-colors"
            >
              Privacy Notice
            </a>
          </div>
        </footer>
      </body>
    </html>
  )
}
