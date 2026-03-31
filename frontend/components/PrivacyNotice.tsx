export default function PrivacyNotice() {
  return (
    <div className="bg-amber-900/30 border border-amber-700 rounded-lg p-4 mb-6 space-y-2">
      <p className="text-amber-200 text-sm">
        <strong>Privacy &amp; Transparency:</strong> Coinbase transactions are publicly visible on the Monero blockchain.
        Mining payouts from P2Pool can be linked to your wallet address. This dashboard displays publicly
        available blockchain data only.
      </p>
      <p className="text-amber-200/80 text-xs">
        <strong>What SideWatch stores:</strong> Share timestamps, hashrate history, payment amounts, and
        worker names derived from the P2Pool sidechain. We publish coinbase private keys for every found
        block so anyone can independently verify that payouts match the PPLNS window.
      </p>
      <p className="text-amber-200/80 text-xs">
        <strong>What SideWatch does NOT store:</strong> IP addresses, connection logs, or any data that
        links your identity to your wallet. If you want additional privacy, connect to the P2Pool node
        through a VPN so your IP cannot be correlated with your mining address at the network level.
      </p>
    </div>
  )
}
