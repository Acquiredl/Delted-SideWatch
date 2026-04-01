'use client'

import FundProgress from '@/components/FundProgress'
import FundHistory from '@/components/FundHistory'
import SupportersPage from '@/components/SupportersPage'
import NodeHealth from '@/components/NodeHealth'

export default function FundPage() {
  return (
    <div>
      <div className="mb-8">
        <h1 className="text-3xl font-bold text-zinc-100 mb-2">Node Fund</h1>
        <p className="text-zinc-400 text-sm">
          SideWatch runs on community contributions. Every dollar is transparently
          allocated to infrastructure and maintenance.
        </p>
      </div>

      <div className="space-y-6">
        <FundProgress />

        <div>
          <h2 className="text-lg font-semibold text-zinc-100 mb-3">Shared Nodes</h2>
          <NodeHealth />
        </div>

        <FundHistory />
        <SupportersPage />

        <div className="stat-card">
          <h3 className="text-sm font-medium text-zinc-400 mb-3">Transparency</h3>
          <div className="text-sm text-zinc-500 space-y-2">
            <p>
              The funding goal covers real infrastructure costs (VPS, storage, domain)
              plus operator time for monitoring, maintenance, and development.
            </p>
            <p>
              If the fund exceeds the goal, surplus goes to the operator as compensation
              for growing and maintaining the community.
            </p>
            <p>
              This is community-funded infrastructure, not a business. The codebase is
              open-source. Anyone can verify the costs.
            </p>
          </div>
        </div>
      </div>
    </div>
  )
}
