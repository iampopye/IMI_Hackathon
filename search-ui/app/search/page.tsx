'use client'
import { useState } from 'react'
import { TopBar } from '@/components/layout/TopBar'
import { SearchBox } from '@/components/search/SearchBox'
import { ResultsTable } from '@/components/search/ResultsTable'
import { TracePanel } from '@/components/search/TracePanel'
import { EngineBadge } from '@/components/search/EngineBadge'
import { Card, CardContent } from '@/components/ui/card'
import { useDatasets } from '@/hooks/useDatasets'
import { useSearch } from '@/hooks/useSearch'

const PAGE_SIZE = 20

export default function SearchPage() {
  const { datasets } = useDatasets()
  const [datasetId, setDatasetId] = useState('')
  const [term, setTerm] = useState('')
  const [activeTerm, setActiveTerm] = useState('')
  const [page, setPage] = useState(1)

  const offset = (page - 1) * PAGE_SIZE
  const { data, loading, error } = useSearch(datasetId, activeTerm, PAGE_SIZE, offset)

  const handleSearch = () => {
    setPage(1)
    setActiveTerm(term)
  }

  return (
    <>
      <TopBar title="Search" />
      <div className="flex-1 overflow-y-auto p-6 flex flex-col gap-4">
        <Card>
          <CardContent className="pt-5">
            <SearchBox
              datasets={datasets}
              datasetId={datasetId}
              onDatasetChange={(id) => { setDatasetId(id); setActiveTerm(''); setPage(1) }}
              term={term}
              onTermChange={setTerm}
              onSearch={handleSearch}
              loading={loading}
            />
          </CardContent>
        </Card>

        {error && (
          <div className="text-sm text-danger bg-danger-light px-4 py-3 rounded-card">{error}</div>
        )}

        {data && (
          <>
            <div className="flex items-center gap-3 text-sm text-ink-secondary">
              <span className="font-medium text-ink">{data.total.toLocaleString()} results</span>
              <span>·</span>
              <EngineBadge engine={data.engine} />
              <span>·</span>
              <span className="font-mono">{(data.took_ns / 1_000_000).toFixed(2)}ms</span>
              {data.engine.includes('cache') && (
                <>
                  <span>·</span>
                  <span className="text-forest text-xs font-medium">● cached</span>
                </>
              )}
            </div>

            <Card>
              <CardContent className="py-0 px-0">
                <ResultsTable
                  hits={data.hits}
                  page={page}
                  pageSize={PAGE_SIZE}
                  total={Number(data.total)}
                  onPageChange={(p) => setPage(p)}
                />
              </CardContent>
            </Card>

            <TracePanel result={data} />
          </>
        )}

        {loading && (
          <div className="text-sm text-ink-muted text-center py-8">Searching…</div>
        )}

        {!loading && !data && activeTerm && (
          <div className="text-sm text-ink-muted text-center py-8">No results for &ldquo;{activeTerm}&rdquo;</div>
        )}
      </div>
    </>
  )
}
