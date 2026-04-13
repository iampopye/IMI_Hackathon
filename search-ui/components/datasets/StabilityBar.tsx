interface StabilityBarProps {
  score: number
}

export function StabilityBar({ score }: StabilityBarProps) {
  const pct = Math.min(100, Math.round(score * 100))
  const color = score >= 0.70 ? 'bg-forest' : score >= 0.50 ? 'bg-amber' : 'bg-danger'

  return (
    <div className="flex items-center gap-2">
      <div className="flex-1 h-1.5 bg-warm-border rounded-pill overflow-hidden">
        <div className={`h-full rounded-pill ${color} transition-all`} style={{ width: `${pct}%` }} />
      </div>
      <span className="text-xs font-mono text-ink-secondary w-9 text-right">{score.toFixed(2)}</span>
    </div>
  )
}
