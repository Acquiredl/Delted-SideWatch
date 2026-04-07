export default function PrivacyNotice() {
  return (
    <div className="bg-zinc-900/80 border border-zinc-700 rounded-lg p-4 mb-6 space-y-2">
      <p className="text-zinc-300 text-sm">
        <strong className="text-cube-yellow">Privacy:</strong> Wallet addresses are public on
        P2Pool&apos;s sidechain. The P2Pool project recommends creating a <strong>dedicated mining
        wallet</strong> rather than reusing your main wallet. Coinbase transactions are also publicly
        visible on the Monero blockchain. This dashboard displays publicly available data only.
      </p>
      <p className="text-zinc-400 text-xs">
        SideWatch does <strong>not</strong> store IP addresses, connection logs, or any data that
        links your identity to your wallet. For additional privacy, use a VPN when accessing the
        dashboard. Tor support is{' '}
        <a href="/privacy" className="text-cube-blue hover:underline">planned for a future release</a>.
      </p>
    </div>
  )
}
