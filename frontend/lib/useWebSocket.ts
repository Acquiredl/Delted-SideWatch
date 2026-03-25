'use client'

import { useState, useEffect, useRef, useCallback } from 'react'

interface WebSocketState<T> {
  data: T | null
  isConnected: boolean
  error: string | null
}

const MAX_RECONNECT_ATTEMPTS = 5
const BASE_DELAY_MS = 1000

export function useWebSocket<T = unknown>(url: string): WebSocketState<T> {
  const [data, setData] = useState<T | null>(null)
  const [isConnected, setIsConnected] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const wsRef = useRef<WebSocket | null>(null)
  const reconnectAttempts = useRef(0)
  const reconnectTimer = useRef<ReturnType<typeof setTimeout> | null>(null)

  const connect = useCallback(() => {
    if (typeof window === 'undefined') return

    try {
      const ws = new WebSocket(url)
      wsRef.current = ws

      ws.onopen = () => {
        setIsConnected(true)
        setError(null)
        reconnectAttempts.current = 0
      }

      ws.onmessage = (event) => {
        try {
          const parsed = JSON.parse(event.data) as T
          setData(parsed)
        } catch {
          setError('Failed to parse WebSocket message')
        }
      }

      ws.onerror = () => {
        setError('WebSocket connection error')
      }

      ws.onclose = () => {
        setIsConnected(false)
        wsRef.current = null

        if (reconnectAttempts.current < MAX_RECONNECT_ATTEMPTS) {
          const delay = BASE_DELAY_MS * Math.pow(2, reconnectAttempts.current)
          reconnectAttempts.current += 1
          reconnectTimer.current = setTimeout(() => {
            connect()
          }, delay)
        } else {
          setError('WebSocket disconnected after max reconnect attempts')
        }
      }
    } catch {
      setError('Failed to create WebSocket connection')
    }
  }, [url])

  useEffect(() => {
    connect()

    return () => {
      if (reconnectTimer.current) {
        clearTimeout(reconnectTimer.current)
      }
      if (wsRef.current) {
        wsRef.current.close()
      }
    }
  }, [connect])

  return { data, isConnected, error }
}
