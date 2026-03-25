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

  it('shows "No active workers" when empty', () => {
    render(<WorkersTable workers={[]} isLoading={false} />)

    expect(screen.getByText('No active workers')).toBeInTheDocument()
  })

  it('renders worker data correctly', () => {
    const workers = [
      {
        worker_name: 'rig1',
        shares: 1500,
        last_share_at: new Date().toISOString(),
      },
    ]

    render(<WorkersTable workers={workers} />)

    expect(screen.getByText('rig1')).toBeInTheDocument()
    expect(screen.getByText('1,500')).toBeInTheDocument()
  })

  it('shows "default" for empty worker name', () => {
    const workers = [
      {
        worker_name: '',
        shares: 100,
        last_share_at: new Date().toISOString(),
      },
    ]

    render(<WorkersTable workers={workers} />)

    expect(screen.getByText('default')).toBeInTheDocument()
  })
})
