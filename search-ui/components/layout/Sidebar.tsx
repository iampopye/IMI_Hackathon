'use client'
import Link from 'next/link'
import { usePathname } from 'next/navigation'
import { LayoutDashboard, Search, Database, BarChart2, Activity } from 'lucide-react'
import { clsx } from 'clsx'

const nav = [
  { href: '/',            label: 'Overview',     icon: LayoutDashboard },
  { href: '/search',      label: 'Search',       icon: Search          },
  { href: '/datasets',    label: 'Datasets',     icon: Database        },
  { href: '/performance', label: 'Performance',  icon: BarChart2       },
  { href: '/system',      label: 'System',       icon: Activity        },
]

export function Sidebar() {
  const path = usePathname()

  return (
    <aside className="w-56 shrink-0 border-r border-warm-border bg-warm-white flex flex-col py-6">
      <div className="px-5 mb-8">
        <span className="font-display text-lg font-semibold text-ink">Search System</span>
      </div>

      <nav className="flex flex-col gap-0.5 px-3">
        {nav.map(({ href, label, icon: Icon }) => {
          const active = href === '/' ? path === '/' : path.startsWith(href)
          return (
            <Link
              key={href}
              href={href}
              className={clsx(
                'flex items-center gap-3 px-3 py-2.5 rounded-input text-sm font-medium transition-colors',
                active
                  ? 'border-l-2 border-orange pl-[10px] bg-orange-light text-orange'
                  : 'text-ink-secondary hover:text-ink hover:bg-warm-beige',
              )}
            >
              <Icon size={16} />
              {label}
            </Link>
          )
        })}
      </nav>
    </aside>
  )
}
