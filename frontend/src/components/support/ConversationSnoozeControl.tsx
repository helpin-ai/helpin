import { format } from 'date-fns'
import { Button } from '@/components/ui/button'
import { useSnoozeConversation, useUnsnoozeConversation } from '@/hooks/queries/useSupportOperations'
import { Clock01Icon } from '@/lib/icons'
import type { SupportConversation } from '@/lib/pmTypes'
import { isSnoozedConversation } from '@/lib/supportInboxFilters'
import { SnoozeMenu } from './SnoozeMenu'

/** Snooze button, or the snooze time with a wake button when snoozed. */
export function ConversationSnoozeControl({
  workspaceId,
  conversation,
  disabled,
}: {
  workspaceId: string
  conversation: Pick<SupportConversation, 'id' | 'status' | 'snoozed_until' | 'merged_into_id'>
  disabled?: boolean
}) {
  const snooze = useSnoozeConversation(workspaceId)
  const unsnooze = useUnsnoozeConversation(workspaceId)
  if (conversation.merged_into_id || conversation.status === 'resolved' || conversation.status === 'spam') return null

  if (isSnoozedConversation(conversation) && conversation.snoozed_until) {
    const until = new Date(conversation.snoozed_until)
    return (
      <div
        className="flex shrink-0 items-center gap-1 rounded-md border border-violet-200 bg-violet-50 pl-2 text-xs text-violet-800 dark:border-violet-900/60 dark:bg-violet-950/30 dark:text-violet-200"
        data-testid="snoozed-indicator"
        title={`Snoozed until ${format(until, 'EEEE, MMMM d, HH:mm')}`}
      >
        <Clock01Icon className="h-3.5 w-3.5" aria-hidden="true" />
        <span className="whitespace-nowrap"><span className="sr-only">Snoozed until </span>{format(until, 'EEE HH:mm')}</span>
        <Button
          size="sm"
          variant="ghost"
          className="h-7 px-2 text-xs text-violet-800 hover:bg-violet-100 dark:text-violet-200 dark:hover:bg-violet-900/40"
          disabled={disabled || unsnooze.isPending}
          onClick={() => unsnooze.mutate({ conversationId: conversation.id })}
        >
          Wake
        </Button>
      </div>
    )
  }

  return (
    <SnoozeMenu
      disabled={disabled || snooze.isPending}
      onSnooze={(until) => snooze.mutate({ conversationId: conversation.id, until })}
      trigger={(
        <Button size="sm" variant="outline" className="h-7 w-7 p-0" disabled={disabled || snooze.isPending} aria-label="Snooze conversation" title="Snooze (Z)">
          <Clock01Icon className="h-3.5 w-3.5" />
        </Button>
      )}
    />
  )
}
