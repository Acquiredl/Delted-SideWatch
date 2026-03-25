import { render, screen } from '@testing-library/react'
import MinerPage from '@/app/miner/page'

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

// Mock child components that have their own SWR/WS dependencies
jest.mock('@/components/PrivacyNotice', () => {
  return function MockPrivacyNotice() {
    return <div data-testid="privacy-notice">Privacy Notice</div>
  }
})

jest.mock('@/components/HashrateChart', () => {
  return function MockHashrateChart() {
    return <div data-testid="hashrate-chart">Hashrate Chart</div>
  }
})

jest.mock('@/components/PaymentsTable', () => {
  return function MockPaymentsTable() {
    return <div data-testid="payments-table">Payments Table</div>
  }
})

describe('MinerPage', () => {
  it('renders the form with input and button', () => {
    render(<MinerPage />)

    expect(
      screen.getByPlaceholderText(/Enter your Monero wallet address/)
    ).toBeInTheDocument()
    expect(
      screen.getByRole('button', { name: /Look Up/ })
    ).toBeInTheDocument()
  })

  it('renders the page heading', () => {
    render(<MinerPage />)

    expect(screen.getByText('Miner Dashboard')).toBeInTheDocument()
  })

  it('shows prompt text when no address is active', () => {
    render(<MinerPage />)

    expect(
      screen.getByText(/Enter your wallet address above/)
    ).toBeInTheDocument()
  })
})
