'use client'
import { AreaChart, Area, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer } from 'recharts'

interface ThroughputPoint {
  label: string
  records: number
}

export function ThroughputArea({ data }: { data: ThroughputPoint[] }) {
  return (
    <ResponsiveContainer width="100%" height={160}>
      <AreaChart data={data} margin={{ top: 4, right: 4, left: -20, bottom: 0 }}>
        <defs>
          <linearGradient id="throughputGrad" x1="0" y1="0" x2="0" y2="1">
            <stop offset="5%" stopColor="#E85D04" stopOpacity={0.15} />
            <stop offset="95%" stopColor="#E85D04" stopOpacity={0} />
          </linearGradient>
        </defs>
        <CartesianGrid strokeDasharray="3 3" stroke="#E8E0D0" />
        <XAxis dataKey="label" tick={{ fontSize: 11, fill: '#9B9088' }} />
        <YAxis tick={{ fontSize: 11, fill: '#9B9088' }} />
        <Tooltip
          contentStyle={{ fontSize: 12, border: '1px solid #E8E0D0', borderRadius: 8 }}
          formatter={(v: number) => [v.toLocaleString(), 'records']}
        />
        <Area
          type="monotone"
          dataKey="records"
          stroke="#E85D04"
          strokeWidth={2}
          fill="url(#throughputGrad)"
        />
      </AreaChart>
    </ResponsiveContainer>
  )
}
