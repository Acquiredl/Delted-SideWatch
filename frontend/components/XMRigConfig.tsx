'use client'

import { useState } from 'react'
import type { NodeConnectionInfo } from '@/lib/api'

interface XMRigConfigProps {
  node: NodeConnectionInfo
}

export default function XMRigConfig({ node }: XMRigConfigProps) {
  const [copied, setCopied] = useState(false)

  const configJSON = JSON.stringify(
    {
      url: node.xmrig_config.url,
      user: node.xmrig_config.user,
      pass: node.xmrig_config.pass,
    },
    null,
    2,
  )

  function handleCopy() {
    navigator.clipboard.writeText(configJSON)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  const healthColor =
    node.status === 'healthy'
      ? 'text-green-400'
      : node.status === 'syncing'
        ? 'text-yellow-400'
        : 'text-zinc-500'

  return (
    <div className="stat-card">
      <div className="flex items-center justify-between mb-3">
        <div className="flex items-center gap-2">
          <h4 className="text-zinc-100 font-semibold">{node.name}</h4>
          <span className={`text-xs ${healthColor}`}>
            {node.status}
          </span>
        </div>
        <span className="text-xs text-zinc-500">{node.sidechain} sidechain</span>
      </div>

      <p className="text-zinc-400 text-sm mb-2">Stratum URL</p>
      <code className="block bg-zinc-950 border border-zinc-800 rounded-lg p-3 text-sm font-mono text-xmr-orange mb-4 select-all">
        {node.stratum_url}
      </code>

      <p className="text-zinc-400 text-sm mb-2">XMRig pool config</p>
      <div className="relative">
        <pre className="bg-zinc-950 border border-zinc-800 rounded-lg p-3 text-xs font-mono text-zinc-300 overflow-x-auto select-all">
          {configJSON}
        </pre>
        <button
          onClick={handleCopy}
          className="absolute top-2 right-2 btn-secondary text-xs px-2 py-1"
        >
          {copied ? 'Copied' : 'Copy'}
        </button>
      </div>

      <p className="text-zinc-600 text-xs mt-3">
        Replace <code className="text-zinc-400">YOUR_WALLET_ADDRESS</code> with your Monero address.
        No registration needed &mdash; just point and mine.
      </p>
    </div>
  )
}
