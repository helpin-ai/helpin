import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { useNow } from '@/hooks/useNow'
import { Timer01Icon } from '@/lib/icons'
import type { SupportConversationSLA } from '@/lib/pmTypes'
import { SUPPORT_SLA_METRIC_SHORT, describeSLAItem, supportSLAChip, type SupportSLATone } from '@/lib/supportSLA'
import { cn } from '@/lib/utils'

const TONE_CLASS: Record<SupportSLATone, string> = {
  breached: 'border-red-200 bg-red-50 text-red-700 dark:border-red-900/60 dark:bg-red-950/40 dark:text-red-300',
  urgent: 'border-orange-200 bg-orange-50 text-orange-700 dark:border-orange-900/60 dark:bg-orange-950/40 dark:text-orange-300',
  warning: 'border-amber-200 bg-amber-50 text-amber-800 dark:border-amber-900/60 dark:bg-amber-950/30 dark:text-amber-200',
  ok: 'border-border bg-muted/50 text-muted-foreground',
}

/**
 * The next open SLA target with a live countdown, or "missed" once overdue.
 * Renders nothing when no target is running.
 */
export function SupportSLABadge({ sla, size = 'sm', className }: { sla?: SupportConversationSLA | null; size?: 'xs' | 'sm'; className?: string }) {
  // Only conversations with a running target pay for a ticking clock.
  if (!sla?.next_due_at) return null
  // Remount when the SLA changes so the clock restarts from the new state.
  return <TickingSLABadge key={`${sla.next_due_at}:${sla.breached}`} sla={sla} size={size} className={className} />
}

function TickingSLABadge({ sla, size, className }: { sla: SupportConversationSLA; size: 'xs' | 'sm'; className?: string }) {
  const now = useNow()
  const chip = supportSLAChip(sla, now)
  if (!chip) return null
  const label = chip.tone === 'breached'
    ? `${SUPPORT_SLA_METRIC_SHORT[chip.metric]} ${chip.remaining.replace('-', '')} over`
    : `${SUPPORT_SLA_METRIC_SHORT[chip.metric]} ${chip.remaining}`
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span
          className={cn(
            'inline-flex shrink-0 items-center gap-1 rounded-full border font-medium tabular-nums',
            size === 'xs' ? 'h-[18px] px-1.5 text-[10.5px]' : 'h-6 px-2 text-xs',
            TONE_CLASS[chip.tone],
            className,
          )}
          data-sla-tone={chip.tone}
          aria-label={`SLA ${chip.tone === 'breached' ? 'missed' : 'due'}: ${label}`}
        >
          <Timer01Icon className={size === 'xs' ? 'h-3 w-3' : 'h-3.5 w-3.5'} aria-hidden="true" />
          {label}
        </span>
      </TooltipTrigger>
      <TooltipContent side="bottom" className="max-w-[260px]">
        <div className="space-y-0.5 text-xs">
          <div className="font-medium">{sla.policy_name}</div>
          {sla.metrics.map((item) => <div key={item.metric}>{describeSLAItem(item, now, sla)}</div>)}
        </div>
      </TooltipContent>
    </Tooltip>
  )
}
