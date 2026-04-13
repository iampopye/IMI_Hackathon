'use client'
import { useState } from 'react'
import { Plus } from 'lucide-react'
import { TopBar } from '@/components/layout/TopBar'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { DatasetTable } from '@/components/datasets/DatasetTable'
import { UploadZone } from '@/components/datasets/UploadZone'
import { NewDatasetModal } from '@/components/datasets/NewDatasetModal'
import { useDatasets } from '@/hooks/useDatasets'

export default function DatasetsPage() {
  const { datasets, loading, refresh } = useDatasets()
  const [showModal, setShowModal] = useState(false)
  const [uploadDatasetId, setUploadDatasetId] = useState('')

  return (
    <>
      <TopBar title="Datasets" />
      <div className="flex-1 overflow-y-auto p-6 flex flex-col gap-5">

        {/* Upload zone */}
        {uploadDatasetId && (
          <Card>
            <CardHeader>
              <CardTitle>Upload to {datasets.find((d) => d.id === uploadDatasetId)?.name ?? uploadDatasetId}</CardTitle>
            </CardHeader>
            <CardContent>
              <UploadZone datasetId={uploadDatasetId} onComplete={() => refresh()} />
            </CardContent>
          </Card>
        )}

        {!uploadDatasetId && (
          <Card>
            <CardContent className="py-5">
              <p className="text-sm text-ink-secondary mb-3">
                Select a dataset below to upload records, or create a new dataset first.
              </p>
              <Button size="sm" variant="ghost" onClick={() => setShowModal(true)}>
                <Plus size={14} /> New Dataset
              </Button>
            </CardContent>
          </Card>
        )}

        {/* Dataset list */}
        <Card>
          <CardHeader className="flex flex-row items-center justify-between">
            <CardTitle>Datasets ({datasets.length})</CardTitle>
            <div className="flex gap-2">
              <Button size="sm" variant="ghost" onClick={() => setShowModal(true)}>
                <Plus size={13} /> New
              </Button>
            </div>
          </CardHeader>
          <CardContent className="p-0">
            {loading ? (
              <p className="text-sm text-ink-muted px-5 py-4">Loading…</p>
            ) : (
              <DatasetTable datasets={datasets} />
            )}
          </CardContent>
        </Card>

        {/* Click dataset → show upload zone */}
        {datasets.length > 0 && !uploadDatasetId && (
          <div className="flex flex-wrap gap-2">
            {datasets.map((d) => (
              <button
                key={d.id}
                onClick={() => setUploadDatasetId(d.id)}
                className="text-xs px-3 py-1.5 border border-warm-border rounded-input text-ink-secondary hover:text-ink hover:bg-warm-beige transition-colors"
              >
                Upload to {d.name}
              </button>
            ))}
          </div>
        )}

        {uploadDatasetId && (
          <button
            onClick={() => setUploadDatasetId('')}
            className="text-xs text-ink-muted hover:text-ink self-start"
          >
            ← Back to list
          </button>
        )}
      </div>

      {showModal && (
        <NewDatasetModal
          onClose={() => setShowModal(false)}
          onCreated={() => { setShowModal(false); refresh() }}
        />
      )}
    </>
  )
}
