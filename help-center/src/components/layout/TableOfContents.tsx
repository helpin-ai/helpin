import { TableOfContents as TocIcon } from 'lucide-react'
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
      <div className="pt-10 pb-6 pr-4 pl-1">
        <h4 className="mb-3 flex items-center gap-1.5 text-[13px] font-bold text-muted-foreground">
          <TocIcon size={14} />
          On this page
        </h4>
        <ul className="space-y-0.5">
          {items.map((item) => (
            <li key={item.id}>
              <a
                href={`#${item.id}`}
                onClick={(e) => {
                  e.preventDefault()
                  document.getElementById(item.id)?.scrollIntoView({ behavior: 'smooth' })
                }}
                className={cn(
                  'block text-[13px] py-1 transition-colors border-l-2',
                  item.level === 3 ? 'pl-5' : 'pl-3',
                  activeId === item.id
                    ? 'border-primary text-primary font-medium'
                    : 'border-transparent text-muted-foreground hover:text-foreground',
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
