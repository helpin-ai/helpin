import type { SupportCoverageGapListItem } from '@/lib/supportCoverageTypes'
import {
  GAP_KIND_COLORS,
  V1_GAP_TYPE_LABELS,
  resolveGapKindDisplay,
} from '@/lib/supportCoverageTypes'
import { cn, timeAgo } from '@/lib/utils'

const ROW_GRID_FULL =
  'grid w-full grid-cols-1 items-center gap-2 px-2 py-4 text-left sm:grid-cols-[minmax(0,1fr)_150px_125px_90px] sm:gap-4'
const ROW_GRID_COMPACT =
  'grid w-full grid-cols-1 items-center gap-2 px-3 py-3 text-left sm:grid-cols-[minmax(0,1fr)_120px_90px] sm:gap-3'

function KindDot({ gapKind }: { gapKind: string }) {
  const kind = resolveGapKindDisplay(gapKind)
  const colors = GAP_KIND_COLORS[kind] ?? GAP_KIND_COLORS.content
  return (
    <span
      className={cn(
        'inline-block h-2.5 w-2.5 shrink-0 rounded-full',
        colors.dot,
      )}
    />
  )
}

export function GapList({
  gaps,
  selectedGapId,
  onSelect,
  compact = false,
  emptyTitle = 'No coverage gaps found.',
  emptyDescription = 'Gaps appear when AI support encounters issues it cannot resolve.',
}: {
  gaps: SupportCoverageGapListItem[]
  selectedGapId?: string
  onSelect: (gapId: string) => void
  compact?: boolean
  emptyTitle?: string
  emptyDescription?: string
}) {
  if (gaps.length === 0) {
    return (
      <div className="space-y-2 py-10 text-quiet-text-tertiary">
        <p className="text-sm font-medium text-foreground">{emptyTitle}</p>
        <p className="mt-1 text-xs text-muted-foreground/70">
          {emptyDescription}
        </p>
      </div>
    )
  }

  return (
    <div className="min-w-0 border-y border-quiet-divider-strong">
      {!compact && (
        <div
          className={cn(
            ROW_GRID_FULL,
            'hidden border-b border-quiet-divider-strong py-2 text-xs font-medium text-quiet-text-tertiary sm:grid',
          )}
        >
          <span>Gap</span>
          <span>Needed improvement</span>
          <span>Impact · 30d</span>
          <span>Last seen</span>
        </div>
      )}
      <div className="divide-y divide-border/40">
        {gaps.map((gap) => {
          const title = gap.canonical_title || gap.title
          const preview = gap.title !== title ? gap.title : gap.topic_title
          const improvement =
            gap.gap_kind === 'data'
              ? 'Customer data'
              : gap.gap_kind === 'action'
                ? 'Action or workflow'
                : gap.gap_kind === 'policy'
                  ? 'Policy or escalation'
                  : (V1_GAP_TYPE_LABELS[gap.v1_gap_type] ?? gap.v1_gap_type)
          const flags = [
            gap.split_review_needed ? 'Review split' : '',
            gap.recurrence_reopened ? 'Reopened' : '',
          ].filter(Boolean)
          return (
            <button
              key={gap.id}
              type="button"
              onClick={() => onSelect(gap.id)}
              aria-current={selectedGapId === gap.id ? true : undefined}
              className={cn(
                compact ? ROW_GRID_COMPACT : ROW_GRID_FULL,
                'transition-colors hover:bg-quiet-row-hover focus-visible:outline-2 focus-visible:outline-ring',
                selectedGapId === gap.id && 'bg-muted/60',
              )}
            >
              <div className="min-w-0">
                <p className="line-clamp-2 break-words text-sm font-semibold">
                  {title}
                </p>
                {preview && (
                  <p className="mt-0.5 line-clamp-1 text-xs text-muted-foreground">
                    {preview}
                  </p>
                )}
                {!compact && flags.length > 0 && (
                  <p className="mt-1 line-clamp-1 text-[11px] font-medium text-amber-700">
                    {flags.join(' · ')}
                  </p>
                )}
              </div>

              {!compact ? (
                <span className="flex items-center gap-1.5 text-xs font-medium text-muted-foreground">
                  <KindDot gapKind={gap.gap_kind} />
                  {improvement}
                </span>
              ) : (
                <span className="flex items-center gap-1.5 text-xs text-muted-foreground">
                  <KindDot gapKind={gap.gap_kind} />
                  <span className="truncate">{improvement}</span>
                </span>
              )}

              {!compact && (
                <span className="text-xs tabular-nums text-muted-foreground">
                  {gap.conversations_30d !== undefined && gap.conversations_30d > 0 ? `${gap.conversations_30d} ${gap.conversations_30d === 1 ? 'conversation' : 'conversations'}` : `${gap.evidence_records_30d ?? gap.evidence_30d} records`}
                  <span className="sm:hidden"> in 30 days</span>
                </span>
              )}

              <span className="truncate text-xs text-muted-foreground">
                {timeAgo(gap.last_seen_at)}
              </span>
            </button>
          )
        })}
      </div>
    </div>
  )
}
