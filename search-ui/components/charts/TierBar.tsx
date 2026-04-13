'use client'
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer } from 'recharts'

interface TierBarProps {
  small: number
  medium: number
  large: number
}

export function TierBar({ small, medium, large }: TierBarProps) {
  const data = [
    { name: 'Small',  count: small,  color: '#2D6A4F' },
    { name: 'Medium', count: medium, color: '#B45309' },
    { name: 'Large',  count: large,  color: '#1A1A1A' },
  ]

  return (
    <ResponsiveContainer width="100%" height={160}>
      <BarChart data={data} layout="vertical" margin={{ top: 4, right: 16, left: 0, bottom: 0 }}>
        <CartesianGrid strokeDasharray="3 3" stroke="#E8E0D0" horizontal={false} />
        <XAxis type="number" tick={{ fontSize: 11, fill: '#9B9088' }} />
        <YAxis type="category" dataKey="name" tick={{ fontSize: 11, fill: '#9B9088' }} width={50} />
        <Tooltip
          formatter={(v: number) => [`${v} datasets`]}
          contentStyle={{ fontSize: 12, border: '1px solid #E8E0D0', borderRadius: 8 }}
        />
        <Bar dataKey="count" radius={[0, 4, 4, 0]}>
          {data.map((entry, i) => (
            <rect key={i} fill={entry.color} />
          ))}
        </Bar>
      </BarChart>
    </ResponsiveContainer>
  )
}
