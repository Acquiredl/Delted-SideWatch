import { render, screen } from '@testing-library/react'
import HashrateChart from '@/components/HashrateChart'
import type { HashratePoint } from '@/lib/api'

// Mock Recharts to avoid rendering issues in jsdom
jest.mock('recharts', () => {
  const OriginalModule = jest.requireActual('recharts')
  return {
    ...OriginalModule,
    ResponsiveContainer: ({ children }: { children: React.ReactNode }) => (
      <div data-testid="responsive-container">{children}</div>
    ),
  }
})

describe('HashrateChart', () => {
  it('shows message when data is empty', () => {
    render(<HashrateChart data={[]} />)

    expect(
      screen.getByText('No hashrate data available')
    ).toBeInTheDocument()
  })

  it('renders without crash when data is provided', () => {
    const data: HashratePoint[] = [
      { hashrate: 50000, bucket_time: '2025-01-01T12:00:00Z' },
      { hashrate: 60000, bucket_time: '2025-01-01T12:15:00Z' },
      { hashrate: 55000, bucket_time: '2025-01-01T12:30:00Z' },
    ]

    const { container } = render(<HashrateChart data={data} />)

    // Should render the chart container
    expect(screen.getByText('Hashrate (24h)')).toBeInTheDocument()
    expect(container.querySelector('[data-testid="responsive-container"]')).toBeInTheDocument()
  })
})
