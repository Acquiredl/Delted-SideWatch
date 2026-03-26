import { render, screen } from '@testing-library/react'
import '@testing-library/jest-dom'
import SubscribePage from '../page'

// Mock next/navigation
jest.mock('next/navigation', () => ({
  useRouter: () => ({ push: jest.fn() }),
  usePathname: () => '/subscribe',
}))

// Mock SWR to avoid actual API calls
jest.mock('swr', () => ({
  __esModule: true,
  default: () => ({ data: undefined, error: undefined, isLoading: false }),
}))

describe('SubscribePage', () => {
  it('renders the page heading', () => {
    render(<SubscribePage />)

    expect(screen.getByText('Subscribe')).toBeInTheDocument()
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

  it('shows pricing info', () => {
    render(<SubscribePage />)

    expect(screen.getByText(/\$5\/month in XMR/)).toBeInTheDocument()
  })
})
