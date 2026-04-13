const ENGINE_STYLES: Record<string, { dot: string; bg: string; text: string }> = {
  bleve_memory:      { dot: 'bg-orange',  bg: 'bg-orange-light',   text: 'text-orange'  },
  bleve_file:        { dot: 'bg-amber',   bg: 'bg-amber-light',    text: 'text-amber'   },
  elasticsearch:     { dot: 'bg-ink',     bg: 'bg-warm-beige',     text: 'text-ink'     },
  postgres_fallback: { dot: 'bg-danger',  bg: 'bg-danger-light',   text: 'text-danger'  },
}

function resolveEngine(engine: string) {
  if (engine.includes('mem_cache') || engine.includes('redis_cache') || engine.includes('cache')) {
    return { dot: 'bg-forest', bg: 'bg-forest-light', text: 'text-forest', label: 'cache' }
  }
  for (const [key, style] of Object.entries(ENGINE_STYLES)) {
    if (engine.startsWith(key)) return { ...style, label: key.replace('_', ' ') }
  }
  return { dot: 'bg-ink-muted', bg: 'bg-warm-beige', text: 'text-ink-secondary', label: engine }
}

export function EngineBadge({ engine }: { engine: string }) {
  const { dot, bg, text, label } = resolveEngine(engine)
  return (
    <span className={`inline-flex items-center gap-1.5 px-2 py-0.5 rounded-pill text-xs font-medium ${bg} ${text}`}>
      <span className={`w-1.5 h-1.5 rounded-full ${dot}`} />
      {label}
    </span>
  )
}
