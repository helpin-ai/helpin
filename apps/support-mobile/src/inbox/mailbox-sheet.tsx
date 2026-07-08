import { Check } from 'lucide-react'
import { useSupportMailboxes } from '@helpin-ai/support-core'
import { formatBadgeCount } from '@mobile/navigation/tab-bar'
import { cn } from '@mobile/lib/cn'
import { Pressable } from '@mobile/ui/pressable'
import { Sheet } from '@mobile/ui/sheet'
import { Skeleton } from '@mobile/ui/skeleton'

export interface MailboxSheetProps {
  workspaceId: string
  open: boolean
  onOpenChange: (open: boolean) => void
  /** `null` means "All inboxes". */
  selectedMailboxId: string | null
  onSelect: (mailboxId: string | null) => void
}

/** A single selectable row: name, optional unread badge, and a check when selected. */
function MailboxRow({
  label,
  unreadCount,
  selected,
  onPress,
}: {
  label: string
  unreadCount?: number
  selected: boolean
  onPress: () => void
}) {
  const badgeLabel = formatBadgeCount(unreadCount ?? 0)
  return (
    <Pressable
      haptic="selection"
      aria-pressed={selected}
      onPress={onPress}
      className="flex w-full items-center gap-3 px-4 py-3 text-left"
    >
      <span className={cn('flex-1 truncate text-body', selected ? 'font-semibold text-foreground' : 'text-foreground')}>
        {label}
      </span>
      {badgeLabel && (
        <span className="shrink-0 rounded-full bg-muted px-2 py-0.5 text-footnote tnum text-muted-foreground">
          {badgeLabel}
        </span>
      )}
      <span className="flex h-5 w-5 shrink-0 items-center justify-center text-primary">
        {selected && <Check className="h-4 w-4" />}
      </span>
    </Pressable>
  )
}

export function MailboxSheet({ workspaceId, open, onOpenChange, selectedMailboxId, onSelect }: MailboxSheetProps) {
  const { data: mailboxes, isLoading } = useSupportMailboxes(workspaceId)

  const handleSelect = (mailboxId: string | null) => {
    onSelect(mailboxId)
    onOpenChange(false)
  }

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <div className="flex items-center justify-center px-4 pb-2">
        <span className="text-headline">Mailboxes</span>
      </div>
      <div className="flex-1 overflow-y-auto pb-2">
        <MailboxRow
          label="All inboxes"
          selected={selectedMailboxId === null || selectedMailboxId === 'all'}
          onPress={() => handleSelect(null)}
        />
        {isLoading &&
          Array.from({ length: 3 }).map((_, index) => (
            <div key={index} className="px-4 py-3">
              <Skeleton className="h-5 w-40" />
            </div>
          ))}
        {mailboxes?.map((mailbox) => (
          <MailboxRow
            key={mailbox.id}
            label={mailbox.name}
            unreadCount={mailbox.unread_count}
            selected={selectedMailboxId === mailbox.id}
            onPress={() => handleSelect(mailbox.id)}
          />
        ))}
      </div>
    </Sheet>
  )
}
