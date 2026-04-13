import { clsx } from 'clsx'

interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: 'primary' | 'ghost'
  size?: 'sm' | 'md'
}

export function Button({ variant = 'primary', size = 'md', className, children, ...props }: ButtonProps) {
  return (
    <button
      {...props}
      className={clsx(
        'inline-flex items-center gap-2 font-medium rounded-input transition-colors disabled:opacity-50',
        variant === 'primary' && 'bg-orange text-white hover:bg-orange-hover',
        variant === 'ghost'   && 'bg-transparent text-ink-secondary hover:text-ink hover:bg-warm-beige',
        size === 'md' && 'px-4 py-2 text-sm',
        size === 'sm' && 'px-3 py-1.5 text-xs',
        className,
      )}
    >
      {children}
    </button>
  )
}
