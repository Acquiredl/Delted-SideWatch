import { render, screen } from '@testing-library/react'
import AdminPage from '@/app/admin/page'

// Mock localStorage
const localStorageMock = (() => {
  let store: Record<string, string> = {}
  return {
    getItem: jest.fn((key: string) => store[key] ?? null),
    setItem: jest.fn((key: string, value: string) => {
      store[key] = value
    }),
    removeItem: jest.fn((key: string) => {
      delete store[key]
    }),
    clear: jest.fn(() => {
      store = {}
    }),
  }
})()

Object.defineProperty(window, 'localStorage', {
  value: localStorageMock,
})

// Mock fetch
const originalFetch = global.fetch

describe('AdminPage', () => {
  beforeEach(() => {
    localStorageMock.clear()
    localStorageMock.getItem.mockReturnValue(null)
    global.fetch = jest.fn()
  })

  afterEach(() => {
    global.fetch = originalFetch
  })

  it('renders login form when no token is stored', () => {
    render(<AdminPage />)

    expect(screen.getByText('Admin Panel')).toBeInTheDocument()
    expect(
      screen.getByText(/Enter your admin JWT token/)
    ).toBeInTheDocument()
    expect(screen.getByLabelText('JWT Token')).toBeInTheDocument()
    expect(
      screen.getByRole('button', { name: /Authenticate/ })
    ).toBeInTheDocument()
  })

  it('renders the token input as a password field', () => {
    render(<AdminPage />)

    const tokenInput = screen.getByLabelText('JWT Token')
    expect(tokenInput).toHaveAttribute('type', 'password')
  })

  it('renders the placeholder text for token input', () => {
    render(<AdminPage />)

    expect(
      screen.getByPlaceholderText('Paste your admin token here')
    ).toBeInTheDocument()
  })
})
