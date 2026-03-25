import LiveStats from '@/components/LiveStats'

export default function HomePage() {
  return (
    <div>
      <div className="mb-8">
        <h1 className="text-3xl font-bold text-zinc-100 mb-2">P2Pool Mini Dashboard</h1>
        <p className="text-zinc-400 text-sm">
          Decentralized Monero mining — no fees, no registration, no custody.
          Your keys, your coins.
        </p>
      </div>
      <LiveStats />
    </div>
  )
}
