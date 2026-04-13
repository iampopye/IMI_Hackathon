import { clsx } from 'clsx'

type Variant = 'default' | 'success' | 'warning' | 'danger' | 'orange'

interface BadgeProps {
  children: React.ReactNode
  variant?: Variant
  className?: string
}

const variants: Record<Variant, string> = {
  default:  'bg-warm-beige text-ink-secondary',
  success:  'bg-forest-light text-forest',
  warning:  'bg-amber-light text-amber',
  danger:   'bg-danger-light text-danger',
  orange:   'bg-orange-light text-orange',
}

export function Badge({ children, variant = 'default', className }: BadgeProps) {
  return (
    <span className={clsx('inline-flex items-center px-2 py-0.5 rounded-pill text-xs font-medium', variants[variant], className)}>
      {children}
    </span>
  )
}
