import { render, screen } from '@testing-library/react'
import PaymentsTable from '@/components/PaymentsTable'
import type { MinerPayment } from '@/lib/api'

describe('PaymentsTable', () => {
  it('renders skeleton elements in loading state', () => {
    const { container } = render(
      <PaymentsTable payments={[]} isLoading={true} />
    )

    const skeletons = container.querySelectorAll('.animate-pulse .bg-zinc-800')
    expect(skeletons.length).toBe(5)
  })

  it('shows "No payments found" when empty', () => {
    render(<PaymentsTable payments={[]} isLoading={false} />)

    expect(screen.getByText('No payments found')).toBeInTheDocument()
  })

  it('renders formatted XMR amounts', () => {
    const payments: MinerPayment[] = [
      {
        amount: 600000000000, // 0.6 XMR
        main_height: 3000000,
        xmr_usd_price: 150,
        xmr_cad_price: 200,
        paid_at: '2025-06-15T10:30:00Z',
      },
    ]

    render(<PaymentsTable payments={payments} />)

    expect(screen.getByText(/0\.6000 XMR/)).toBeInTheDocument()
  })

  it('shows "--" for null fiat prices', () => {
    const payments: MinerPayment[] = [
      {
        amount: 1e12,
        main_height: 3000000,
        xmr_usd_price: 0,
        xmr_cad_price: 0,
        paid_at: '2025-06-15T10:30:00Z',
      },
    ]

    render(<PaymentsTable payments={payments} />)

    const dashes = screen.getAllByText('--')
    expect(dashes.length).toBe(2)
  })

  it('calculates and displays fiat value when price is present', () => {
    const payments: MinerPayment[] = [
      {
        amount: 1e12, // 1 XMR
        main_height: 3000000,
        xmr_usd_price: 150.0,
        xmr_cad_price: 200.0,
        paid_at: '2025-06-15T10:30:00Z',
      },
    ]

    render(<PaymentsTable payments={payments} />)

    // 1 XMR * $150 = $150.00
    expect(screen.getByText('$150.00')).toBeInTheDocument()
    // 1 XMR * C$200 = C$200.00
    expect(screen.getByText('C$200.00')).toBeInTheDocument()
  })
})
