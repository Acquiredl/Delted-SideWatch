export default function AboutPage() {
  return (
    <div>
      <div className="mb-8">
        <h1 className="text-3xl font-bold text-zinc-100 mb-2">About SideWatch</h1>
        <p className="text-zinc-400 text-sm mb-3">
          What this project is, what it isn&apos;t, and why it exists.
        </p>
        <div className="cube-divider">
          <span style={{ backgroundColor: 'var(--cube-orange)', animationDelay: '0s' }} />
          <span style={{ backgroundColor: 'var(--cube-blue)', animationDelay: '0.4s' }} />
          <span style={{ backgroundColor: 'var(--cube-green)', animationDelay: '0.8s' }} />
        </div>
      </div>

      <div className="space-y-6">
        <div className="stat-card stat-card-green">
          <h2 className="text-lg font-semibold text-zinc-100 mb-3">
            Tax exports &mdash; the thing nobody else does
          </h2>
          <p className="text-zinc-300 text-sm mb-3">
            If you mine on P2Pool and file taxes in Canada or the US, you need a record of
            every coinbase payment with the fiat value at the time it was received. That data
            doesn&apos;t exist anywhere in a usable format.
          </p>
          <p className="text-zinc-400 text-sm">
            SideWatch records each payment with the XMR/USD and XMR/CAD spot price from
            CoinGecko and lets you download a CSV broken down by tax year. This is the
            strongest reason to use SideWatch over other tools &mdash; it solves a real
            problem that{' '}
            <a href="https://mini.p2pool.observer" target="_blank" rel="noopener noreferrer" className="text-zinc-300 hover:text-zinc-100 underline">
              mini.p2pool.observer
            </a>{' '}
            and the P2Pool node itself don&apos;t address.
          </p>
        </div>

        <div className="stat-card stat-card-blue">
          <h2 className="text-lg font-semibold text-zinc-100 mb-3">
            The managed node trade-off
          </h2>
          <p className="text-zinc-300 text-sm mb-3">
            P2Pool&apos;s entire philosophy is &ldquo;don&apos;t trust anyone, run your own
            node.&rdquo; SideWatch asks you to trust us to run the node for you. That&apos;s
            a real philosophical tension, and we won&apos;t pretend it isn&apos;t.
          </p>
          <p className="text-zinc-400 text-sm mb-3">
            The trade-off is convenience. Syncing a full Monero node (~200 GB), running a
            P2Pool sidechain node alongside it, and keeping both online 24/7 with updates and
            monitoring is a meaningful commitment. Most hobby miners just want to point XMRig
            at something and see their stats.
          </p>
          <p className="text-zinc-400 text-sm">
            If you have the resources and inclination to run your own P2Pool node, you
            absolutely should &mdash; it strengthens the network and gives you full sovereignty.
            SideWatch is for the miners who don&apos;t want that overhead. Both options give you
            zero-fee, direct-to-wallet payouts via the Monero blockchain.
          </p>
        </div>

        <div className="stat-card stat-card-orange">
          <h2 className="text-lg font-semibold text-zinc-100 mb-3">
            How SideWatch compares to p2pool.observer
          </h2>
          <p className="text-zinc-300 text-sm mb-3">
            <a href="https://mini.p2pool.observer" target="_blank" rel="noopener noreferrer" className="text-zinc-200 hover:text-zinc-100 underline">
              mini.p2pool.observer
            </a>{' '}
            is an excellent tool. It indexes the full P2Pool sidechain and lets any miner
            look up their shares, payments, and stats &mdash; regardless of which node
            they&apos;re mining through. If you just want to check your mining stats,
            p2pool.observer is probably what you need.
          </p>
          <p className="text-zinc-400 text-sm mb-3">
            SideWatch is different in scope. It only tracks miners connected to its own node,
            which means the stats you see here are limited to this node&apos;s workers. Where
            SideWatch adds value is in the managed infrastructure (no sync, no maintenance),
            the tax export pipeline (fiat-converted CSV), and the subscription system for
            extended data retention.
          </p>
          <p className="text-zinc-400 text-sm">
            We may integrate p2pool.observer data as a fallback for global miner lookups in
            the future, but for now, the two tools serve different purposes.
          </p>
        </div>

        <div className="stat-card stat-card-yellow">
          <h2 className="text-lg font-semibold text-zinc-100 mb-3">
            Why this project exists
          </h2>
          <p className="text-zinc-300 text-sm mb-3">
            SideWatch started as a solo project to solve a real problem &mdash; good
            observability for P2Pool mining without running heavy infrastructure. But the
            project became as much about the learning as the product.
          </p>
          <p className="text-zinc-400 text-sm mb-3">
            The author is a penetration tester who needed hands-on experience with cloud
            infrastructure, server management, and full-stack development. Building and
            operating SideWatch end-to-end forced real skill development across Go backend
            engineering, PostgreSQL, Redis, Docker, CI/CD with DevSecOps, Prometheus/Grafana
            observability, Next.js frontend, and cryptocurrency protocol internals.
          </p>
          <p className="text-zinc-400 text-sm">
            The entire codebase was built with{' '}
            <a href="https://docs.anthropic.com/en/docs/claude-code" target="_blank" rel="noopener noreferrer" className="text-zinc-300 hover:text-zinc-100 underline">
              Claude Code
            </a>{' '}
            as a development partner. The source is available under{' '}
            <a href="https://github.com/Acquiredl/Delted-SideWatch" target="_blank" rel="noopener noreferrer" className="text-zinc-300 hover:text-zinc-100 underline">
              AGPL-3.0 on GitHub
            </a>{' '}
            &mdash; both because transparency builds trust with privacy-conscious miners,
            and because it documents every decision made along the way.
          </p>
        </div>
      </div>
    </div>
  )
}
