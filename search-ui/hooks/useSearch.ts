'use client'
import { useState, useEffect, useRef } from 'react'
import { api } from '@/lib/api'
import type { SearchResult } from '@/lib/types'

export function useSearch(datasetId: string, term: string, limit = 20, offset = 0) {
  const [data, setData] = useState<SearchResult | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(() => {
    if (!datasetId || !term.trim()) {
      setData(null)
      return
    }

    if (timer.current) clearTimeout(timer.current)
    timer.current = setTimeout(async () => {
      setLoading(true)
      setError(null)
      try {
        const result = await api.search(datasetId, term, limit, offset)
        setData(result)
      } catch (e) {
        setError(e instanceof Error ? e.message : 'Search failed')
        setData(null)
      } finally {
        setLoading(false)
      }
    }, 300)

    return () => { if (timer.current) clearTimeout(timer.current) }
  }, [datasetId, term, limit, offset])

  return { data, loading, error }
}
