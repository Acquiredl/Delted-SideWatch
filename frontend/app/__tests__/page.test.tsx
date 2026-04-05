import { render, screen } from '@testing-library/react'
import HomePage from '@/app/page'

// Mock SWR
jest.mock('swr', () => ({
  __esModule: true,
  default: () => ({
    data: undefined,
    error: undefined,
    isLoading: false,
    isValidating: false,
    mutate: jest.fn(),
  }),
}))

// Mock the LiveStats component since it has its own complex dependencies
jest.mock('@/components/LiveStats', () => {
  return function MockLiveStats() {
    return <div data-testid="live-stats">LiveStats Component</div>
  }
})

jest.mock('@/components/FundProgress', () => {
  return function MockFundProgress() {
    return <div data-testid="fund-progress">Fund Progress</div>
  }
})

describe('HomePage', () => {
  it('renders the heading', () => {
    render(<HomePage />)

    expect(
      screen.getByRole('heading', { name: /Welcome to.*SideWatch/ })
    ).toBeInTheDocument()
  })

  it('renders the description text', () => {
    render(<HomePage />)

    expect(
      screen.getByText(/observability dashboard for P2Pool/)
    ).toBeInTheDocument()
  })

  it('renders the LiveStats component', () => {
    render(<HomePage />)

    expect(screen.getByTestId('live-stats')).toBeInTheDocument()
  })
})
