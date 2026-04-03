'use client'

import { useState, useEffect, FormEvent } from 'react'

const API_BASE = process.env.NEXT_PUBLIC_API_URL || ''
const TOKEN_KEY = 'p2pool_admin_token'

interface HealthStatus {
  status: string
  postgres: string
  redis: string
}

export default function AdminPage() {
  const [token, setToken] = useState<string | null>(null)
  const [tokenInput, setTokenInput] = useState('')
  const [loginError, setLoginError] = useState<string | null>(null)
  const [health, setHealth] = useState<HealthStatus | null>(null)
  const [healthError, setHealthError] = useState<string | null>(null)

  useEffect(() => {
    const stored = localStorage.getItem(TOKEN_KEY)
    if (stored) {
      setToken(stored)
    }
  }, [])

  useEffect(() => {
    if (!token) return

    async function checkHealth() {
      try {
        const res = await fetch(`${API_BASE}/health`, {
          headers: { Authorization: `Bearer ${token}` },
        })
        if (!res.ok) {
          if (res.status === 401 || res.status === 403) {
            setToken(null)
            localStorage.removeItem(TOKEN_KEY)
            setLoginError('Token is invalid or expired')
            return
          }
          throw new Error(`HTTP ${res.status}`)
        }
        const data = await res.json() as HealthStatus
        setHealth(data)
        setHealthError(null)
      } catch (err) {
        setHealthError(err instanceof Error ? err.message : 'Unknown error')
      }
    }

    checkHealth()
  }, [token])

  async function handleLogin(e: FormEvent) {
    e.preventDefault()
    setLoginError(null)

    const trimmed = tokenInput.trim()
    if (!trimmed) {
      setLoginError('Please enter a token')
      return
    }

    try {
      const res = await fetch(`${API_BASE}/health`, {
        headers: { Authorization: `Bearer ${trimmed}` },
      })
      if (!res.ok) {
        setLoginError('Invalid token')
        return
      }
      localStorage.setItem(TOKEN_KEY, trimmed)
      setToken(trimmed)
      setTokenInput('')
    } catch {
      setLoginError('Failed to validate token')
    }
  }

  function handleLogout() {
    localStorage.removeItem(TOKEN_KEY)
    setToken(null)
    setHealth(null)
  }

  if (!token) {
    return (
      <div>
        <div className="mb-8">
          <h1 className="text-3xl font-bold text-zinc-100 mb-2">Admin Panel</h1>
          <p className="text-zinc-400 text-sm">
            Enter your admin JWT token to access the admin panel.
          </p>
        </div>

        <form onSubmit={handleLogin} className="max-w-lg">
          <div className="mb-4">
            <label htmlFor="token" className="block text-zinc-400 text-sm mb-2">
              JWT Token
            </label>
            <input
              id="token"
              type="password"
              value={tokenInput}
              onChange={(e) => setTokenInput(e.target.value)}
              placeholder="Paste your admin token here"
              className="input-field"
            />
          </div>
          {loginError && (
            <p className="text-red-400 text-sm mb-4">{loginError}</p>
          )}
          <button type="submit" className="btn-primary">
            Authenticate
          </button>
        </form>
      </div>
    )
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-8">
        <div>
          <h1 className="text-3xl font-bold text-zinc-100 mb-2">Admin Panel</h1>
          <p className="text-zinc-400 text-sm">Authenticated as admin</p>
        </div>
        <button onClick={handleLogout} className="btn-secondary text-sm">
          Logout
        </button>
      </div>

      <div className="mb-8">
        <h2 className="text-xl font-bold text-zinc-100 mb-4">Service Health</h2>
        {healthError && (
          <div className="text-red-400 text-sm p-4 bg-red-900/20 border border-red-800 rounded-lg mb-4">
            Failed to fetch health: {healthError}
          </div>
        )}
        {health && (
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
            <div className="stat-card">
              <p className="text-zinc-400 text-sm mb-1">API Status</p>
              <p className={`text-xl font-bold ${health.status === 'ok' ? 'text-green-400' : 'text-red-400'}`}>
                {health.status}
              </p>
            </div>
            <div className="stat-card">
              <p className="text-zinc-400 text-sm mb-1">PostgreSQL</p>
              <p className={`text-xl font-bold ${health.postgres === 'ok' ? 'text-green-400' : 'text-red-400'}`}>
                {health.postgres}
              </p>
            </div>
            <div className="stat-card">
              <p className="text-zinc-400 text-sm mb-1">Redis</p>
              <p className={`text-xl font-bold ${health.redis === 'ok' ? 'text-green-400' : 'text-red-400'}`}>
                {health.redis}
              </p>
            </div>
          </div>
        )}
      </div>

      <div className="stat-card text-zinc-500 py-6 text-sm">
        <p className="mb-2 text-zinc-400 font-medium">Admin Endpoints</p>
        <ul className="space-y-1 text-xs">
          <li><code className="text-zinc-400">POST /api/admin/backfill-prices</code> &mdash; backfill historical XMR/USD + XMR/CAD for payments</li>
          <li><code className="text-zinc-400">POST /api/admin/backfill-sub-prices</code> &mdash; backfill subscription payment fiat values</li>
          <li><code className="text-zinc-400">GET /api/admin/subscription-income</code> &mdash; subscription revenue analytics</li>
        </ul>
      </div>
    </div>
  )
}
