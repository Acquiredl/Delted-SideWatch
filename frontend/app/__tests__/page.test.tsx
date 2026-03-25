import { render, screen } from '@testing-library/react'
import HomePage from '@/app/page'

// Mock the LiveStats component since it has its own complex dependencies
jest.mock('@/components/LiveStats', () => {
  return function MockLiveStats() {
    return <div data-testid="live-stats">LiveStats Component</div>
  }
})

describe('HomePage', () => {
  it('renders the heading', () => {
    render(<HomePage />)

    expect(
      screen.getByText('P2Pool Mini Dashboard')
    ).toBeInTheDocument()
  })

  it('renders the description text', () => {
    render(<HomePage />)

    expect(
      screen.getByText(/Decentralized Monero mining/)
    ).toBeInTheDocument()
  })

  it('renders the LiveStats component', () => {
    render(<HomePage />)

    expect(screen.getByTestId('live-stats')).toBeInTheDocument()
  })
})
