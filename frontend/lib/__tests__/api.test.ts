import {
  formatXMR,
  formatHashrate,
  truncateAddress,
  formatRelativeTime,
  formatDate,
  formatEffort,
  formatDifficulty,
  formatUSD,
  formatCAD,
  fetcher,
} from '@/lib/api'

// --- formatXMR ---
describe('formatXMR', () => {
  it('formats 0 atomic units', () => {
    expect(formatXMR(0)).toBe('0.0000')
  })

  it('formats 1 XMR (1e12 atomic units)', () => {
    expect(formatXMR(1e12)).toBe('1.0000')
  })

  it('formats 0.5 XMR (5e11 atomic units)', () => {
    expect(formatXMR(5e11)).toBe('0.5000')
  })

  it('formats fractional amounts with 4 decimal places', () => {
    expect(formatXMR(123456789012)).toBe('0.1235')
  })
})

// --- formatHashrate ---
describe('formatHashrate', () => {
  it('formats 0 H/s', () => {
    expect(formatHashrate(0)).toBe('0 H/s')
  })

  it('formats values below 1000 as H/s', () => {
    expect(formatHashrate(500)).toBe('500 H/s')
  })

  it('formats values in KH/s range', () => {
    expect(formatHashrate(50000)).toBe('50.00 KH/s')
  })

  it('formats values in MH/s range', () => {
    expect(formatHashrate(5e6)).toBe('5.00 MH/s')
  })

  it('formats values in GH/s range', () => {
    expect(formatHashrate(1.5e9)).toBe('1.50 GH/s')
  })
})

// --- truncateAddress ---
describe('truncateAddress', () => {
  it('returns short strings unchanged', () => {
    expect(truncateAddress('abcdef')).toBe('abcdef')
  })

  it('returns 16-char strings unchanged', () => {
    expect(truncateAddress('1234567890123456')).toBe('1234567890123456')
  })

  it('truncates long addresses to first 8 + ... + last 8', () => {
    const addr = '4ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuv'
    const result = truncateAddress(addr)
    expect(result).toBe(addr.slice(0, 8) + '...' + addr.slice(-8))
    // first 8 chars + '...' + last 8 chars
    expect(result.length).toBe(8 + 3 + 8)
  })
})

// --- formatRelativeTime ---
describe('formatRelativeTime', () => {
  beforeEach(() => {
    jest.useFakeTimers()
  })

  afterEach(() => {
    jest.useRealTimers()
  })

  it('formats seconds ago', () => {
    const now = new Date('2025-01-01T12:00:30Z')
    jest.setSystemTime(now)
    expect(formatRelativeTime('2025-01-01T12:00:00Z')).toBe('30s ago')
  })

  it('formats minutes ago', () => {
    const now = new Date('2025-01-01T12:05:00Z')
    jest.setSystemTime(now)
    expect(formatRelativeTime('2025-01-01T12:00:00Z')).toBe('5m ago')
  })

  it('formats hours and minutes ago', () => {
    const now = new Date('2025-01-01T14:30:00Z')
    jest.setSystemTime(now)
    expect(formatRelativeTime('2025-01-01T12:00:00Z')).toBe('2h 30m ago')
  })

  it('formats days and hours ago', () => {
    const now = new Date('2025-01-03T14:00:00Z')
    jest.setSystemTime(now)
    expect(formatRelativeTime('2025-01-01T12:00:00Z')).toBe('2d 2h ago')
  })
})

// --- formatDate ---
describe('formatDate', () => {
  it('formats an ISO date string', () => {
    const result = formatDate('2025-06-15T10:30:00Z')
    // Verify it contains expected parts (locale-dependent)
    expect(result).toBeTruthy()
    expect(typeof result).toBe('string')
    // Should contain "Jun" and "15" and "2025"
    expect(result).toContain('Jun')
    expect(result).toContain('15')
    expect(result).toContain('2025')
  })
})

// --- formatEffort ---
describe('formatEffort', () => {
  it('formats 50% effort', () => {
    expect(formatEffort(0.5)).toBe('50.0%')
  })

  it('formats 100% effort', () => {
    expect(formatEffort(1.0)).toBe('100.0%')
  })

  it('formats 150% effort', () => {
    expect(formatEffort(1.5)).toBe('150.0%')
  })
})

// --- formatDifficulty ---
describe('formatDifficulty', () => {
  it('formats plain numbers below 1K', () => {
    expect(formatDifficulty(500)).toBe('500')
  })

  it('formats K range', () => {
    expect(formatDifficulty(5000)).toBe('5.00 K')
  })

  it('formats M range', () => {
    expect(formatDifficulty(5e6)).toBe('5.00 M')
  })

  it('formats G range', () => {
    expect(formatDifficulty(5e9)).toBe('5.00 G')
  })

  it('formats T range', () => {
    expect(formatDifficulty(5e12)).toBe('5.00 T')
  })
})

// --- formatUSD ---
describe('formatUSD', () => {
  it('formats a dollar amount', () => {
    expect(formatUSD(150.5)).toBe('$150.50')
  })

  it('formats zero', () => {
    expect(formatUSD(0)).toBe('$0.00')
  })
})

// --- formatCAD ---
describe('formatCAD', () => {
  it('formats a CAD amount', () => {
    expect(formatCAD(200.75)).toBe('C$200.75')
  })

  it('formats zero', () => {
    expect(formatCAD(0)).toBe('C$0.00')
  })
})

// --- fetcher ---
describe('fetcher', () => {
  const originalFetch = global.fetch

  afterEach(() => {
    global.fetch = originalFetch
  })

  it('returns parsed JSON on success (200)', async () => {
    global.fetch = jest.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: () => Promise.resolve({ data: 'test' }),
    })

    const result = await fetcher('/api/test')
    expect(result).toEqual({ data: 'test' })
    expect(global.fetch).toHaveBeenCalledWith('/api/test')
  })

  it('throws on error response (404)', async () => {
    global.fetch = jest.fn().mockResolvedValue({
      ok: false,
      status: 404,
    })

    await expect(fetcher('/api/missing')).rejects.toThrow('API error: 404')
  })

  it('throws on network error', async () => {
    global.fetch = jest.fn().mockRejectedValue(new Error('Network failure'))

    await expect(fetcher('/api/test')).rejects.toThrow('Network failure')
  })
})
