import { FileSearchIcon } from '@/lib/icons'
import type { SupportCoverageGapListItem } from '@/lib/supportCoverageTypes'
import { GAP_KIND_LABELS, GAP_KIND_COLORS, resolveGapKindDisplay } from '@/lib/supportCoverageTypes'
import { cn, timeAgo } from '@/lib/utils'
import { GapImpactBadge } from './GapImpactBadge'

const ROW_GRID_FULL =
  'grid w-full grid-cols-[minmax(0,1fr)_100px_100px_90px_90px] items-center gap-4 px-4 py-3 text-left'
const ROW_GRID_COMPACT =
  'grid w-full grid-cols-[minmax(0,1fr)_80px_90px] items-center gap-3 px-3 py-3 text-left'

function KindBadge({ gapKind }: { gapKind: string }) {
  const kind = resolveGapKindDisplay(gapKind)
  const colors = GAP_KIND_COLORS[kind] ?? GAP_KIND_COLORS.content
  const label = GAP_KIND_LABELS[gapKind] ?? gapKind
  return (
    <span
      className={cn(
        'inline-flex items-center rounded-full border px-2 py-0.5 text-[10px] font-medium',
        colors.bg,
        colors.text,
        colors.border
      )}
    >
      {label}
    </span>
  )
}

export function GapList({
  gaps,
  selectedGapId,
  onSelect,
  compact = false,
}: {
  gaps: SupportCoverageGapListItem[]
  selectedGapId?: string
  onSelect: (gapId: string) => void
  compact?: boolean
}) {
  if (gaps.length === 0) {
    return (
      <div className="flex flex-col items-center py-14 text-muted-foreground">
        <FileSearchIcon className="mb-3 h-10 w-10 text-muted-foreground/30" />
        <p className="text-sm font-medium text-foreground">No coverage gaps found.</p>
        <p className="mt-1 text-xs text-muted-foreground/70">
          Gaps appear when AI support encounters issues it cannot resolve.
        </p>
      </div>
    )
  }

  return (
    <div className="overflow-hidden rounded-xl border border-border/60 bg-card">
      {!compact && (
        <div
          className={cn(
            ROW_GRID_FULL,
            'border-b border-border/40 bg-muted/20 py-2 text-[10px] font-medium uppercase tracking-wide text-muted-foreground/70'
          )}
        >
          <span>Gap</span>
          <span>Type</span>
          <span>Impact</span>
          <span>Evidence</span>
          <span>Last seen</span>
        </div>
      )}
      <div className="divide-y divide-border/40">
        {gaps.map((gap) => {
          const title = gap.canonical_title || gap.title
          const preview = gap.title !== title ? gap.title : gap.topic_title
          return (
            <button
              key={gap.id}
              type="button"
              onClick={() => onSelect(gap.id)}
              className={cn(
                compact ? ROW_GRID_COMPACT : ROW_GRID_FULL,
                'transition-colors hover:bg-muted/35',
                selectedGapId === gap.id && 'bg-muted/60'
              )}
            >
              <div className="min-w-0">
                <p className="truncate text-sm font-semibold">{title}</p>
                {preview && (
                  <p className="mt-0.5 line-clamp-1 text-xs text-muted-foreground">{preview}</p>
                )}
              </div>

              {compact ? (
                <div>
                  <KindBadge gapKind={gap.gap_kind} />
                </div>
              ) : (
                <div>
                  <KindBadge gapKind={gap.gap_kind} />
                </div>
              )}

              {!compact && (
                <div className="flex justify-start">
                  <GapImpactBadge tier={gap.impact_tier} />
                </div>
              )}

              {!compact && (
                <span className="text-xs tabular-nums text-muted-foreground">
                  {gap.evidence_30d} in 30d
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
