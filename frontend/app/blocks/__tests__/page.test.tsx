import { render, screen } from '@testing-library/react'
import BlocksPage from '@/app/blocks/page'

// Track the URL passed to SWR so we can verify offset changes
const swrUrls: (string | null)[] = []

jest.mock('swr', () => ({
  __esModule: true,
  default: (url: string | null) => {
    swrUrls.push(url)
    // Return empty blocks and null pool stats
    if (url && url.includes('/api/blocks')) {
      return {
        data: [],
        error: undefined,
        isLoading: false,
        isValidating: false,
        mutate: jest.fn(),
      }
    }
    return {
      data: undefined,
      error: undefined,
      isLoading: false,
      isValidating: false,
      mutate: jest.fn(),
    }
  },
}))

jest.mock('@/components/BlocksTable', () => {
  return function MockBlocksTable() {
    return <div data-testid="blocks-table">Blocks Table</div>
  }
})

describe('BlocksPage', () => {
  beforeEach(() => {
    swrUrls.length = 0
  })

  it('renders the page heading', () => {
    render(<BlocksPage />)

    expect(screen.getByText('Blocks Found')).toBeInTheDocument()
  })

  it('renders the blocks table component', () => {
    render(<BlocksPage />)

    expect(screen.getByTestId('blocks-table')).toBeInTheDocument()
  })

  it('hides pagination when no blocks are found', () => {
    render(<BlocksPage />)

    expect(screen.queryByText('Previous')).not.toBeInTheDocument()
    expect(screen.queryByText('Next')).not.toBeInTheDocument()
  })

  it('fetches blocks with offset 0 initially', () => {
    render(<BlocksPage />)

    const blocksUrl = swrUrls.find(u => u && u.includes('/api/blocks'))
    expect(blocksUrl).toContain('offset=0')
  })
})
