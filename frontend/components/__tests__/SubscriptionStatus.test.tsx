import { render, screen } from '@testing-library/react'
import '@testing-library/jest-dom'
import SubscriptionStatus from '@/components/SubscriptionStatus'
import type { SubscriptionStatus as SubStatus } from '@/lib/api'

describe('SubscriptionStatus', () => {
  it('renders Free badge for free tier', () => {
    const status: SubStatus = {
      miner_address: '4...',
      tier: 'free',
      active: false,
      expires_at: null,
      grace_until: null,
      has_api_key: false,
    }

    render(<SubscriptionStatus status={status} />)

    expect(screen.getByText('Free')).toBeInTheDocument()
    expect(screen.getByText(/30 days hashrate history/)).toBeInTheDocument()
  })

  it('renders Supporter badge with expiry for active supporter tier', () => {
    const futureDate = new Date(Date.now() + 30 * 86400000).toISOString()
    const status: SubStatus = {
      miner_address: '4...',
      tier: 'supporter',
      active: true,
      expires_at: futureDate,
      grace_until: null,
      has_api_key: true,
    }

    render(<SubscriptionStatus status={status} />)

    expect(screen.getByText('Supporter')).toBeInTheDocument()
    expect(screen.getByText('API key active')).toBeInTheDocument()
    expect(screen.getByText(/Expires:/)).toBeInTheDocument()
  })

  it('renders Champion badge for active champion tier', () => {
    const futureDate = new Date(Date.now() + 30 * 86400000).toISOString()
    const status: SubStatus = {
      miner_address: '4...',
      tier: 'champion',
      active: true,
      expires_at: futureDate,
      grace_until: null,
      has_api_key: false,
    }

    render(<SubscriptionStatus status={status} />)

    expect(screen.getByText('Champion')).toBeInTheDocument()
    expect(screen.getByText(/Expires:/)).toBeInTheDocument()
  })

  it('renders Grace Period badge when expired but in grace', () => {
    const pastDate = new Date(Date.now() - 86400000).toISOString()
    const futureGrace = new Date(Date.now() + 86400000).toISOString()
    const status: SubStatus = {
      miner_address: '4...',
      tier: 'supporter',
      active: false,
      expires_at: pastDate,
      grace_until: futureGrace,
      has_api_key: false,
    }

    render(<SubscriptionStatus status={status} />)

    expect(screen.getByText('Grace Period')).toBeInTheDocument()
    expect(screen.getByText(/Grace period ends:/)).toBeInTheDocument()
  })

  it('renders loading skeleton', () => {
    const status: SubStatus = {
      miner_address: '4...',
      tier: 'free',
      active: false,
      expires_at: null,
      grace_until: null,
      has_api_key: false,
    }

    const { container } = render(<SubscriptionStatus status={status} isLoading={true} />)
    expect(container.querySelector('.animate-pulse')).toBeTruthy()
  })
})
