import { formatSnoozeUntil } from '@/lib/supportSnooze'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { Clock01Icon } from '@/lib/icons'
import { PriorityIcon } from '@/lib/pmConstants'
import type { SupportConversation } from '@/lib/pmTypes'
import { isSnoozedConversation } from '@/lib/supportInboxFilters'
import { SupportSLABadge } from './SupportSLABadge'

/** High and urgent priority as a badge on the bottom-right of the row avatar. */
export function ConversationPriorityBadge({ priority }: { priority: SupportConversation['priority'] }) {
  if (priority !== 'urgent' && priority !== 'high') return null
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span
          className="absolute -bottom-0.5 -right-0.5 z-10 inline-flex h-3.5 w-3.5 items-center justify-center rounded-full bg-background ring-1 ring-background shadow-sm"
          role="img"
          aria-label={`${priority} priority`}
        >
          <PriorityIcon priority={priority} className="h-3.5 w-3.5" />
        </span>
      </TooltipTrigger>
      <TooltipContent side="top"><span className="text-xs">{priority === 'urgent' ? 'Urgent' : 'High'} priority</span></TooltipContent>
    </Tooltip>
  )
}

/** Snooze, SLA and request type markers on a conversation row. */
export function ConversationRowOperations({
  conversation,
  requestTypeName,
}: {
  conversation: Pick<SupportConversation, 'snoozed_until' | 'status' | 'sla'>
  requestTypeName?: string
}) {
  const snoozed = isSnoozedConversation(conversation) && conversation.snoozed_until
  const showSLA = Boolean(conversation.sla?.next_due_at) && conversation.status !== 'resolved' && conversation.status !== 'spam'
  if (!snoozed && !showSLA && !requestTypeName) return null
  return (
    <div className="mt-1 flex min-w-0 items-center gap-1.5 overflow-hidden" data-testid="conversation-row-operations">
      {requestTypeName ? <span className="min-w-0 truncate text-[10.5px] text-muted-foreground">{requestTypeName}</span> : null}
      {showSLA ? <SupportSLABadge sla={conversation.sla} size="xs" /> : null}
      {snoozed ? (
        <span className="inline-flex shrink-0 items-center gap-1 text-[10.5px] font-medium text-violet-700 dark:text-violet-300">
          <Clock01Icon className="h-3 w-3" aria-hidden="true" />
          {formatSnoozeUntil(new Date(conversation.snoozed_until!))}
        </span>
      ) : null}
    </div>
  )
}
