'use client'
import { useState } from 'react'
import { Search } from 'lucide-react'
import { Input, Select } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import type { Dataset } from '@/lib/types'

interface SearchBoxProps {
  datasets: Dataset[]
  datasetId: string
  onDatasetChange: (id: string) => void
  term: string
  onTermChange: (t: string) => void
  onSearch: () => void
  loading: boolean
}

export function SearchBox({
  datasets,
  datasetId,
  onDatasetChange,
  term,
  onTermChange,
  onSearch,
  loading,
}: SearchBoxProps) {
  return (
    <div className="flex gap-3 items-center">
      <Select value={datasetId} onChange={(e) => onDatasetChange(e.target.value)} className="w-56 shrink-0">
        <option value="">Select dataset…</option>
        {datasets.map((d) => (
          <option key={d.id} value={d.id}>
            {d.name} ({d.record_count.toLocaleString()})
          </option>
        ))}
      </Select>

      <div className="relative flex-1">
        <Search size={15} className="absolute left-3 top-1/2 -translate-y-1/2 text-ink-muted" />
        <Input
          placeholder="Search records…"
          value={term}
          onChange={(e) => onTermChange(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && onSearch()}
          className="pl-9"
        />
      </div>

      <Button onClick={onSearch} disabled={!datasetId || !term.trim() || loading}>
        {loading ? 'Searching…' : 'Search'}
      </Button>
    </div>
  )
}
