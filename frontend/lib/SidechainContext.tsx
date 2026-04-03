'use client'

import { createContext, useContext, useState, useEffect, useCallback, type ReactNode } from 'react'
import useSWR from 'swr'
import { fetcher } from './api'

interface SidechainContextValue {
  sidechain: string
  setSidechain: (sc: string) => void
  sidechains: string[]
  isLoading: boolean
}

const SidechainContext = createContext<SidechainContextValue>({
  sidechain: 'mini',
  setSidechain: () => {},
  sidechains: ['mini'],
  isLoading: true,
})

export function useSidechain() {
  return useContext(SidechainContext)
}

/** Appends ?sidechain= to an API path, preserving existing query params. */
export function withSidechain(path: string, sidechain: string): string {
  const sep = path.includes('?') ? '&' : '?'
  return `${path}${sep}sidechain=${sidechain}`
}

export function SidechainProvider({ children }: { children: ReactNode }) {
  const { data: sidechains } = useSWR<string[]>('/api/sidechains', fetcher)
  const [sidechain, setSidechainState] = useState('mini')

  // Read initial sidechain from URL query param on mount.
  useEffect(() => {
    if (typeof window === 'undefined') return
    const params = new URLSearchParams(window.location.search)
    const sc = params.get('sidechain')
    if (sc && sidechains?.includes(sc)) {
      setSidechainState(sc)
    } else if (sidechains && sidechains.length > 0 && !sidechains.includes(sidechain)) {
      setSidechainState(sidechains[0])
    }
  }, [sidechains]) // eslint-disable-line react-hooks/exhaustive-deps

  const setSidechain = useCallback((sc: string) => {
    setSidechainState(sc)
    // Update URL query param without triggering navigation.
    if (typeof window !== 'undefined') {
      const url = new URL(window.location.href)
      url.searchParams.set('sidechain', sc)
      window.history.replaceState({}, '', url.toString())
    }
  }, [])

  return (
    <SidechainContext.Provider
      value={{
        sidechain,
        setSidechain,
        sidechains: sidechains ?? ['mini'],
        isLoading: !sidechains,
      }}
    >
      {children}
    </SidechainContext.Provider>
  )
}
