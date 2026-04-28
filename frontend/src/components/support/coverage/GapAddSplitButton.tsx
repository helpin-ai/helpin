import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { ArrowDown02Icon, MagicWand01Icon } from '@/lib/icons'
import type { SupportCoverageGapDetail, SupportGapSuggestion } from '@/lib/supportCoverageTypes'
import { cn } from '@/lib/utils'

export type GapAddRoute = 'create_article' | 'update_article'

function primaryLabel(gap: SupportCoverageGapDetail, suggestion?: SupportGapSuggestion | null) {
  const route = suggestion?.suggestion_type
  if (route === 'update_article') {
    const articleTitle = gap.related_articles[0]?.article_title
    return articleTitle ? `Add to "${articleTitle}"` : 'Add to article'
  }
  return 'Create new article'
}

export function GapAddSplitButton({
  gap,
  suggestion,
  disabled,
  onPrimary,
  onOverride,
  className,
}: {
  gap: SupportCoverageGapDetail
  suggestion?: SupportGapSuggestion | null
  disabled?: boolean
  onPrimary: () => void
  onOverride: (route: GapAddRoute, targetDocumentId?: string) => void
  className?: string
}) {
  const targetArticle = gap.related_articles[0]

  return (
    <div className={cn('inline-flex overflow-hidden rounded-md shadow-sm', className)}>
      <button
        type="button"
        onClick={onPrimary}
        disabled={disabled}
        className="inline-flex items-center gap-2 bg-primary px-3 py-2 text-xs font-semibold text-primary-foreground transition-colors hover:bg-primary/90 disabled:opacity-50"
      >
        <MagicWand01Icon className="h-3.5 w-3.5" />
        {primaryLabel(gap, suggestion)}
      </button>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <button
            type="button"
            disabled={disabled}
            aria-label="Choose add route"
            className="border-l border-primary-foreground/20 bg-primary px-2 text-primary-foreground transition-colors hover:bg-primary/90 disabled:opacity-50"
          >
            <ArrowDown02Icon className="h-3.5 w-3.5" />
          </button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="w-56">
          {targetArticle && (
            <DropdownMenuItem onClick={() => onOverride('update_article', targetArticle.document_id)}>
              Add to "{targetArticle.article_title || 'linked article'}"
            </DropdownMenuItem>
          )}
          {targetArticle && <DropdownMenuSeparator />}
          <DropdownMenuItem onClick={() => onOverride('create_article')}>
            Create new article
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  )
}
