'use client'
import { PieChart, Pie, Cell, Tooltip, Legend, ResponsiveContainer } from 'recharts'

const ENGINE_COLORS: Record<string, string> = {
  cache:             '#2D6A4F',
  bleve_memory:      '#E85D04',
  bleve_file:        '#B45309',
  elasticsearch:     '#1A1A1A',
  postgres_fallback: '#C0392B',
}

interface EngineDonutProps {
  data: { engine: string; count: number }[]
}

export function EngineDonut({ data }: EngineDonutProps) {
  const mapped = data.map((d) => ({
    name: d.engine.replace(/_/g, ' '),
    value: d.count,
    color: ENGINE_COLORS[d.engine] ?? '#9B9088',
  }))

  return (
    <ResponsiveContainer width="100%" height={200}>
      <PieChart>
        <Pie
          data={mapped}
          cx="50%"
          cy="50%"
          innerRadius={55}
          outerRadius={80}
          dataKey="value"
          paddingAngle={2}
        >
          {mapped.map((entry, i) => (
            <Cell key={i} fill={entry.color} />
          ))}
        </Pie>
        <Tooltip
          formatter={(v: number, name: string) => [`${v.toLocaleString()} queries`, name]}
          contentStyle={{ fontSize: 12, border: '1px solid #E8E0D0', borderRadius: 8 }}
        />
        <Legend iconType="circle" wrapperStyle={{ fontSize: 11 }} />
      </PieChart>
    </ResponsiveContainer>
  )
}
