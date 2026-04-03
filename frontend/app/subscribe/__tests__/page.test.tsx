import { render, screen } from '@testing-library/react'
import '@testing-library/jest-dom'
import SubscribePage from '../page'

// Mock next/navigation
jest.mock('next/navigation', () => ({
  useRouter: () => ({ push: jest.fn() }),
  usePathname: () => '/subscribe',
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

  it('renders address input and look up button', () => {
    render(<SubscribePage />)

    expect(screen.getByPlaceholderText(/Enter your Monero wallet address/)).toBeInTheDocument()
    expect(screen.getByText('Look Up')).toBeInTheDocument()
  })

  it('shows prompt text when no address is entered', () => {
    render(<SubscribePage />)

    expect(screen.getByText(/Enter your wallet address above/)).toBeInTheDocument()
  })

  it('shows tiered pricing info', () => {
    render(<SubscribePage />)

    expect(screen.getByText(/\$1\+ Supporter/)).toBeInTheDocument()
    expect(screen.getByText(/\$5\+ Champion/)).toBeInTheDocument()
  })
})
