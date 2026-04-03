import { render, screen } from '@testing-library/react'
import PrivacyNotice from '@/components/PrivacyNotice'

describe('PrivacyNotice', () => {
  it('renders the privacy notice warning text', () => {
    render(<PrivacyNotice />)

    expect(screen.getByText(/Privacy:/)).toBeInTheDocument()
    expect(
      screen.getByText(/Coinbase transactions are publicly visible/)
    ).toBeInTheDocument()
  })
})
