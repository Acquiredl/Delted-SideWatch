'use client'

export default function ConnectPage() {
  return (
    <div>
      <div className="mb-8">
        <h1 className="text-3xl font-bold text-zinc-100 mb-2">Connect to <span className="text-xmr-orange">SideWatch</span></h1>
        <div className="cube-divider">
          <span style={{ backgroundColor: 'var(--cube-orange)', animationDelay: '0s' }} />
          <span style={{ backgroundColor: 'var(--cube-blue)', animationDelay: '0.4s' }} />
          <span style={{ backgroundColor: 'var(--cube-green)', animationDelay: '0.8s' }} />
        </div>
      </div>

      {/* Warning banner */}
      <div className="bg-red-900/30 border-2 border-red-500 rounded-lg p-6 mb-6">
        <h2 className="text-red-400 text-lg font-bold mb-3">Do Not Connect to This Node</h2>
        <div className="space-y-3 text-sm text-zinc-300">
          <p>
            SideWatch is <strong className="text-red-400">not ready for public mining</strong>. The
            way P2Pool works, the wallet address is configured on the node itself &mdash; not
            in your XMRig miner. This means if you connect to this node, <strong className="text-red-400">all
            of your mining rewards go to the node operator&apos;s wallet, not yours</strong>.
          </p>
          <p>
            I originally built this under the mistaken assumption that P2Pool would split
            rewards between everyone connected to the node, similar to how traditional mining
            pools work. That is not how P2Pool works. In P2Pool, one node = one wallet. Every
            miner connecting to a node contributes hashrate to that single wallet. There is no
            per-miner payout.
          </p>
          <p>
            If you connect your XMRig to this stratum endpoint right now, you would be
            donating your hashrate and electricity to someone else for free. Please do not do that.
          </p>
        </div>
      </div>

      {/* What to do instead */}
      <div className="stat-card mb-6">
        <h2 className="text-lg font-semibold text-zinc-100 mb-3">What Should You Do Instead?</h2>
        <div className="space-y-3 text-sm text-zinc-400">
          <p>
            To mine with P2Pool and receive your own rewards, you need to run your own P2Pool
            node with <strong className="text-zinc-200">your own wallet address</strong>:
          </p>
          <pre className="bg-zinc-950 border border-zinc-800 rounded-lg p-3 text-xs font-mono text-zinc-300 overflow-x-auto">
{`# Start P2Pool with YOUR wallet
./p2pool --host 127.0.0.1 --wallet YOUR_MONERO_ADDRESS --mini

# Then point XMRig at it (no wallet needed in XMRig)
./xmrig -o 127.0.0.1:3333`}
          </pre>
          <p>
            SideWatch is open-source. You can use it as a dashboard for your own P2Pool
            node by following
            the <a href="https://github.com/acquiredl/xmr-p2pool-dashboard" className="text-cube-blue hover:underline">self-hosting guide</a>.
          </p>
        </div>
      </div>

      {/* Status */}
      <div className="stat-card">
        <h2 className="text-lg font-semibold text-zinc-100 mb-3">Project Status</h2>
        <p className="text-sm text-zinc-400">
          SideWatch was designed as a hosted observability dashboard where multiple miners
          could connect and see their stats. The underlying architecture needs to change
          before that vision can work &mdash; each miner would need their own P2Pool node
          with their own wallet. Until that is built, this page is disabled and the stratum
          endpoint should not be used.
        </p>
      </div>
    </div>
  )
}
