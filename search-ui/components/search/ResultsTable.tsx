import { ExternalLink } from 'lucide-react'
import { Table, Thead, Th, Tbody, Tr, Td } from '@/components/ui/table'
import type { Hit } from '@/lib/types'

interface ResultsTableProps {
  hits: Hit[]
  page: number
  pageSize: number
  total: number
  onPageChange: (p: number) => void
}

function getWebsite(value?: Record<string, unknown>): string | null {
  if (!value) return null
  const w = value['website'] ?? value['url'] ?? value['link']
  return typeof w === 'string' ? w : null
}

export function ResultsTable({ hits, page, pageSize, total, onPageChange }: ResultsTableProps) {
  const totalPages = Math.ceil(total / pageSize)

  return (
    <div className="flex flex-col gap-3">
      <Table>
        <Thead>
          <tr>
            <Th>Name</Th>
            <Th>Score</Th>
            <Th>Website</Th>
          </tr>
        </Thead>
        <Tbody>
          {hits.map((h) => {
            const website = getWebsite(h.value)
            return (
              <Tr key={h.id}>
                <Td className="font-medium max-w-xs truncate">{h.name || h.id}</Td>
                <Td className="font-mono text-ink-secondary">{h.score.toFixed(3)}</Td>
                <Td>
                  {website ? (
                    <a
                      href={website.startsWith('http') ? website : `https://${website}`}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="flex items-center gap-1 text-orange hover:underline text-xs"
                    >
                      <ExternalLink size={11} />
                      {website.replace(/^https?:\/\//, '').slice(0, 30)}
                    </a>
                  ) : (
                    <span className="text-ink-muted">—</span>
                  )}
                </Td>
              </Tr>
            )
          })}
        </Tbody>
      </Table>

      {totalPages > 1 && (
        <div className="flex items-center gap-1 justify-center">
          {Array.from({ length: Math.min(totalPages, 7) }, (_, i) => i + 1).map((p) => (
            <button
              key={p}
              onClick={() => onPageChange(p)}
              className={`w-8 h-8 text-xs rounded-input transition-colors ${
                p === page
                  ? 'bg-orange text-white font-medium'
                  : 'text-ink-secondary hover:bg-warm-beige'
              }`}
            >
              {p}
            </button>
          ))}
        </div>
      )}
    </div>
  )
}
