import { clsx } from 'clsx'

interface CardProps {
  children: React.ReactNode
  className?: string
}

export function Card({ children, className }: CardProps) {
  return (
    <div className={clsx('bg-white border border-warm-border rounded-card shadow-card', className)}>
      {children}
    </div>
  )
}

export function CardHeader({ children, className }: CardProps) {
  return (
    <div className={clsx('px-5 py-4 border-b border-warm-border', className)}>
      {children}
    </div>
  )
}

export function CardTitle({ children, className }: CardProps) {
  return (
    <h2 className={clsx('text-sm font-semibold text-ink', className)}>{children}</h2>
  )
}

export function CardContent({ children, className }: CardProps) {
  return <div className={clsx('px-5 py-4', className)}>{children}</div>
}
