import LiveStats from '@/components/LiveStats'

const sidechain = (process.env.NEXT_PUBLIC_SIDECHAIN || 'mini').charAt(0).toUpperCase() + (process.env.NEXT_PUBLIC_SIDECHAIN || 'mini').slice(1)

export default function HomePage() {
  return (
    <div>
      <div className="mb-8">
        <h1 className="text-3xl font-bold text-zinc-100 mb-2">P2Pool {sidechain} Dashboard</h1>
        <p className="text-zinc-400 text-sm">
          Decentralized Monero mining — no fees, no registration, no custody.
          Your keys, your coins.
        </p>
      </div>
      <LiveStats />
    </div>
  )
}
