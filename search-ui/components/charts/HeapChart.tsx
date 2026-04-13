'use client'
import { AreaChart, Area, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer } from 'recharts'

interface HeapPoint {
  label: string
  mb: number
}

export function HeapChart({ data }: { data: HeapPoint[] }) {
  return (
    <ResponsiveContainer width="100%" height={160}>
      <AreaChart data={data} margin={{ top: 4, right: 4, left: -20, bottom: 0 }}>
        <defs>
          <linearGradient id="heapGrad" x1="0" y1="0" x2="0" y2="1">
            <stop offset="5%" stopColor="#1A1A1A" stopOpacity={0.1} />
            <stop offset="95%" stopColor="#1A1A1A" stopOpacity={0} />
          </linearGradient>
        </defs>
        <CartesianGrid strokeDasharray="3 3" stroke="#E8E0D0" />
        <XAxis dataKey="label" tick={{ fontSize: 11, fill: '#9B9088' }} />
        <YAxis tick={{ fontSize: 11, fill: '#9B9088' }} unit="MB" />
        <Tooltip
          contentStyle={{ fontSize: 12, border: '1px solid #E8E0D0', borderRadius: 8 }}
          formatter={(v: number) => [`${v}MB`, 'Heap']}
        />
        <Area type="monotone" dataKey="mb" stroke="#1A1A1A" strokeWidth={1.5} fill="url(#heapGrad)" />
      </AreaChart>
    </ResponsiveContainer>
  )
}
