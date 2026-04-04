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
          Point your XMRig at our shared P2Pool node. No account, no registration &mdash;
          just your Monero wallet address.
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
              <li>Replace <code className="text-zinc-300">YOUR_WALLET_ADDRESS</code> with your Monero address</li>
              <li>Start XMRig &mdash; it will connect and begin submitting shares</li>
              <li>Visit the <a href="/miner" className="text-cube-blue hover:underline">Miner</a> page and enter your address to see stats</li>
            </ol>
            <p className="text-zinc-500 text-xs mt-4">
              P2Pool is trustless &mdash; rewards go directly to your wallet via the Monero
              coinbase transaction. SideWatch never touches your funds.
            </p>
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
