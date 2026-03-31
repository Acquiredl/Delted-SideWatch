import { render, screen } from '@testing-library/react'
import SidechainTable from '@/components/SidechainTable'
import type { SidechainShare } from '@/lib/api'

describe('SidechainTable', () => {
  it('renders skeleton elements in loading state', () => {
    const { container } = render(
      <SidechainTable shares={[]} isLoading={true} />
    )

    const skeletons = container.querySelectorAll('.animate-pulse .bg-zinc-800')
    expect(skeletons.length).toBe(5)
  })

  it('shows "No sidechain shares found" when empty', () => {
    render(<SidechainTable shares={[]} isLoading={false} />)

    expect(screen.getByText('No sidechain shares found')).toBeInTheDocument()
  })

  it('renders truncated addresses', () => {
    const shares: SidechainShare[] = [
      {
        miner_address:
          '4ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuv',
        worker_name: 'rig1',
        sidechain: 'mini',
        sidechain_height: 500000,
        difficulty: 250000,
        is_uncle: false,
        software_id: null,
        software_version: null,
        created_at: new Date().toISOString(),
      },
    ]

    render(<SidechainTable shares={shares} />)

    // Should be truncated: first 8 + ... + last 8
    const addr = '4ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuv'
    const expected = addr.slice(0, 8) + '...' + addr.slice(-8)
    expect(screen.getByText(expected)).toBeInTheDocument()
  })

  it('renders formatted difficulty', () => {
    const shares: SidechainShare[] = [
      {
        miner_address: '4ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuv',
        worker_name: 'rig1',
        sidechain: 'mini',
        sidechain_height: 500000,
        difficulty: 5000000,
        is_uncle: false,
        software_id: null,
        software_version: null,
        created_at: new Date().toISOString(),
      },
    ]

    render(<SidechainTable shares={shares} />)

    expect(screen.getByText('5.00 M')).toBeInTheDocument()
  })

  it('shows "--" for null worker name', () => {
    const shares: SidechainShare[] = [
      {
        miner_address: '4ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuv',
        worker_name: '',
        sidechain: 'mini',
        sidechain_height: 500000,
        difficulty: 1000,
        is_uncle: false,
        software_id: null,
        software_version: null,
        created_at: new Date().toISOString(),
      },
    ]

    render(<SidechainTable shares={shares} />)

    expect(screen.getByText('--')).toBeInTheDocument()
  })
})
