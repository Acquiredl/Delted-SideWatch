import { render, screen } from '@testing-library/react'
import WorkersTable from '@/components/WorkersTable'

describe('WorkersTable', () => {
  it('renders skeleton elements in loading state', () => {
    const { container } = render(
      <WorkersTable workers={[]} isLoading={true} />
    )

    const skeletons = container.querySelectorAll('.animate-pulse .bg-zinc-800')
    expect(skeletons.length).toBe(3)
  })

  it('shows empty message when no workers', () => {
    render(<WorkersTable workers={[]} isLoading={false} />)

    expect(screen.getByText('No active workers connected to this node')).toBeInTheDocument()
  })

  it('renders worker data correctly', () => {
    const workers = [
      {
        miner_address: '4ABC123def456789abcdef012345678',
        current_hashrate: 15000,
        last_seen: new Date().toISOString(),
      },
    ]

    render(<WorkersTable workers={workers} />)

    expect(screen.getByText('4ABC123d...12345678')).toBeInTheDocument()
    expect(screen.getByText('15.00 KH/s')).toBeInTheDocument()
  })
})
