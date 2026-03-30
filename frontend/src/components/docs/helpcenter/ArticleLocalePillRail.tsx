import { Check, CircleAlert, PenLine, Plus, Settings2 } from 'lucide-react'
import { QuickTooltip } from '@/components/ui/quick-tooltip'
import type { DocStatus, DocsHelpcenterTranslationState } from '@/lib/docsTypes'
import { getHelpcenterLocaleLabel } from '@/lib/docsTypes'

export interface ArticleLocalePillItem {
  locale: string
  shortLabel: string
  isActive: boolean
  isSource?: boolean
  sourceStatus?: DocStatus
  translationState?: DocsHelpcenterTranslationState
}

const STATUS_ICON: Record<DocsHelpcenterTranslationState, { icon: React.FC<{ className?: string }>; className: string; label: string }> = {
  missing: { icon: Plus, className: 'text-muted-foreground', label: 'Not translated' },
  draft: { icon: PenLine, className: 'text-amber-500', label: 'Draft' },
  published: { icon: Check, className: 'text-emerald-500', label: 'Published' },
  needs_review: { icon: CircleAlert, className: 'text-blue-500', label: 'Needs review' },
}

const SOURCE_ICON: Record<DocStatus, { icon: React.FC<{ className?: string }>; className: string; label: string }> = {
  draft: { icon: PenLine, className: 'text-amber-500', label: 'Draft' },
  published: { icon: Check, className: 'text-emerald-500', label: 'Published' },
  archived: { icon: CircleAlert, className: 'text-muted-foreground', label: 'Archived' },
}

export function ArticleLocalePillRail({
  items,
  onSelectLocale,
  onOpenSettings,
}: {
  items: ArticleLocalePillItem[]
  onSelectLocale: (locale: string) => void
  onOpenSettings?: (locale: string) => void
}) {
  return (
    <div className="flex items-center justify-center gap-1.5 overflow-x-auto pb-1">
      {items.map((item) => {
        const isMissing = !item.isSource && (item.translationState ?? 'missing') === 'missing'
        const meta = item.isSource
          ? SOURCE_ICON[item.sourceStatus ?? 'draft']
          : STATUS_ICON[item.translationState ?? 'missing']
        const StatusIcon = meta.icon
        const tooltipText = item.isSource
          ? `${getHelpcenterLocaleLabel(item.locale)} (Source) · ${meta.label}`
          : `${getHelpcenterLocaleLabel(item.locale)} · ${meta.label}`

        return (
          <QuickTooltip key={item.locale} label={tooltipText}>
            <button
              type="button"
              onClick={() => {
                if (isMissing) {
                  onSelectLocale(item.locale)
                  return
                }
                onSelectLocale(item.locale)
              }}
              className={`group/pill relative flex items-center gap-1.5 rounded-full px-3 py-1.5 text-xs font-semibold tracking-wide transition-all ${
                item.isActive
                  ? 'bg-foreground/5 border border-foreground/15 shadow-[0_0_0_1px_rgba(0,0,0,0.06)]'
                  : 'border border-transparent opacity-60 hover:opacity-100 hover:bg-muted/40'
              }`}
            >
              <StatusIcon className={`h-3 w-3 ${meta.className}`} />
              <span>{item.shortLabel}</span>
              {item.isSource && (
                <span className="text-[9px] font-normal text-muted-foreground tracking-normal">SRC</span>
              )}
              {!item.isSource && !isMissing && onOpenSettings && (
                <button
                  type="button"
                  onClick={(e) => {
                    e.stopPropagation()
                    onOpenSettings(item.locale)
                  }}
                  className="ml-0.5 opacity-0 group-hover/pill:opacity-100 transition-opacity text-muted-foreground hover:text-foreground"
                >
                  <Settings2 className="h-3 w-3" />
                </button>
              )}
            </button>
          </QuickTooltip>
        )
      })}
    </div>
  )
}
