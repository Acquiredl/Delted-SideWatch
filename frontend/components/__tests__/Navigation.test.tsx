import { render, screen } from '@testing-library/react'
import Navigation from '@/components/Navigation'

jest.mock('next/navigation', () => ({
  usePathname: () => '/',
}))

describe('Navigation', () => {
  it('renders nav links', () => {
    render(<Navigation />)

    expect(screen.getByText('Home')).toBeInTheDocument()
    expect(screen.getByText('Miner')).toBeInTheDocument()
    expect(screen.getByText('Blocks')).toBeInTheDocument()
    expect(screen.getByText('Sidechain')).toBeInTheDocument()
    expect(screen.getByText('Fund')).toBeInTheDocument()
    expect(screen.getByText('Connect')).toBeInTheDocument()
    expect(screen.getByText('Subscribe')).toBeInTheDocument()
  })

  it('renders the SideWatch brand', () => {
    render(<Navigation />)

    expect(screen.getByText('SideWatch')).toBeInTheDocument()
  })

  it('highlights the active link', () => {
    render(<Navigation />)

    const homeLink = screen.getByText('Home')
    expect(homeLink.className).toContain('text-xmr-orange')
  })
})
