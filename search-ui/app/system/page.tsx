'use client'
import { TopBar } from '@/components/layout/TopBar'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { useSystemHealth } from '@/hooks/useSystemHealth'
import { useSystemStats } from '@/hooks/useSystemStats'
import { useDatasets } from '@/hooks/useDatasets'

function ServiceCard({ name, ok, latency }: { name: string; ok: boolean; latency: number }) {
  const tooSlow = latency > 500
  return (
    <div className="flex items-center gap-3 px-4 py-3 border border-warm-border rounded-card">
      <span className={`w-2.5 h-2.5 rounded-full shrink-0 ${ok ? 'bg-forest' : 'bg-danger'}`} />
      <span className="text-sm font-medium text-ink capitalize flex-1">{name.replace('_', ' ')}</span>
      <span className={`text-xs font-mono ${tooSlow ? 'text-danger' : 'text-ink-secondary'}`}>
        {latency}ms
      </span>
      {ok ? (
        <Badge variant="success">✓</Badge>
      ) : (
        <Badge variant="danger">✗</Badge>
      )}
    </div>
  )
}

export default function SystemPage() {
  const { health } = useSystemHealth()
  const { stats } = useSystemStats()
  const { datasets } = useDatasets()

  const small  = datasets.filter((d) => d.record_count < 100_000).length
  const medium = datasets.filter((d) => d.record_count >= 100_000 && d.record_count < 5_000_000).length
  const large  = datasets.filter((d) => d.record_count >= 5_000_000).length

  return (
    <>
      <TopBar title="System Health" />
      <div className="flex-1 overflow-y-auto p-6 flex flex-col gap-5">

        {/* Overall status */}
        <div className="flex items-center gap-3">
          {health?.all_ok ? (
            <Badge variant="success" className="text-sm px-3 py-1">All Systems Operational ✓</Badge>
          ) : (
            <Badge variant="danger" className="text-sm px-3 py-1">Degraded — check services below</Badge>
          )}
        </div>

        {/* Service cards */}
        <Card>
          <CardHeader><CardTitle>Services</CardTitle></CardHeader>
          <CardContent className="grid grid-cols-2 gap-3">
            {(health?.services ?? []).map((svc) => (
              <ServiceCard key={svc.name} name={svc.name} ok={svc.ok} latency={svc.latency_ms} />
            ))}
            {!health?.services?.length && (
              <p className="col-span-2 text-sm text-ink-muted">Loading service status…</p>
            )}
          </CardContent>
        </Card>

        {/* Outbox pipeline */}
        <Card>
          <CardHeader><CardTitle>Outbox Pipeline</CardTitle></CardHeader>
          <CardContent className="grid grid-cols-3 gap-4 text-sm">
            <div>
              <p className="text-xs text-ink-muted mb-1">Pending</p>
              <p className="font-mono text-lg font-semibold text-ink">
                {stats?.outbox_pending?.toLocaleString() ?? '—'}
              </p>
            </div>
          </CardContent>
        </Card>

        {/* Tier summary */}
        <Card>
          <CardHeader><CardTitle>Tier Summary</CardTitle></CardHeader>
          <CardContent className="flex gap-5 text-sm">
            <div className="flex items-center gap-2">
              <span className="w-2.5 h-2.5 rounded-full bg-forest" />
              <span className="text-ink-secondary">Small</span>
              <span className="font-mono font-semibold text-ink">{small}</span>
            </div>
            <div className="flex items-center gap-2">
              <span className="w-2.5 h-2.5 rounded-full bg-amber" />
              <span className="text-ink-secondary">Medium</span>
              <span className="font-mono font-semibold text-ink">{medium}</span>
            </div>
            <div className="flex items-center gap-2">
              <span className="w-2.5 h-2.5 rounded-full bg-ink" />
              <span className="text-ink-secondary">Large</span>
              <span className="font-mono font-semibold text-ink">{large}</span>
            </div>
          </CardContent>
        </Card>

        {/* Link to Grafana */}
        <a
          href="http://localhost:3001"
          target="_blank"
          rel="noopener noreferrer"
          className="text-sm text-orange hover:underline self-start"
        >
          Open Grafana Dashboard →
        </a>
      </div>
    </>
  )
}
