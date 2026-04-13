'use client'
import { useEffect, useState } from 'react'
import { useParams } from 'next/navigation'
import Link from 'next/link'
import { ArrowLeft, RefreshCw } from 'lucide-react'
import { TopBar } from '@/components/layout/TopBar'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { StabilityBar } from '@/components/datasets/StabilityBar'
import { UploadZone } from '@/components/datasets/UploadZone'
import { Badge } from '@/components/ui/badge'
import { api } from '@/lib/api'
import type { Dataset, DatasetStats } from '@/lib/types'

export default function DatasetDetailPage() {
  const { id } = useParams<{ id: string }>()
  const [dataset, setDataset] = useState<Dataset | null>(null)
  const [stats, setStats] = useState<DatasetStats | null>(null)

  useEffect(() => {
    const load = async () => {
      try {
        const [listRes, statsRes] = await Promise.all([
          api.listDatasets(),
          api.getDatasetStats(id),
        ])
        setDataset(listRes.datasets.find((d) => d.id === id) ?? null)
        setStats(statsRes)
      } catch { /* noop */ }
    }
    load()
  }, [id])

  const tierLabel = (count: number) => {
    if (count >= 5_000_000) return <Badge variant="default">Large</Badge>
    if (count >= 100_000)   return <Badge variant="warning">Medium</Badge>
    return                         <Badge variant="success">Small</Badge>
  }

  return (
    <>
      <TopBar title={dataset?.name ?? 'Dataset'} />
      <div className="flex-1 overflow-y-auto p-6 flex flex-col gap-5">

        <Link href="/datasets" className="flex items-center gap-1.5 text-sm text-ink-secondary hover:text-ink w-fit">
          <ArrowLeft size={14} /> Back to Datasets
        </Link>

        {dataset && (
          <div className="flex items-center gap-3 text-sm text-ink-secondary">
            <span className="font-mono">{dataset.record_count.toLocaleString()} records</span>
            <span>·</span>
            {tierLabel(dataset.record_count)}
            <span>·</span>
            <span>{dataset.state}</span>
          </div>
        )}

        {/* Stability */}
        {dataset && (
          <Card>
            <CardHeader><CardTitle>Stability Score</CardTitle></CardHeader>
            <CardContent>
              <div className="flex items-center gap-4">
                <div className="flex-1">
                  <StabilityBar score={dataset.stability_score} />
                </div>
                <Badge variant={dataset.stability_score >= 0.7 ? 'success' : 'warning'}>
                  {dataset.state}
                </Badge>
              </div>
            </CardContent>
          </Card>
        )}

        {/* Sync history */}
        <Card>
          <CardHeader><CardTitle>Sync History</CardTitle></CardHeader>
          <CardContent className="p-0">
            {!stats?.sync_history?.length ? (
              <p className="px-5 py-4 text-sm text-ink-muted">No sync history yet.</p>
            ) : (
              <div className="divide-y divide-warm-border">
                {stats.sync_history.map((entry, i) => (
                  <div key={i} className="px-5 py-3 flex items-center gap-4 text-sm">
                    <span className="font-mono text-xs text-ink-muted w-32 shrink-0">
                      {new Date(entry.time).toLocaleString('en-IN', { month: 'short', day: '2-digit', hour: '2-digit', minute: '2-digit', hour12: false })}
                    </span>
                    <span className="text-ink-secondary">{entry.type}</span>
                    <span className="ml-auto text-xs font-mono text-forest">+{entry.inserted.toLocaleString()}</span>
                    <span className="text-xs font-mono text-ink-muted">{entry.skipped} skipped</span>
                    {entry.failed > 0 && <span className="text-xs font-mono text-danger">{entry.failed} failed</span>}
                  </div>
                ))}
              </div>
            )}
          </CardContent>
        </Card>

        {/* Value fields */}
        {stats?.value_fields?.length ? (
          <Card>
            <CardHeader><CardTitle>Value Fields</CardTitle></CardHeader>
            <CardContent>
              <div className="flex flex-wrap gap-2">
                {stats.value_fields.map((f) => (
                  <span key={f} className="px-2.5 py-1 bg-warm-beige text-ink-secondary text-xs rounded-pill">{f}</span>
                ))}
              </div>
            </CardContent>
          </Card>
        ) : null}

        {/* Upload */}
        <Card>
          <CardHeader className="flex flex-row items-center gap-2">
            <RefreshCw size={13} className="text-ink-muted" />
            <CardTitle>Upload Records</CardTitle>
          </CardHeader>
          <CardContent>
            <UploadZone datasetId={id} />
          </CardContent>
        </Card>
      </div>
    </>
  )
}
