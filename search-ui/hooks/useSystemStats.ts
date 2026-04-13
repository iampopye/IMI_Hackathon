'use client'
import { useState, useEffect } from 'react'
import { api } from '@/lib/api'
import type { SystemStats } from '@/lib/types'

export function useSystemStats(intervalMs = 30_000) {
  const [stats, setStats] = useState<SystemStats | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const fetch = async () => {
    try {
      const data = await api.getSystemStats()
      setStats(data)
      setError(null)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load stats')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetch()
    const id = setInterval(fetch, intervalMs)
    return () => clearInterval(id)
  }, [intervalMs])

  return { stats, loading, error }
}
