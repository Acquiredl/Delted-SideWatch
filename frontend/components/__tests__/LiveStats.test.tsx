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

// Mock SWR — LiveStats calls useSWR twice (pool stats, node status).
// Return pool stats for /api/pool/stats and null for /api/nodes/status.
const mockPoolSwrReturn = {
  data: undefined as unknown,
  error: undefined as Error | undefined,
  isLoading: true,
  isValidating: false,
  mutate: jest.fn(),
}

const mockNodeSwrReturn = {
  data: undefined as unknown,
  error: undefined as Error | undefined,
  isLoading: false,
  isValidating: false,
  mutate: jest.fn(),
}

jest.mock('swr', () => ({
  __esModule: true,
  default: (key: string | null) => {
    if (key && key.includes('/api/nodes/')) return mockNodeSwrReturn
    return mockPoolSwrReturn
  },
}))

describe('LiveStats', () => {
  beforeEach(() => {
    mockWsReturn.data = null
    mockWsReturn.isConnected = false
    mockWsReturn.error = null
    mockPoolSwrReturn.data = undefined
    mockPoolSwrReturn.error = undefined
    mockPoolSwrReturn.isLoading = true
    mockNodeSwrReturn.data = undefined
  })

  it('renders loading skeleton when no data available', () => {
    const { container } = render(<LiveStats />)

    const skeletons = container.querySelectorAll('.animate-pulse')
    expect(skeletons.length).toBeGreaterThan(0)
  })

  it('renders 4 stat cards when connected with data', () => {
    mockWsReturn.isConnected = true
    mockWsReturn.data = {
      total_miners: 150,
      total_hashrate: 5000000,
      blocks_found: 42,
      last_block_found_at: '2025-01-01T12:00:00Z',
      total_paid: 1000000000000,
      sidechain: 'mini',
    }
    mockPoolSwrReturn.isLoading = false

    render(<LiveStats />)

    expect(screen.getByText('Total Hashrate')).toBeInTheDocument()
    expect(screen.getByText('Active Miners')).toBeInTheDocument()
    expect(screen.getByText('Blocks Found')).toBeInTheDocument()
    expect(screen.getByText('Total Paid')).toBeInTheDocument()

    // Check formatted values
    expect(screen.getByText('5.00 MH/s')).toBeInTheDocument()
    expect(screen.getByText('150')).toBeInTheDocument()
    expect(screen.getByText('42')).toBeInTheDocument()
    expect(screen.getByText('1.0000 XMR')).toBeInTheDocument()
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
    }
    mockPoolSwrReturn.isLoading = false

    render(<LiveStats />)

    expect(screen.getByText('Live')).toBeInTheDocument()
  })

  it('shows Polling indicator when WebSocket is not connected', () => {
    mockWsReturn.isConnected = false
    mockPoolSwrReturn.isLoading = false
    mockPoolSwrReturn.data = {
      total_miners: 10,
      total_hashrate: 1000,
      blocks_found: 1,
      last_block_found_at: '2025-01-01T12:00:00Z',
      total_paid: 0,
      sidechain: 'mini',
    }

    render(<LiveStats />)

    expect(screen.getByText('Polling')).toBeInTheDocument()
  })

  it('renders node health indicators when node data available', () => {
    mockWsReturn.isConnected = true
    mockWsReturn.data = {
      total_miners: 10,
      total_hashrate: 1000,
      blocks_found: 1,
      last_block_found_at: '2025-01-01T12:00:00Z',
      total_paid: 0,
      sidechain: 'mini',
    }
    mockPoolSwrReturn.isLoading = false
    mockNodeSwrReturn.data = {
      nodes: [
        { name: 'SideWatch Mini', sidechain: 'mini', status: 'healthy', miners: 10, hashrate: 5000, peers: 8, last_health_at: null },
      ],
    }

    render(<LiveStats />)

    expect(screen.getByText('SideWatch Mini')).toBeInTheDocument()
  })
})
