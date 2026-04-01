'use client'

import useSWR from 'swr'
import { fetcher } from '@/lib/api'
import type { ConnectionInfoResponse } from '@/lib/api'
import XMRigConfig from '@/components/XMRigConfig'
import NodeHealth from '@/components/NodeHealth'

export default function ConnectPage() {
  const { data, isLoading } = useSWR<ConnectionInfoResponse>(
    '/api/nodes/connection-info',
    fetcher,
  )

  return (
    <div>
      <div className="mb-8">
        <h1 className="text-3xl font-bold text-zinc-100 mb-2">Connect to SideWatch</h1>
        <p className="text-zinc-400 text-sm">
          Point your XMRig at our shared P2Pool node. No account, no registration &mdash;
          just your wallet address.
        </p>
      </div>

      <div className="space-y-6">
        {/* Step 1: Choose a node */}
        <div>
          <h2 className="text-lg font-semibold text-zinc-100 mb-3">1. Node Status</h2>
          <NodeHealth />
        </div>

        {/* Step 2: Configure XMRig */}
        <div>
          <h2 className="text-lg font-semibold text-zinc-100 mb-3">2. Configure XMRig</h2>

          {isLoading && (
            <div className="stat-card animate-pulse">
              <div className="h-4 bg-zinc-800 rounded w-48 mb-3" />
              <div className="h-24 bg-zinc-800 rounded" />
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

        {/* Step 3: Verify */}
        <div>
          <h2 className="text-lg font-semibold text-zinc-100 mb-3">3. Start Mining</h2>
          <div className="stat-card">
            <ol className="list-decimal list-inside space-y-2 text-sm text-zinc-400">
              <li>Add the pool config above to your XMRig <code className="text-zinc-300">config.json</code></li>
              <li>Start XMRig &mdash; it will connect and begin submitting shares</li>
              <li>Visit the <a href="/miner" className="text-blue-400 hover:underline">Miner</a> page and enter your address to see stats</li>
              <li>Shares and payments appear within minutes</li>
            </ol>
            <p className="text-zinc-500 text-xs mt-4">
              P2Pool is trustless &mdash; rewards go directly to your wallet via the Monero
              coinbase transaction. SideWatch never touches your funds.
            </p>
          </div>
        </div>

        {/* Tor */}
        {data?.onion_url && (
          <div>
            <h2 className="text-lg font-semibold text-zinc-100 mb-3">Tor Hidden Service</h2>
            <div className="stat-card">
              <p className="text-zinc-400 text-sm mb-2">
                For maximum privacy, connect via our .onion address:
              </p>
              <code className="block bg-zinc-950 border border-zinc-800 rounded-lg p-3 text-sm font-mono text-purple-400 break-all select-all">
                {data.onion_url}
              </code>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
