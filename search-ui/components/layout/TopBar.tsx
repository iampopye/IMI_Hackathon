'use client'

import { useState, useEffect } from 'react'

interface TopBarProps {
  title: string
  live?: boolean
  onToggleLive?: () => void
}

export function TopBar({ title, live, onToggleLive }: TopBarProps) {
  const [mounted, setMounted] = useState(false)
  useEffect(() => { setMounted(true) }, [])

  const now = new Date()
  const dateStr = mounted ? now.toLocaleDateString('en-IN', { day: '2-digit', month: 'short' }) : ''
  const timeStr = mounted ? now.toLocaleTimeString('en-IN', { hour: '2-digit', minute: '2-digit', hour12: false }) : ''

  return (
    <header className="h-14 border-b border-warm-border bg-white flex items-center px-6 gap-4 shrink-0">
      <h1 className="font-display text-base font-semibold text-ink flex-1">{title}</h1>

      {onToggleLive !== undefined && (
        <button
          onClick={onToggleLive}
          className={`flex items-center gap-1.5 text-xs font-medium px-2.5 py-1 rounded-pill transition-colors ${
            live ? 'bg-forest-light text-forest' : 'bg-warm-beige text-ink-secondary'
          }`}
        >
          <span className={`w-2 h-2 rounded-full ${live ? 'bg-forest animate-pulse' : 'bg-ink-muted'}`} />
          {live ? 'Live' : 'Paused'}
        </button>
      )}

      <span className="text-xs text-ink-muted font-mono">
        {dateStr} &nbsp; {timeStr}
      </span>
    </header>
  )
}
