import { Badge } from '@/components/ui/badge'
import type { DocsHelpcenterTranslationState } from '@/lib/docsTypes'

const STATUS_META: Record<DocsHelpcenterTranslationState, { label: string; className: string }> = {
  missing: {
    label: 'Missing',
    className: 'border-dashed border-border/80 bg-transparent text-muted-foreground',
  },
  draft: {
    label: 'Draft',
    className: 'border-amber-500/30 bg-amber-500/10 text-amber-700 dark:text-amber-300',
  },
  published: {
    label: 'Published',
    className: 'border-emerald-500/30 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300',
  },
  needs_review: {
    label: 'Needs review',
    className: 'border-blue-500/30 bg-blue-500/10 text-blue-700 dark:text-blue-300',
  },
}

export function TranslationStatusBadge({ state }: { state: DocsHelpcenterTranslationState }) {
  const meta = STATUS_META[state]
  return (
    <Badge variant="outline" className={`h-6 rounded-full px-2.5 text-[11px] font-medium ${meta.className}`}>
      {meta.label}
    </Badge>
  )
}
