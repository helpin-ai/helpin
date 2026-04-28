import { Badge } from '@/components/ui/badge'
import { FileSearchIcon } from '@/lib/icons'
import type { SupportCoverageGapListItem } from '@/lib/supportCoverageTypes'
import { V1_GAP_TYPE_LABELS } from '@/lib/supportCoverageTypes'
import { cn, timeAgo } from '@/lib/utils'
import { GapImpactBadge } from './GapImpactBadge'

const GAP_KIND_LABELS: Record<string, string> = {
  content: 'Content gap',
  data: 'Data gap',
  action: 'Action gap',
}

export function GapList({
  gaps,
  selectedGapId,
  onSelect,
}: {
  gaps: SupportCoverageGapListItem[]
  selectedGapId?: string
  onSelect: (gapId: string) => void
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
                'grid w-full grid-cols-[1fr_auto] gap-3 px-4 py-3.5 text-left transition-colors hover:bg-muted/35',
                selectedGapId === gap.id && 'bg-muted/60'
              )}
            >
              <div className="min-w-0">
                <div className="mb-1.5 flex flex-wrap items-center gap-1.5">
                  <Badge
                    variant="secondary"
                    className="border border-border/50 bg-muted/50 text-[11px] text-muted-foreground"
                  >
                    {GAP_KIND_LABELS[gap.gap_kind] ?? gap.gap_kind}
                  </Badge>
                  <Badge
                    variant="secondary"
                    className="border border-border/50 bg-background text-[11px] text-muted-foreground"
                  >
                    {V1_GAP_TYPE_LABELS[gap.v1_gap_type] ?? gap.v1_gap_type}
                  </Badge>
                </div>
                <p className="truncate text-sm font-semibold">{title}</p>
                {preview && (
                  <p className="mt-1 line-clamp-1 text-xs text-muted-foreground">{preview}</p>
                )}
              </div>

              <div className="flex shrink-0 flex-col items-end gap-2">
                <GapImpactBadge tier={gap.impact_tier} />
                <span className="text-xs text-muted-foreground">
                  {gap.evidence_30d} in 30d · {timeAgo(gap.last_seen_at)}
                </span>
              </div>
            </button>
          )
        })}
      </div>
    </div>
  )
}
