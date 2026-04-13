'use client'
import { useState } from 'react'
import { TopBar } from '@/components/layout/TopBar'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { LatencyChart } from '@/components/charts/LatencyChart'
import { EngineDonut } from '@/components/charts/EngineDonut'
import { EngineBadge } from '@/components/search/EngineBadge'
import { usePerformance } from '@/hooks/usePerformance'

function StatCard({ label, value }: { label: string; value: string }) {
  return (
    <Card className="flex-1">
      <CardContent className="py-5">
        <p className="text-xs text-ink-muted font-medium mb-1">{label}</p>
        <p className="font-mono text-2xl font-semibold text-ink">{value}</p>
      </CardContent>
    </Card>
  )
}

function latencyColor(ms: number) {
  if (ms < 5) return 'text-forest'
  if (ms < 20) return 'text-amber'
  return 'text-danger'
}

export default function PerformancePage() {
  const [live, setLive] = useState(true)
  const { data } = usePerformance(live, 10_000)

  const latencyChartData = (data?.queries ?? []).slice(-50).map((q, i) => ({
    label: String(i),
    p50: q.latency_ms,
    p95: q.latency_ms,
    p99: q.latency_ms,
  }))

  const engineCounts = (() => {
    const map: Record<string, number> = {}
    for (const q of data?.queries ?? []) {
      const key = q.cache_hit ? 'cache' : q.engine.split('+')[0]
      map[key] = (map[key] ?? 0) + 1
    }
    return Object.entries(map).map(([engine, count]) => ({ engine, count }))
  })()

  return (
    <>
      <TopBar title="Performance" live={live} onToggleLive={() => setLive((v) => !v)} />
      <div className="flex-1 overflow-y-auto p-6 flex flex-col gap-5">

        <div className="flex gap-4">
          <StatCard label="p50" value={`${data?.p50?.toFixed(1) ?? '—'}ms`} />
          <StatCard label="p95" value={`${data?.p95?.toFixed(1) ?? '—'}ms`} />
          <StatCard label="p99" value={`${data?.p99?.toFixed(1) ?? '—'}ms`} />
          <StatCard label="Cache Hit Rate" value={`${data?.cache_hit_rate?.toFixed(1) ?? '—'}%`} />
        </div>

        <div className="grid grid-cols-2 gap-4">
          <Card>
            <CardHeader><CardTitle>Latency — last 50 queries</CardTitle></CardHeader>
            <CardContent>
              <LatencyChart data={latencyChartData} />
            </CardContent>
          </Card>
          <Card>
            <CardHeader><CardTitle>Engine Distribution</CardTitle></CardHeader>
            <CardContent>
              <EngineDonut data={engineCounts} />
            </CardContent>
          </Card>
        </div>

        {/* Live query log */}
        <Card>
          <CardHeader className="flex flex-row items-center gap-2">
            <CardTitle>Live Query Log</CardTitle>
            {live && (
              <span className="ml-auto text-xs text-forest flex items-center gap-1">
                <span className="w-1.5 h-1.5 bg-forest rounded-full animate-pulse" /> refreshing
              </span>
            )}
          </CardHeader>
          <div className="divide-y divide-warm-border max-h-96 overflow-y-auto">
            {(data?.queries ?? []).slice().reverse().slice(0, 50).map((q, i) => (
              <div key={i} className="px-5 py-2.5 flex items-center gap-3 text-xs">
                <span className="font-mono text-ink-muted w-20 shrink-0">
                  {new Date(q.time).toLocaleTimeString('en-IN', { hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false })}
                </span>
                <span className="text-ink font-medium truncate w-32 shrink-0">&ldquo;{q.term}&rdquo;</span>
                <EngineBadge engine={q.cache_hit ? 'cache' : q.engine} />
                <span className={`font-mono font-semibold ml-auto ${latencyColor(q.latency_ms)}`}>
                  {q.latency_ms.toFixed(1)}ms
                </span>
                <span className="text-ink-muted w-16 text-right">{q.hits.toLocaleString()} hits</span>
                {q.cache_hit && <span className="text-forest">●</span>}
              </div>
            ))}
            {!data?.queries?.length && (
              <p className="px-5 py-6 text-sm text-ink-muted text-center">No queries recorded yet.</p>
            )}
          </div>
        </Card>
      </div>
    </>
  )
}
