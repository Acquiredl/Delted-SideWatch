import { renderHook, act } from '@testing-library/react'
import { useWebSocket } from '@/lib/useWebSocket'

// Mock WebSocket
class MockWebSocket {
  static instances: MockWebSocket[] = []
  url: string
  onopen: ((event: Event) => void) | null = null
  onmessage: ((event: MessageEvent) => void) | null = null
  onerror: ((event: Event) => void) | null = null
  onclose: ((event: CloseEvent) => void) | null = null
  readyState: number = 0
  closeCalled = false

  constructor(url: string) {
    this.url = url
    MockWebSocket.instances.push(this)
  }

  close() {
    this.closeCalled = true
  }

  simulateOpen() {
    this.readyState = 1
    if (this.onopen) {
      this.onopen(new Event('open'))
    }
  }

  simulateMessage(data: unknown) {
    if (this.onmessage) {
      this.onmessage({ data: JSON.stringify(data) } as MessageEvent)
    }
  }

  simulateError() {
    if (this.onerror) {
      this.onerror(new Event('error'))
    }
  }

  simulateClose() {
    this.readyState = 3
    if (this.onclose) {
      this.onclose(new CloseEvent('close'))
    }
  }
}

describe('useWebSocket', () => {
  beforeEach(() => {
    MockWebSocket.instances = []
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    ;(global as any).WebSocket = MockWebSocket
    jest.useFakeTimers()
  })

  afterEach(() => {
    jest.useRealTimers()
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    delete (global as any).WebSocket
  })

  it('starts with initial state: data null, not connected, no error', () => {
    const { result } = renderHook(() => useWebSocket('ws://test'))

    expect(result.current.data).toBeNull()
    expect(result.current.isConnected).toBe(false)
    expect(result.current.error).toBeNull()
  })

  it('sets isConnected to true on open', () => {
    const { result } = renderHook(() => useWebSocket('ws://test'))

    act(() => {
      MockWebSocket.instances[0].simulateOpen()
    })

    expect(result.current.isConnected).toBe(true)
    expect(result.current.error).toBeNull()
  })

  it('parses data from JSON message', () => {
    const { result } = renderHook(() =>
      useWebSocket<{ value: number }>('ws://test')
    )

    act(() => {
      MockWebSocket.instances[0].simulateOpen()
    })

    act(() => {
      MockWebSocket.instances[0].simulateMessage({ value: 42 })
    })

    expect(result.current.data).toEqual({ value: 42 })
  })

  it('sets error on WebSocket error', () => {
    const { result } = renderHook(() => useWebSocket('ws://test'))

    act(() => {
      MockWebSocket.instances[0].simulateError()
    })

    expect(result.current.error).toBe('WebSocket connection error')
  })

  it('cleans up WebSocket on unmount', () => {
    const { unmount } = renderHook(() => useWebSocket('ws://test'))

    const ws = MockWebSocket.instances[0]

    unmount()

    expect(ws.closeCalled).toBe(true)
  })
})
