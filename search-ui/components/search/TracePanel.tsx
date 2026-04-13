'use client'
import { useState } from 'react'
import { ChevronDown, ChevronRight } from 'lucide-react'
import type { SearchResult } from '@/lib/types'

function ms(ns: number) {
  return (ns / 1_000_000).toFixed(2) + 'ms'
}

export function TracePanel({ result }: { result: SearchResult }) {
  const [open, setOpen] = useState(false)
  const totalMs = (result.took_ns / 1_000_000).toFixed(2)
  const cacheHit = result.engine.includes('cache')

  return (
    <div className="border border-warm-border rounded-card bg-warm-white overflow-hidden">
      <button
        onClick={() => setOpen(!open)}
        className="w-full flex items-center gap-2 px-4 py-3 text-sm text-ink-secondary hover:text-ink hover:bg-warm-beige transition-colors"
      >
        {open ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
        <span className="font-medium">Performance</span>
        <span className="ml-auto font-mono text-ink">{totalMs}ms</span>
        {cacheHit && (
          <span className="flex items-center gap-1 text-xs text-forest font-medium">
            <span className="w-1.5 h-1.5 bg-forest rounded-full" /> cached
          </span>
        )}
      </button>

      {open && (
        <div className="border-t border-warm-border px-4 py-3 grid grid-cols-2 gap-x-8 gap-y-2 text-xs">
          <Row label="Engine" value={result.engine} mono />
          <Row label="Total time" value={ms(result.took_ns)} mono />
          <Row label="Total hits" value={result.total.toLocaleString()} mono />
          <Row label="Cache hit" value={cacheHit ? 'yes' : 'no'} mono />
        </div>
      )}
    </div>
  )
}

function Row({ label, value, mono }: { label: string; value: string | number; mono?: boolean }) {
  return (
    <>
      <span className="text-ink-muted">{label}</span>
      <span className={`text-ink ${mono ? 'font-mono' : ''}`}>{value}</span>
    </>
  )
}
