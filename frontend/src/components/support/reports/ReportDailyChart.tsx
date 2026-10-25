import { useMemo, useState } from 'react'
import type { SupportReportDay } from '@/lib/supportReportTypes'
import { formatReportDuration } from '@/lib/supportReportFormat'
import { cn } from '@/lib/utils'

interface ReportDailyChartProps {
  days: SupportReportDay[]
  className?: string
}

const CHART_HEIGHT = 140

function shortDate(date: string): string {
  const [year, month, day] = date.split('-').map(Number)
  return new Date(year, month - 1, day).toLocaleDateString(undefined, { month: 'short', day: 'numeric' })
}

/**
 * Grouped daily bars: conversations created and resolved. The axis labels
 * thin out on long ranges so they never overlap.
 */
export function ReportDailyChart({ days, className }: ReportDailyChartProps) {
  const [hovered, setHovered] = useState<number | null>(null)
  const max = useMemo(
    () => Math.max(1, ...days.map((day) => Math.max(day.conversations_created, day.conversations_resolved))),
    [days],
  )
  if (days.length === 0) {
    return <p className="py-8 text-center text-sm text-quiet-text-tertiary">No days in this period.</p>
  }
  const labelEvery = Math.max(1, Math.ceil(days.length / 10))
  const active = hovered === null ? null : days[hovered]

  return (
    <div className={cn('space-y-2', className)}>
      <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-[12px] text-quiet-text-tertiary">
        <span className="inline-flex items-center gap-1.5"><span className="h-2 w-2 rounded-sm bg-foreground/70" />Created</span>
        <span className="inline-flex items-center gap-1.5"><span className="h-2 w-2 rounded-sm bg-emerald-500/80" />Resolved</span>
        <span className="ml-auto min-h-[18px] tabular-nums" aria-live="polite">
          {active
            ? `${shortDate(active.date)}: ${active.conversations_created} created, ${active.conversations_resolved} resolved` +
              (active.median_first_human_response_seconds !== null
                ? `, first response ${formatReportDuration(active.median_first_human_response_seconds)}`
                : '')
            : null}
        </span>
      </div>
      <div
        className="grid items-end gap-[2px]"
        style={{ gridTemplateColumns: `repeat(${days.length}, minmax(0, 1fr))`, height: CHART_HEIGHT }}
        role="group"
        aria-label="Conversations created and resolved per day"
        onMouseLeave={() => setHovered(null)}
      >
        {days.map((day, index) => (
          <div
            key={day.date}
            className={cn('flex h-full items-end justify-center gap-[1px] rounded-sm px-[1px] focus-visible:outline-2 focus-visible:outline-quiet-text-primary', hovered === index && 'bg-quiet-row-hover')}
            onMouseEnter={() => setHovered(index)}
            onFocus={() => setHovered(index)}
            onBlur={() => setHovered(null)}
            tabIndex={0}
            role="img"
            aria-label={`${shortDate(day.date)}: ${day.conversations_created} created, ${day.conversations_resolved} resolved`}
            data-testid="report-day"
          >
            <div
              className="w-1/2 max-w-[14px] rounded-t-[2px] bg-foreground/70"
              style={{ height: `${(day.conversations_created / max) * 100}%`, minHeight: day.conversations_created ? 2 : 0 }}
            />
            <div
              className="w-1/2 max-w-[14px] rounded-t-[2px] bg-emerald-500/80"
              style={{ height: `${(day.conversations_resolved / max) * 100}%`, minHeight: day.conversations_resolved ? 2 : 0 }}
            />
          </div>
        ))}
      </div>
      <div className="grid gap-[2px] text-[11px] text-quiet-text-tertiary" style={{ gridTemplateColumns: `repeat(${days.length}, minmax(0, 1fr))` }}>
        {days.map((day, index) => (
          <span key={day.date} className="overflow-visible whitespace-nowrap text-center">
            {index % labelEvery === 0 ? shortDate(day.date) : ''}
          </span>
        ))}
      </div>
    </div>
  )
}
