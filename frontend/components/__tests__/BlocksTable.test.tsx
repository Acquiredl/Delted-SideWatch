import { render, screen } from '@testing-library/react'
import BlocksTable from '@/components/BlocksTable'
import type { FoundBlock } from '@/lib/api'

describe('BlocksTable', () => {
  it('renders skeleton elements in loading state', () => {
    const { container } = render(<BlocksTable blocks={[]} isLoading={true} />)

    const skeletons = container.querySelectorAll('.animate-pulse .bg-zinc-800')
    expect(skeletons.length).toBe(5)
  })

  it('shows "No blocks found yet" when empty', () => {
    render(<BlocksTable blocks={[]} isLoading={false} />)

    expect(screen.getByText('No blocks found yet')).toBeInTheDocument()
  })

  it('renders rows with formatted values when data is provided', () => {
    const blocks: FoundBlock[] = [
      {
        main_height: 3000000,
        main_hash: 'a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2',
        sidechain_height: 100000,
        coinbase_reward: 600000000000, // 0.6 XMR
        effort: 0.85,
        coinbase_private_key: null,
        found_at: new Date().toISOString(),
      },
    ]

    render(<BlocksTable blocks={blocks} />)

    // Height should be rendered
    expect(screen.getByText('3,000,000')).toBeInTheDocument()
    // Reward should be formatted
    expect(screen.getByText(/0\.6000 XMR/)).toBeInTheDocument()
    // Effort should be formatted
    expect(screen.getByText('85.0%')).toBeInTheDocument()
  })

  it('colors effort red when >100% and green when <100%', () => {
    const blocks: FoundBlock[] = [
      {
        main_height: 1,
        main_hash: 'aabbccdd11223344aabbccdd11223344aabbccdd11223344aabbccdd11223344',
        sidechain_height: 1,
        coinbase_reward: 1e12,
        effort: 1.5,
        coinbase_private_key: null,
        found_at: new Date().toISOString(),
      },
      {
        main_height: 2,
        main_hash: '11223344aabbccdd11223344aabbccdd11223344aabbccdd11223344aabbccdd',
        sidechain_height: 2,
        coinbase_reward: 1e12,
        effort: 0.5,
        coinbase_private_key: null,
        found_at: new Date().toISOString(),
      },
    ]

    render(<BlocksTable blocks={blocks} />)

    const effortCells = screen.getAllByText(/%$/)
    // effort > 1 should have red text
    expect(effortCells[0].className).toContain('text-red-400')
    // effort < 1 should have green text
    expect(effortCells[1].className).toContain('text-green-400')
  })
})
