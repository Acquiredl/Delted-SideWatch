export default function PrivacyNotice() {
  return (
    <div className="bg-amber-900/30 border border-amber-700 rounded-lg p-4 mb-6">
      <p className="text-amber-200 text-sm">
        <strong>Privacy Notice:</strong> Coinbase transactions are publicly visible on the Monero blockchain.
        Mining payouts from P2Pool can be linked to your wallet address. This dashboard displays publicly
        available blockchain data only.
      </p>
    </div>
  )
}
