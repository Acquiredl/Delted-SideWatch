import { render, screen } from '@testing-library/react'
import LiveStats from '@/components/LiveStats'

// Mock the useWebSocket hook
const mockWsReturn = {
  data: null as unknown,
  isConnected: false,
  error: null as string | null,
}

jest.mock('@/lib/useWebSocket', () => ({
  useWebSocket: () => mockWsReturn,
}))

// Mock SWR
const mockSwrReturn = {
  data: undefined as unknown,
  error: undefined as Error | undefined,
  isLoading: true,
  isValidating: false,
  mutate: jest.fn(),
}

jest.mock('swr', () => ({
  __esModule: true,
  default: () => mockSwrReturn,
}))

describe('LiveStats', () => {
  beforeEach(() => {
    mockWsReturn.data = null
    mockWsReturn.isConnected = false
    mockWsReturn.error = null
    mockSwrReturn.data = undefined
    mockSwrReturn.error = undefined
    mockSwrReturn.isLoading = true
  })

  it('renders loading skeleton when no data available', () => {
    const { container } = render(<LiveStats />)

    const skeletons = container.querySelectorAll('.animate-pulse')
    expect(skeletons.length).toBeGreaterThan(0)
  })

  it('renders stat cards when connected with data', () => {
    mockWsReturn.isConnected = true
    mockWsReturn.data = {
      total_miners: 150,
      total_hashrate: 5000000,
      blocks_found: 42,
      last_block_found_at: '2025-01-01T12:00:00Z',
      total_paid: 1000000000000,
      sidechain: 'mini',
      sidechain_height: 12345678,
      sidechain_difficulty: 500000000,
    }
    mockSwrReturn.isLoading = false

    render(<LiveStats />)

    expect(screen.getByText('Pool Hashrate')).toBeInTheDocument()
    expect(screen.getByText('Active Miners')).toBeInTheDocument()
    expect(screen.getByText('Sidechain Height')).toBeInTheDocument()
    expect(screen.getByText('Blocks Found')).toBeInTheDocument()

    // Check formatted values
    expect(screen.getByText('5.00 MH/s')).toBeInTheDocument()
    expect(screen.getByText('150')).toBeInTheDocument()
    expect(screen.getByText('42')).toBeInTheDocument()
    expect(screen.getByText('12,345,678')).toBeInTheDocument()
  })

  it('shows Live indicator when WebSocket is connected', () => {
    mockWsReturn.isConnected = true
    mockWsReturn.data = {
      total_miners: 10,
      total_hashrate: 1000,
      blocks_found: 1,
      last_block_found_at: '2025-01-01T12:00:00Z',
      total_paid: 0,
      sidechain: 'mini',
      sidechain_height: 100,
      sidechain_difficulty: 100,
    }
    mockSwrReturn.isLoading = false

    render(<LiveStats />)

    expect(screen.getByText('Live')).toBeInTheDocument()
  })

  it('shows Polling indicator when WebSocket is not connected', () => {
    mockWsReturn.isConnected = false
    mockSwrReturn.isLoading = false
    mockSwrReturn.data = {
      total_miners: 10,
      total_hashrate: 1000,
      blocks_found: 1,
      last_block_found_at: '2025-01-01T12:00:00Z',
      total_paid: 0,
      sidechain: 'mini',
      sidechain_height: 100,
      sidechain_difficulty: 100,
    }

    render(<LiveStats />)

    expect(screen.getByText('Polling')).toBeInTheDocument()
  })
})
