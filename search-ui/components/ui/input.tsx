import { clsx } from 'clsx'

interface InputProps extends React.InputHTMLAttributes<HTMLInputElement> {}

export function Input({ className, ...props }: InputProps) {
  return (
    <input
      {...props}
      className={clsx(
        'w-full px-3 py-2 text-sm border border-warm-border rounded-input bg-white text-ink placeholder:text-ink-muted',
        'focus:outline-none focus:ring-2 focus:ring-orange/30 focus:border-orange-border transition-colors',
        className,
      )}
    />
  )
}

interface SelectProps extends React.SelectHTMLAttributes<HTMLSelectElement> {}

export function Select({ className, children, ...props }: SelectProps) {
  return (
    <select
      {...props}
      className={clsx(
        'px-3 py-2 text-sm border border-warm-border rounded-input bg-white text-ink',
        'focus:outline-none focus:ring-2 focus:ring-orange/30 focus:border-orange-border transition-colors',
        className,
      )}
    >
      {children}
    </select>
  )
}
