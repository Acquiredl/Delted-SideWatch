import { render, screen } from '@testing-library/react'
import '@testing-library/jest-dom'
import SubscriptionPayment from '@/components/SubscriptionPayment'
import type { PaymentAddress, SubPayment } from '@/lib/api'

describe('SubscriptionPayment', () => {
  const mockAddress: PaymentAddress = {
    miner_address: '4ABC...',
    subaddress: '8DEF1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef',
    suggested_amount_xmr: '0.031250',
    amount_usd: '$5.00',
  }

  it('renders payment subaddress', () => {
    render(<SubscriptionPayment paymentAddress={mockAddress} payments={[]} />)

    expect(screen.getByText(mockAddress.subaddress)).toBeInTheDocument()
    expect(screen.getByText(/0\.031250 XMR/)).toBeInTheDocument()
    expect(screen.getByText(/\$5\.00/)).toBeInTheDocument()
  })

  it('renders Copy button', () => {
    render(<SubscriptionPayment paymentAddress={mockAddress} payments={[]} />)

    expect(screen.getByText('Copy')).toBeInTheDocument()
  })

  it('renders payment history table', () => {
    const payments: SubPayment[] = [
      {
        id: 1,
        miner_address: '4ABC...',
        tx_hash: 'aabbccdd11223344aabbccdd11223344aabbccdd11223344aabbccdd11223344',
        amount: 31250000000, // ~0.03125 XMR
        xmr_usd_price: 160.0,
        confirmed: true,
        main_height: 3100000,
        created_at: '2026-03-20T10:00:00Z',
      },
    ]

    render(<SubscriptionPayment paymentAddress={mockAddress} payments={payments} />)

    expect(screen.getByText('Subscription Payments')).toBeInTheDocument()
    expect(screen.getByText('Confirmed')).toBeInTheDocument()
    expect(screen.getByText(/0\.0313 XMR/)).toBeInTheDocument()
  })

  it('renders Pending badge for unconfirmed payments', () => {
    const payments: SubPayment[] = [
      {
        id: 2,
        miner_address: '4ABC...',
        tx_hash: '1122334455667788112233445566778811223344556677881122334455667788',
        amount: 31250000000,
        xmr_usd_price: null,
        confirmed: false,
        main_height: null,
        created_at: '2026-03-25T14:00:00Z',
      },
    ]

    render(<SubscriptionPayment paymentAddress={mockAddress} payments={payments} />)

    expect(screen.getByText('Pending')).toBeInTheDocument()
  })

  it('renders loading skeleton', () => {
    const { container } = render(
      <SubscriptionPayment paymentAddress={undefined} payments={[]} isLoading={true} />
    )
    expect(container.querySelector('.animate-pulse')).toBeTruthy()
  })
})
