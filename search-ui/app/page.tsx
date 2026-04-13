'use client'
import { useEffect, useState } from 'react'
import { TopBar } from '@/components/layout/TopBar'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { LatencyChart } from '@/components/charts/LatencyChart'
import { EngineDonut } from '@/components/charts/EngineDonut'
import { ThroughputArea } from '@/components/charts/ThroughputArea'
import { useSystemStats } from '@/hooks/useSystemStats'
import { usePerformance } from '@/hooks/usePerformance'
import { api } from '@/lib/api'
import type { ActivityEvent } from '@/lib/types'

function StatCard({ label, value, sub }: { label: string; value: string; sub?: string }) {
  return (
    <Card className="flex-1">
      <CardContent className="py-5">
        <p className="text-xs text-ink-muted font-medium mb-1">{label}</p>
        <p className="font-mono text-2xl font-semibold text-ink">{value}</p>
        {sub && <p className="text-xs text-ink-muted mt-0.5">{sub}</p>}
      </CardContent>
    </Card>
  )
}

function fmt(n: number) {
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(2) + 'M'
  if (n >= 1_000) return (n / 1_000).toFixed(1) + 'K'
  return n.toString()
}

export default function OverviewPage() {
  const { stats } = useSystemStats(30_000)
  const { data: perf } = usePerformance(true, 30_000)
  const [activity, setActivity] = useState<ActivityEvent[]>([])

  useEffect(() => {
    const load = async () => {
      try {
        const res = await api.getActivity(20)
        setActivity(res.events ?? [])
      } catch { /* noop */ }
    }
    load()
    const id = setInterval(load, 30_000)
    return () => clearInterval(id)
  }, [])

  // Build latency chart data from query log
  const latencyData = (() => {
    if (!perf?.queries?.length) return []
    const buckets = perf.queries.slice(-24).map((q, i) => ({
      label: String(i),
      p50: q.latency_ms,
      p95: q.latency_ms,
      p99: q.latency_ms,
    }))
    return buckets
  })()

  // Engine distribution from query log
  const engineCounts = (() => {
    if (!perf?.queries) return []
    const map: Record<string, number> = {}
    for (const q of perf.queries) {
      const key = q.cache_hit ? 'cache' : q.engine.split('+')[0]
      map[key] = (map[key] ?? 0) + 1
    }
    return Object.entries(map).map(([engine, count]) => ({ engine, count }))
  })()

  return (
    <>
      <TopBar title="Overview" />
      <div className="flex-1 overflow-y-auto p-6 flex flex-col gap-5">

        {/* Stat cards */}
        <div className="flex gap-4">
          <StatCard label="Total Records" value={fmt(stats?.total_records ?? 0)} />
          <StatCard label="Searches Today" value={(stats?.searches_today ?? 0).toLocaleString()} />
          <StatCard label="Cache Hit Rate" value={`${(stats?.cache_hit_rate ?? 0).toFixed(1)}%`} />
          <StatCard label="Avg Latency" value={`${(stats?.avg_latency_ms ?? 0).toFixed(1)}ms`} />
        </div>

        {/* Charts row 1 */}
        <div className="grid grid-cols-2 gap-4">
          <Card>
            <CardHeader><CardTitle>Latency — last queries</CardTitle></CardHeader>
            <CardContent>
              <LatencyChart data={latencyData} />
            </CardContent>
          </Card>
          <Card>
            <CardHeader><CardTitle>Engine Distribution</CardTitle></CardHeader>
            <CardContent>
              <EngineDonut data={engineCounts} />
            </CardContent>
          </Card>
        </div>

        {/* Charts row 2 */}
        <Card>
          <CardHeader><CardTitle>Upsert Throughput</CardTitle></CardHeader>
          <CardContent>
            <ThroughputArea data={[]} />
          </CardContent>
        </Card>

        {/* Activity feed */}
        <Card>
          <CardHeader><CardTitle>Recent Activity</CardTitle></CardHeader>
          <div className="divide-y divide-warm-border">
            {activity.length === 0 && (
              <p className="px-5 py-4 text-sm text-ink-muted">No recent activity.</p>
            )}
            {activity.map((ev, i) => (
              <div key={i} className="px-5 py-3 flex items-center gap-3 text-sm">
                <span className="text-xs font-mono text-ink-muted w-20 shrink-0">
                  {new Date(ev.time).toLocaleTimeString('en-IN', { hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false })}
                </span>
                <span className="font-medium text-ink truncate">{ev.dataset}</span>
                <span className="text-ink-secondary truncate flex-1">{ev.message || ev.type}</span>
                {ev.engine && <span className="text-xs text-ink-muted font-mono shrink-0">{ev.engine}</span>}
              </div>
            ))}
          </div>
        </Card>
      </div>
    </>
  )
}
