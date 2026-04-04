import { PlusSignIcon } from '@/lib/icons'
import type { DocsHelpcenterTranslationState } from '@/lib/docsTypes'

const STATUS_META: Record<DocsHelpcenterTranslationState, { label: string; className: string; icon?: boolean }> = {
  missing: {
    label: 'Add',
    className: 'text-muted-foreground',
    icon: true,
  },
  draft: {
    label: 'Draft',
    className: 'text-amber-600 dark:text-amber-400',
  },
  published: {
    label: 'Published',
    className: 'text-emerald-600 dark:text-emerald-400',
  },
  needs_review: {
    label: 'Needs review',
    className: 'text-blue-600 dark:text-blue-400',
  },
}

export function TranslationStatusBadge({ state }: { state: DocsHelpcenterTranslationState }) {
  const meta = STATUS_META[state]
  return (
    <span className={`inline-flex items-center text-[11px] font-medium ${meta.className}`}>
      {meta.icon && <PlusSignIcon className="h-3 w-3 mr-0.5" />}
      {meta.label}
    </span>
  )
}
