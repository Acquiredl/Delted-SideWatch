import { render, screen } from '@testing-library/react'
import '@testing-library/jest-dom'
import SubscribePage from '../page'

// Mock next/navigation
jest.mock('next/navigation', () => ({
  useRouter: () => ({ push: jest.fn() }),
  usePathname: () => '/subscribe',
  useSearchParams: () => new URLSearchParams(),
}))

// Mock SWR — SubscribePage calls useSWR for status, address, and payments.
jest.mock('swr', () => ({
  __esModule: true,
  default: () => ({ data: undefined, error: undefined, isLoading: false }),
}))

describe('SubscribePage', () => {
  it('renders the page heading', () => {
    render(<SubscribePage />)

    expect(screen.getByRole('heading', { name: /Support.*SideWatch/ })).toBeInTheDocument()
  })

  it('renders address input and payment button', () => {
    render(<SubscribePage />)

    expect(screen.getByPlaceholderText(/Enter your Monero wallet address/)).toBeInTheDocument()
    expect(screen.getByText('Get Payment Address')).toBeInTheDocument()
  })

  it('shows payment flow steps when no address is entered', () => {
    render(<SubscribePage />)

    expect(screen.getByText('Pay with XMR')).toBeInTheDocument()
    expect(screen.getByText(/Get your payment address/)).toBeInTheDocument()
  })

  it('shows tiered pricing info', () => {
    render(<SubscribePage />)

    expect(screen.getByText(/\$1\+ Supporter/)).toBeInTheDocument()
    expect(screen.getByText(/\$5\+ Champion/)).toBeInTheDocument()
  })
})
