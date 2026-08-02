import { clsx } from 'clsx'

export function Table({ children, className }: { children: React.ReactNode; className?: string }) {
  return (
    <div className={clsx('w-full overflow-x-auto', className)}>
      <table className="w-full text-sm">{children}</table>
    </div>
  )
}

export function Thead({ children }: { children: React.ReactNode }) {
  return <thead className="border-b border-warm-border">{children}</thead>
}

export function Th({ children, className, ...props }: React.ThHTMLAttributes<HTMLTableCellElement>) {
  return (
    <th
      className={clsx('px-4 py-2.5 text-left text-xs font-semibold text-ink-secondary uppercase tracking-wide', className)}
      {...props}
    >
      {children}
    </th>
  )
}

export function Tbody({ children }: { children: React.ReactNode }) {
  return <tbody className="divide-y divide-warm-border">{children}</tbody>
}

export function Tr({ children, className, ...props }: React.HTMLAttributes<HTMLTableRowElement>) {
  return (
    <tr className={clsx('hover:bg-warm-beige/40 transition-colors', className)} {...props}>
      {children}
    </tr>
  )
}

export function Td({ children, className, ...props }: React.TdHTMLAttributes<HTMLTableCellElement>) {
  return (
    <td className={clsx('px-4 py-3 text-sm text-ink', className)} {...props}>
      {children}
    </td>
  )
}
