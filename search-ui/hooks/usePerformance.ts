'use client'
import { useState, useEffect } from 'react'
import { api } from '@/lib/api'
import type { PerformanceData } from '@/lib/types'

export function usePerformance(live: boolean, intervalMs = 10_000, n = 1000) {
  const [data, setData] = useState<PerformanceData | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const fetch = async () => {
    try {
      const result = await api.getPerformance(n)
      setData(result)
      setError(null)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load performance')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetch()
    if (!live) return
    const id = setInterval(fetch, intervalMs)
    return () => clearInterval(id)
  }, [live, intervalMs, n])

  return { data, loading, error, refresh: fetch }
}
