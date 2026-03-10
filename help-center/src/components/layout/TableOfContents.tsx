import { cn } from '@/lib/utils'
import type { TocItem } from '@/lib/toc'

interface TableOfContentsProps {
  items: TocItem[]
  activeId?: string
}

export function TableOfContents({ items, activeId }: TableOfContentsProps) {
  if (items.length === 0) return null

  return (
    <aside
      className="sticky top-[var(--hc-header-height)] hidden xl:block shrink-0 overflow-y-auto"
      style={{
        width: 'var(--hc-toc-width)',
        height: 'calc(100vh - var(--hc-header-height))',
      }}
    >
      <div className="py-6 px-4">
        <h4
          className="mb-3 text-xs font-semibold uppercase tracking-wider"
          style={{ color: 'var(--hc-text-muted)' }}
        >
          On this page
        </h4>
        <ul className="space-y-1">
          {items.map((item) => (
            <li key={item.id}>
              <a
                href={`#${item.id}`}
                className={cn(
                  'block text-sm py-1 transition-colors border-l-2',
                  item.level === 3 ? 'pl-6' : 'pl-3',
                  activeId === item.id
                    ? 'border-[var(--hc-accent)] text-[var(--hc-accent)] font-medium'
                    : 'border-transparent text-[var(--hc-text-secondary)] hover:text-[var(--hc-text)] hover:border-[var(--hc-border)]',
                )}
              >
                {item.text}
              </a>
            </li>
          ))}
        </ul>
      </div>
    </aside>
  )
}
