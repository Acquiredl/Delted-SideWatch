'use client'

import useSWR from 'swr'
import { fetcher } from '@/lib/api'
import type { ConnectionInfoResponse } from '@/lib/api'
import XMRigConfig from '@/components/XMRigConfig'
import NodeHealth from '@/components/NodeHealth'

export default function ConnectPage() {
  const { data, error, isLoading } = useSWR<ConnectionInfoResponse>(
    '/api/nodes/connection-info',
    fetcher,
  )

  return (
    <div>
      <div className="mb-8">
        <h1 className="text-3xl font-bold text-zinc-100 mb-2">Connect to <span className="text-xmr-orange">SideWatch</span></h1>
        <p className="text-zinc-400 text-sm mb-3">
          Point your XMRig at our shared P2Pool node. No account, no registration,
          no wallet configuration needed in XMRig.
        </p>
        <div className="cube-divider">
          <span style={{ backgroundColor: 'var(--cube-orange)', animationDelay: '0s' }} />
          <span style={{ backgroundColor: 'var(--cube-blue)', animationDelay: '0.4s' }} />
          <span style={{ backgroundColor: 'var(--cube-green)', animationDelay: '0.8s' }} />
        </div>
      </div>

      <div className="space-y-6">
        {/* Step 1: Node status */}
        <div>
          <h2 className="text-lg font-semibold text-zinc-100 mb-3">
            <span className="text-cube-orange mr-2">1.</span> Node Status
          </h2>
          <NodeHealth />
        </div>

        {/* Step 2: Configure XMRig */}
        <div>
          <h2 className="text-lg font-semibold text-zinc-100 mb-3">
            <span className="text-cube-blue mr-2">2.</span> Configure XMRig
          </h2>
          <p className="text-zinc-400 text-sm mb-3">
            Don&apos;t have XMRig yet?{' '}
            <a
              href="https://xmrig.com/download"
              target="_blank"
              rel="noopener noreferrer"
              className="text-cube-blue hover:underline"
            >
              Download it from xmrig.com
            </a>
          </p>

          {isLoading && (
            <div className="stat-card animate-pulse">
              <div className="h-4 bg-zinc-800 rounded w-48 mb-3" />
              <div className="h-24 bg-zinc-800 rounded" />
            </div>
          )}

          {!isLoading && error && (
            <div className="stat-card">
              <p className="text-zinc-500 text-sm">Connection info unavailable. The node pool service may not be running yet.</p>
            </div>
          )}

          {data && data.nodes.length > 0 && (
            <div className="space-y-4">
              {data.nodes.map((node) => (
                <XMRigConfig key={node.name} node={node} />
              ))}
            </div>
          )}

          {data && data.nodes.length === 0 && (
            <div className="stat-card">
              <p className="text-zinc-500 text-sm">No nodes are currently running. Check back later.</p>
            </div>
          )}
        </div>

        {/* Step 3: Start mining */}
        <div>
          <h2 className="text-lg font-semibold text-zinc-100 mb-3">
            <span className="text-cube-green mr-2">3.</span> Start Mining
          </h2>
          <div className="stat-card stat-card-green">
            <ol className="list-decimal list-inside space-y-2 text-sm text-zinc-400">
              <li>Copy the stratum URL or full XMRig config from above</li>
              <li>Add it to your XMRig <code className="text-zinc-300">config.json</code> pools array</li>
              <li>Start XMRig &mdash; it will connect and begin submitting shares</li>
              <li>Visit the <a href="/miner" className="text-cube-blue hover:underline">Miner</a> page and enter the node&apos;s wallet address to see stats</li>
            </ol>
            <p className="text-zinc-500 text-xs mt-4">
              No wallet address needed in XMRig &mdash; the wallet is configured on the P2Pool
              node itself. All rewards go directly to the node&apos;s wallet via the Monero
              coinbase transaction. SideWatch never touches your funds.
            </p>
          </div>
        </div>

        {/* How the wallet model works */}
        <div>
          <h2 className="text-lg font-semibold text-zinc-100 mb-3">How P2Pool Wallets Work</h2>
          <div className="stat-card">
            <div className="space-y-3 text-sm text-zinc-400">
              <p>
                In P2Pool, the wallet address is set on the <strong className="text-zinc-200">node</strong>, not in XMRig.
                When you start a P2Pool node with <code className="text-zinc-300">--wallet YOUR_ADDRESS</code>,
                all miners connecting to that node&apos;s stratum contribute hashrate toward that wallet&apos;s PPLNS shares.
              </p>
              <p>
                <strong className="text-cube-orange">SideWatch runs the P2Pool node for you.</strong> The
                node&apos;s wallet is pre-configured &mdash; you just point XMRig at the stratum URL and mine.
                This means all connected miners share the same wallet and split the rewards proportionally
                based on submitted work.
              </p>
              <p>
                If you want payouts to go to <strong className="text-zinc-200">your own wallet</strong>, you
                need to run your own P2Pool node with your own <code className="text-zinc-300">--wallet</code> flag.
                SideWatch is open-source &mdash; see
                the <a href="https://github.com/acquiredl/xmr-p2pool-dashboard" className="text-cube-blue hover:underline">self-hosting guide</a> to
                run your own instance.
              </p>
            </div>
          </div>
        </div>

        {/* How it works */}
        <div>
          <h2 className="text-lg font-semibold text-zinc-100 mb-3">How P2Pool Works</h2>
          <div className="stat-card">
            <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 text-xs text-zinc-400">
              <div>
                <p className="text-cube-orange font-semibold text-sm mb-1">Decentralized</p>
                <p>No central pool operator. Miners share a sidechain and split rewards trustlessly via the Monero coinbase.</p>
              </div>
              <div>
                <p className="text-cube-blue font-semibold text-sm mb-1">Zero Fee</p>
                <p>P2Pool takes no cut. 100% of the block reward goes to miners based on their PPLNS shares.</p>
              </div>
              <div>
                <p className="text-cube-green font-semibold text-sm mb-1">Your Keys</p>
                <p>Payouts are built into the coinbase transaction. No pool wallet, no withdrawal, no trust required.</p>
              </div>
            </div>
          </div>
        </div>

        {/* Tor — not yet implemented */}
        <div>
          <h2 className="text-lg font-semibold text-zinc-100 mb-3">Tor Hidden Service</h2>
          <div className="stat-card">
            <p className="text-zinc-400 text-sm">
              Tor support is not yet implemented. A <code className="text-zinc-300">.onion</code> endpoint
              for maximum mining privacy is planned for a future release.
            </p>
          </div>
        </div>
      </div>
    </div>
  )
}
