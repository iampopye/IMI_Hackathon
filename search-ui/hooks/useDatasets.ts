'use client'
import { useState, useEffect } from 'react'
import { api } from '@/lib/api'
import type { Dataset } from '@/lib/types'

export function useDatasets(intervalMs = 30_000) {
  const [datasets, setDatasets] = useState<Dataset[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const fetch = async () => {
    try {
      const res = await api.listDatasets()
      setDatasets(res.datasets ?? [])
      setError(null)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load datasets')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetch()
    const id = setInterval(fetch, intervalMs)
    return () => clearInterval(id)
  }, [intervalMs])

  return { datasets, loading, error, refresh: fetch }
}
