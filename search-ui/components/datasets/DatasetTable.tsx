import Link from 'next/link'
import { Table, Thead, Th, Tbody, Tr, Td } from '@/components/ui/table'
import { Badge } from '@/components/ui/badge'
import { StabilityBar } from './StabilityBar'
import type { Dataset } from '@/lib/types'

function tierBadge(count: number) {
  if (count >= 5_000_000) return <Badge variant="default">● Large</Badge>
  if (count >= 100_000)   return <Badge variant="warning">● Medium</Badge>
  return                         <Badge variant="success">● Small</Badge>
}

function stateBadge(state: string) {
  if (state === 'STABLE')   return <Badge variant="success">✓ Stable</Badge>
  if (state === 'VOLATILE') return <Badge variant="warning">~ Volatile</Badge>
  return <Badge variant="default">{state}</Badge>
}

export function DatasetTable({ datasets }: { datasets: Dataset[] }) {
  return (
    <Table>
      <Thead>
        <tr>
          <Th>Name</Th>
          <Th>Records</Th>
          <Th>Tier</Th>
          <Th className="w-40">Stability</Th>
          <Th>Status</Th>
        </tr>
      </Thead>
      <Tbody>
        {datasets.map((d) => (
          <Tr key={d.id}>
            <Td>
              <Link href={`/datasets/${d.id}`} className="font-medium text-orange hover:underline">
                {d.name}
              </Link>
            </Td>
            <Td className="font-mono">{d.record_count.toLocaleString()}</Td>
            <Td>{tierBadge(d.record_count)}</Td>
            <Td><StabilityBar score={d.stability_score} /></Td>
            <Td>{stateBadge(d.state)}</Td>
          </Tr>
        ))}
        {datasets.length === 0 && (
          <Tr>
            <Td colSpan={5} className="text-center text-ink-muted py-8">
              No datasets yet. Upload one above.
            </Td>
          </Tr>
        )}
      </Tbody>
    </Table>
  )
}
