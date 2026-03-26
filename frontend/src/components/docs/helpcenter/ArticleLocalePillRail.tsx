import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import type { DocStatus, DocsHelpcenterTranslationState } from '@/lib/docsTypes'
import { TranslationStatusBadge } from './TranslationStatusBadge'

export interface ArticleLocalePillItem {
  locale: string
  shortLabel: string
  isActive: boolean
  isSource?: boolean
  sourceStatus?: DocStatus
  translationState?: DocsHelpcenterTranslationState
}

function SourceStatusBadge({ status }: { status: DocStatus }) {
  const meta = {
    draft: 'text-amber-600 dark:text-amber-400',
    published: 'text-emerald-600 dark:text-emerald-400',
    archived: 'text-muted-foreground',
  } satisfies Record<DocStatus, string>

  return (
    <span className={`text-[11px] font-medium ${meta[status]}`}>
      {status === 'published' ? 'Published' : status === 'archived' ? 'Archived' : 'Draft'}
    </span>
  )
}

export function ArticleLocalePillRail({
  items,
  onSelectLocale,
}: {
  items: ArticleLocalePillItem[]
  onSelectLocale: (locale: string) => void
}) {
  return (
    <div className="flex items-center gap-2 overflow-x-auto pb-1">
      {items.map((item) => (
        <Button
          key={item.locale}
          type="button"
          variant={item.isActive ? 'secondary' : 'outline'}
          className={`h-auto min-h-10 shrink-0 rounded-2xl px-3 py-2 text-left ${item.isActive ? 'bg-foreground/5 border-foreground/15 shadow-[0_0_0_1px_rgba(0,0,0,0.06)]' : 'bg-background/70 opacity-60 hover:opacity-100'}`}
          onClick={() => onSelectLocale(item.locale)}
        >
          <div className="flex items-center gap-2">
            <span className="text-[11px] font-semibold tracking-[0.18em] text-foreground">{item.shortLabel}</span>
            {item.isSource ? (
              <>
                <Badge variant="secondary" className="h-6 rounded-full px-2.5 text-[11px] font-medium">
                  Source
                </Badge>
                <SourceStatusBadge status={item.sourceStatus ?? 'draft'} />
              </>
            ) : (
              <TranslationStatusBadge state={item.translationState ?? 'missing'} />
            )}
          </div>
        </Button>
      ))}
    </div>
  )
}
