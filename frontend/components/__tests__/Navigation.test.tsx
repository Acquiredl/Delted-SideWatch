import { render, screen } from '@testing-library/react'
import Navigation from '@/components/Navigation'

jest.mock('next/navigation', () => ({
  usePathname: () => '/',
}))

describe('Navigation', () => {
  it('renders 4 nav links', () => {
    render(<Navigation />)

    expect(screen.getByText('Home')).toBeInTheDocument()
    expect(screen.getByText('Miner')).toBeInTheDocument()
    expect(screen.getByText('Blocks')).toBeInTheDocument()
    expect(screen.getByText('Sidechain')).toBeInTheDocument()
  })

  it('renders the P2Pool Mini brand link', () => {
    render(<Navigation />)

    expect(screen.getByText('P2Pool Mini')).toBeInTheDocument()
  })

  it('highlights the active link', () => {
    render(<Navigation />)

    const homeLink = screen.getByText('Home')
    expect(homeLink.className).toContain('text-xmr-orange')
  })
})
