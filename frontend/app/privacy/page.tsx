export default function PrivacyPage() {
  return (
    <div>
      <div className="mb-8">
        <h1 className="text-3xl font-bold text-zinc-100 mb-2">Privacy Notice</h1>
        <p className="text-zinc-400 text-sm mb-3">
          SideWatch is designed with miner privacy as a core principle.
        </p>
        <div className="cube-divider">
          <span style={{ backgroundColor: 'var(--cube-green)', animationDelay: '0s' }} />
          <span style={{ backgroundColor: 'var(--cube-blue)', animationDelay: '0.5s' }} />
        </div>
      </div>

      <div className="space-y-6">
        <div className="stat-card stat-card-green">
          <h2 className="text-lg font-semibold text-zinc-100 mb-3">What We Store</h2>
          <ul className="space-y-2 text-sm text-zinc-300">
            <li className="flex items-start gap-2">
              <span className="text-cube-green mt-0.5">+</span>
              <span>
                <strong>Truncated wallet addresses</strong> &mdash; P2Pool only exposes the first ~32
                characters of miner addresses via the stratum API. We store what P2Pool provides.
              </span>
            </li>
            <li className="flex items-start gap-2">
              <span className="text-cube-green mt-0.5">+</span>
              <span>
                <strong>Hashrate and share history</strong> &mdash; bucketed per 15 minutes for
                dashboard display. Free tier retains 30 days; paid tier retains 15 months.
              </span>
            </li>
            <li className="flex items-start gap-2">
              <span className="text-cube-green mt-0.5">+</span>
              <span>
                <strong>Payment records</strong> &mdash; on-chain coinbase outputs linked to miner
                addresses. This is public blockchain data.
              </span>
            </li>
            <li className="flex items-start gap-2">
              <span className="text-cube-green mt-0.5">+</span>
              <span>
                <strong>Subscription payments</strong> &mdash; XMR transaction hashes and amounts
                for subscription verification. Stored only if you subscribe.
              </span>
            </li>
          </ul>
        </div>

        <div className="stat-card stat-card-blue">
          <h2 className="text-lg font-semibold text-zinc-100 mb-3">What We Do Not Store</h2>
          <ul className="space-y-2 text-sm text-zinc-300">
            <li className="flex items-start gap-2">
              <span className="text-cube-blue mt-0.5">&ndash;</span>
              <span><strong>IP addresses</strong> &mdash; we do not log or associate IP addresses with wallet lookups</span>
            </li>
            <li className="flex items-start gap-2">
              <span className="text-cube-blue mt-0.5">&ndash;</span>
              <span><strong>Email or personal information</strong> &mdash; no accounts, no registration, no KYC</span>
            </li>
            <li className="flex items-start gap-2">
              <span className="text-cube-blue mt-0.5">&ndash;</span>
              <span><strong>Cookies or tracking</strong> &mdash; no analytics, no ad networks, no third-party scripts</span>
            </li>
            <li className="flex items-start gap-2">
              <span className="text-cube-blue mt-0.5">&ndash;</span>
              <span><strong>Full wallet addresses</strong> &mdash; only the truncated prefix from P2Pool&apos;s stratum API</span>
            </li>
          </ul>
        </div>

        <div className="stat-card">
          <h2 className="text-lg font-semibold text-zinc-100 mb-3">Recommendations</h2>
          <div className="space-y-2 text-sm text-zinc-400">
            <p>
              For maximum privacy, consider connecting to the SideWatch stratum via our
              <strong className="text-zinc-300"> Tor hidden service</strong>. This prevents your ISP
              or network from seeing that you are mining with P2Pool.
            </p>
            <p>
              Use a <strong className="text-zinc-300">VPN</strong> or Tor when accessing the dashboard
              if you prefer not to associate your IP with a specific wallet lookup.
            </p>
            <p>
              SideWatch is <strong className="text-zinc-300">open source</strong> (AGPL-3.0). You can
              audit the entire codebase to verify these claims.
            </p>
          </div>
        </div>

        <div className="stat-card">
          <h2 className="text-lg font-semibold text-zinc-100 mb-3">Data Retention</h2>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4 text-sm">
            <div>
              <p className="text-zinc-400 mb-1">Free Tier</p>
              <p className="text-zinc-300">30-day rolling window. Hashrate, shares, and payment
                display data are pruned daily.</p>
            </div>
            <div>
              <p className="text-zinc-400 mb-1">Supporter / Champion Tier</p>
              <p className="text-zinc-300">15 months from the date of your first subscription payment.
                Includes full payment history and tax export access.</p>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
