import { render, screen } from '@testing-library/react'
import BlocksPage from '@/app/blocks/page'

// Track the URL passed to SWR so we can verify offset changes
let swrUrl: string | null = null

jest.mock('swr', () => ({
  __esModule: true,
  default: (url: string | null) => {
    swrUrl = url
    return {
      data: [],
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
    swrUrl = null
  })

  it('renders the page heading', () => {
    render(<BlocksPage />)

    expect(screen.getByText('Blocks Found')).toBeInTheDocument()
  })

  it('has Previous button disabled at offset 0', () => {
    render(<BlocksPage />)

    const prevButton = screen.getByText('Previous')
    expect(prevButton).toBeDisabled()
  })

  it('renders the pagination controls', () => {
    render(<BlocksPage />)

    expect(screen.getByText('Previous')).toBeInTheDocument()
    expect(screen.getByText('Next')).toBeInTheDocument()
  })

  it('shows correct range text', () => {
    render(<BlocksPage />)

    expect(screen.getByText(/Showing 1 -/)).toBeInTheDocument()
  })

  it('fetches with offset 0 initially', () => {
    render(<BlocksPage />)

    expect(swrUrl).toContain('offset=0')
  })
})
