'use client'
import { useState, useEffect } from 'react'
import { api } from '@/lib/api'
import type { SystemHealth } from '@/lib/types'

export function useSystemHealth(intervalMs = 15_000) {
  const [health, setHealth] = useState<SystemHealth | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const fetch = async () => {
    try {
      const data = await api.getSystemHealth()
      setHealth(data)
      setError(null)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load health')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetch()
    const id = setInterval(fetch, intervalMs)
    return () => clearInterval(id)
  }, [intervalMs])

  return { health, loading, error }
}
