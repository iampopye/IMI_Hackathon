'use client'
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, Legend, ResponsiveContainer } from 'recharts'

interface LatencyPoint {
  label: string
  p50: number
  p95: number
  p99: number
}

interface LatencyChartProps {
  data: LatencyPoint[]
}

export function LatencyChart({ data }: LatencyChartProps) {
  return (
    <ResponsiveContainer width="100%" height={200}>
      <LineChart data={data} margin={{ top: 4, right: 4, left: -20, bottom: 0 }}>
        <CartesianGrid strokeDasharray="3 3" stroke="#E8E0D0" />
        <XAxis dataKey="label" tick={{ fontSize: 11, fill: '#9B9088' }} />
        <YAxis tick={{ fontSize: 11, fill: '#9B9088' }} unit="ms" />
        <Tooltip
          contentStyle={{ fontSize: 12, border: '1px solid #E8E0D0', borderRadius: 8, background: '#fff' }}
          formatter={(v: number) => [`${v.toFixed(1)}ms`]}
        />
        <Legend iconType="circle" wrapperStyle={{ fontSize: 11 }} />
        <Line type="monotone" dataKey="p50" stroke="#E85D04" dot={false} strokeWidth={2} name="p50" />
        <Line type="monotone" dataKey="p95" stroke="#B45309" dot={false} strokeWidth={1.5} strokeDasharray="4 2" name="p95" />
        <Line type="monotone" dataKey="p99" stroke="#C0392B" dot={false} strokeWidth={1.5} strokeDasharray="2 2" name="p99" />
      </LineChart>
    </ResponsiveContainer>
  )
}
