'use client'
import { useState, useCallback } from 'react'
import { useDropzone } from 'react-dropzone'
import { UploadCloud, CheckCircle, AlertCircle } from 'lucide-react'
import { api } from '@/lib/api'
import type { UpsertResult } from '@/lib/types'

interface UploadZoneProps {
  datasetId: string
  onComplete?: (result: UpsertResult) => void
}

type State = 'idle' | 'parsing' | 'uploading' | 'done' | 'error'

export function UploadZone({ datasetId, onComplete }: UploadZoneProps) {
  const [state, setState] = useState<State>('idle')
  const [progress, setProgress] = useState(0)
  const [result, setResult] = useState<UpsertResult | null>(null)
  const [error, setError] = useState<string | null>(null)

  const onDrop = useCallback(
    async (files: File[]) => {
      if (!files[0]) return
      setState('parsing')
      setProgress(0)
      setError(null)

      try {
        const text = await files[0].text()
        let records: unknown[]

        if (files[0].name.endsWith('.json')) {
          const parsed = JSON.parse(text)
          records = Array.isArray(parsed) ? parsed : [parsed]
        } else if (files[0].name.endsWith('.jsonl')) {
          records = text.trim().split('\n').filter(Boolean).map((l) => JSON.parse(l))
        } else {
          // CSV: first line = headers
          const lines = text.trim().split('\n')
          const headers = lines[0].split(',').map((h) => h.trim().replace(/^"|"$/g, ''))
          records = lines.slice(1).map((line) => {
            const vals = line.split(',')
            return Object.fromEntries(headers.map((h, i) => [h, vals[i]?.trim().replace(/^"|"$/g, '') ?? '']))
          })
        }

        setState('uploading')

        // Chunk into 500-record batches and fake progress
        const chunkSize = 500
        let uploaded = 0
        let last: UpsertResult = { inserted: 0, updated: 0, skipped: 0, failed: 0, total: 0 }

        for (let i = 0; i < records.length; i += chunkSize) {
          const chunk = records.slice(i, i + chunkSize)
          const res = await api.bulkUpsert(datasetId, chunk)
          last = {
            inserted: last.inserted + res.inserted,
            updated: last.updated + res.updated,
            skipped: last.skipped + res.skipped,
            failed: last.failed + res.failed,
            total: last.total + res.total,
          }
          uploaded += chunk.length
          setProgress(Math.round((uploaded / records.length) * 100))
        }

        setResult(last)
        setState('done')
        onComplete?.(last)
      } catch (e) {
        setError(e instanceof Error ? e.message : 'Upload failed')
        setState('error')
      }
    },
    [datasetId, onComplete],
  )

  const { getRootProps, getInputProps, isDragActive } = useDropzone({
    onDrop,
    accept: { 'text/csv': ['.csv'], 'application/json': ['.json', '.jsonl'] },
    multiple: false,
    disabled: state === 'parsing' || state === 'uploading',
  })

  return (
    <div className="flex flex-col gap-3">
      <div
        {...getRootProps()}
        className={`border-2 border-dashed rounded-card px-6 py-10 text-center cursor-pointer transition-colors
          ${isDragActive ? 'border-orange bg-orange-light' : 'border-warm-border hover:border-orange-border hover:bg-warm-beige'}`}
      >
        <input {...getInputProps()} />
        <UploadCloud size={28} className="mx-auto mb-3 text-ink-muted" />
        <p className="text-sm font-medium text-ink">
          {isDragActive ? 'Drop it here…' : 'Drop CSV or JSON file here, or click to browse'}
        </p>
        <p className="text-xs text-ink-muted mt-1">.csv  .json  .jsonl — Max 500 MB</p>
      </div>

      {(state === 'parsing' || state === 'uploading') && (
        <div className="flex flex-col gap-1.5">
          <div className="flex justify-between text-xs text-ink-secondary">
            <span>{state === 'parsing' ? 'Parsing file…' : `Uploading… ${progress}%`}</span>
            <span>{progress}%</span>
          </div>
          <div className="h-2 bg-warm-border rounded-pill overflow-hidden">
            <div
              className="h-full bg-orange rounded-pill transition-all"
              style={{ width: `${progress}%` }}
            />
          </div>
        </div>
      )}

      {state === 'done' && result && (
        <div className="flex items-center gap-2 text-sm text-forest bg-forest-light px-4 py-2.5 rounded-input">
          <CheckCircle size={15} />
          <span>
            Done — {result.inserted.toLocaleString()} inserted · {result.updated.toLocaleString()} updated ·{' '}
            {result.skipped.toLocaleString()} skipped · {result.failed.toLocaleString()} failed
          </span>
        </div>
      )}

      {state === 'error' && error && (
        <div className="flex items-center gap-2 text-sm text-danger bg-danger-light px-4 py-2.5 rounded-input">
          <AlertCircle size={15} />
          {error}
        </div>
      )}
    </div>
  )
}
