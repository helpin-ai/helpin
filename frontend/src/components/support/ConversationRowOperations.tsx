import { format } from 'date-fns'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { Clock01Icon } from '@/lib/icons'
import { PriorityIcon } from '@/lib/pmConstants'
import type { SupportConversation } from '@/lib/pmTypes'
import { isSnoozedConversation } from '@/lib/supportInboxFilters'
import { SupportSLABadge } from './SupportSLABadge'

/** Priority, snooze and SLA markers on a conversation row. */
export function ConversationRowOperations({
  conversation,
}: {
  conversation: Pick<SupportConversation, 'priority' | 'snoozed_until' | 'status' | 'sla'>
}) {
  const showPriority = conversation.priority === 'urgent' || conversation.priority === 'high'
  const snoozed = isSnoozedConversation(conversation) && conversation.snoozed_until
  const showSLA = Boolean(conversation.sla?.next_due_at) && conversation.status !== 'resolved' && conversation.status !== 'spam'
  if (!showPriority && !snoozed && !showSLA) return null
  return (
    <div className="mt-1 flex min-w-0 items-center gap-1.5 overflow-hidden" data-testid="conversation-row-operations">
      {showPriority ? (
        <Tooltip>
          <TooltipTrigger asChild>
            <span className="inline-flex shrink-0" role="img" aria-label={`${conversation.priority} priority`}>
              <PriorityIcon priority={conversation.priority} className="h-3.5 w-3.5" />
            </span>
          </TooltipTrigger>
          <TooltipContent side="top"><span className="text-xs capitalize">{conversation.priority} priority</span></TooltipContent>
        </Tooltip>
      ) : null}
      {showSLA ? <SupportSLABadge sla={conversation.sla} size="xs" /> : null}
      {snoozed ? (
        <span className="inline-flex shrink-0 items-center gap-1 text-[10.5px] font-medium text-violet-700 dark:text-violet-300">
          <Clock01Icon className="h-3 w-3" aria-hidden="true" />
          {format(new Date(conversation.snoozed_until!), 'EEE HH:mm')}
        </span>
      ) : null}
    </div>
  )
}
