import { useState, type ReactNode } from 'react'
import { AlertOctagon, ChevronLeft, ChevronRight, Inbox, MailOpen, Pencil, Trash2 } from 'lucide-react'
import {
  useDeleteConversation,
  useMarkConversationUnread,
  useMoveConversation,
  useInboxScopes,
  useUpdateConversationStatus,
  useUpdateConversationSubject,
  type SupportConversation,
} from '@helpin-ai/support-core'
import { toast } from 'sonner'
import { Sheet } from '@mobile/ui/sheet'
import { Pressable } from '@mobile/ui/pressable'
import { Spinner } from '@mobile/ui/spinner'
import { cn } from '@mobile/lib/cn'
import { haptic } from '@mobile/lib/haptics'

export interface ConversationActionsSheetProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  workspaceId: string
  conversation: SupportConversation
  /** Called after actions that should leave the thread (mark unread, delete). */
  onLeave: () => void
}

type View = 'menu' | 'subject' | 'move' | 'delete'

export function ConversationActionsSheet({
  open,
  onOpenChange,
  workspaceId,
  conversation,
  onLeave,
}: ConversationActionsSheetProps) {
  const [view, setView] = useState<View>('menu')
  const [subjectDraft, setSubjectDraft] = useState(conversation.subject)

  const markUnread = useMarkConversationUnread(workspaceId)
  const updateSubject = useUpdateConversationSubject(workspaceId)
  const moveConversation = useMoveConversation(workspaceId)
  const updateStatus = useUpdateConversationStatus(workspaceId)
  const deleteConversation = useDeleteConversation(workspaceId)
  const inboxScopes = useInboxScopes(workspaceId)
  const mailboxes = inboxScopes.data?.mailboxes ?? []

  const isSpam = conversation.status === 'spam'
  const conversationId = conversation.id

  const close = () => {
    onOpenChange(false)
    // Reset to the menu for next open, after the close animation.
    setTimeout(() => setView('menu'), 250)
  }

  const handleMarkUnread = () => {
    markUnread.mutate(conversationId)
    haptic('selection')
    onOpenChange(false)
    onLeave()
  }

  const handleSaveSubject = () => {
    const trimmed = subjectDraft.trim()
    if (!trimmed) return
    updateSubject.mutate(
      { conversationId, subject: trimmed },
      {
        onSuccess: () => {
          toast.success('Subject updated')
          haptic('notificationSuccess')
          close()
        },
        onError: () => toast.error('Could not update subject'),
      },
    )
  }

  const handleMove = (mailboxId: string | null) => {
    moveConversation.mutate(
      { conversationId, mailboxId },
      {
        onSuccess: () => {
          toast.success('Conversation moved')
          haptic('notificationSuccess')
          close()
        },
        onError: () => toast.error('Could not move conversation'),
      },
    )
  }

  const handleToggleSpam = () => {
    updateStatus.mutate(
      { conversationId, status: isSpam ? 'open' : 'spam' },
      {
        onSuccess: () => {
          toast.success(isSpam ? 'Marked as not spam' : 'Marked as spam')
          haptic('selection')
          close()
        },
        onError: () => toast.error('Could not update conversation'),
      },
    )
  }

  const handleDelete = () => {
    deleteConversation.mutate(conversationId, {
      onSuccess: () => {
        toast.success('Conversation deleted')
        haptic('notificationSuccess')
        onOpenChange(false)
        onLeave()
      },
      onError: () => toast.error('Could not delete conversation'),
    })
  }

  return (
    <Sheet open={open} onOpenChange={onOpenChange} title="Conversation actions">
      <div className="px-4 pb-2">
        {view === 'menu' && (
          <div className="flex flex-col">
            <ActionRow icon={MailOpen} label="Mark as unread" onPress={handleMarkUnread} />
            <ActionRow icon={Pencil} label="Set subject" chevron onPress={() => setView('subject')} />
            <ActionRow icon={Inbox} label="Move to inbox" chevron onPress={() => setView('move')} />
            <ActionRow
              icon={AlertOctagon}
              label={isSpam ? 'Not spam' : 'Mark as spam'}
              onPress={handleToggleSpam}
              busy={updateStatus.isPending}
            />
            <ActionRow icon={Trash2} label="Delete conversation" destructive onPress={() => setView('delete')} />
          </div>
        )}

        {view === 'subject' && (
          <div className="flex flex-col gap-3">
            <SubViewHeader title="Set subject" onBack={() => setView('menu')} />
            <input
              autoFocus
              value={subjectDraft}
              onChange={(event) => setSubjectDraft(event.target.value)}
              placeholder="Conversation subject"
              className="rounded-xl border border-input bg-background px-3 py-2.5 text-body outline-none placeholder:text-muted-foreground"
            />
            <PrimaryButton onPress={handleSaveSubject} busy={updateSubject.isPending} disabled={!subjectDraft.trim()}>
              Save
            </PrimaryButton>
          </div>
        )}

        {view === 'move' && (
          <div className="flex flex-col gap-2">
            <SubViewHeader title="Move to inbox" onBack={() => setView('menu')} />
            {inboxScopes.isPending ? (
              <div className="flex justify-center py-6">
                <Spinner />
              </div>
            ) : (
              <div className="flex flex-col">
                <ActionRow
                  icon={Inbox}
                  label="Shared inbox"
                  active={!conversation.mailbox_id}
                  onPress={() => handleMove(null)}
                />
                {mailboxes.map((mailbox) => (
                  <ActionRow
                    key={mailbox.id}
                    icon={Inbox}
                    label={mailbox.name}
                    active={conversation.mailbox_id === mailbox.id}
                    onPress={() => handleMove(mailbox.id)}
                  />
                ))}
              </div>
            )}
          </div>
        )}

        {view === 'delete' && (
          <div className="flex flex-col gap-3">
            <SubViewHeader title="Delete conversation?" onBack={() => setView('menu')} />
            <p className="text-footnote text-muted-foreground">
              This permanently deletes the conversation and its messages. This can’t be undone.
            </p>
            <Pressable
              haptic="impactMedium"
              onPress={handleDelete}
              className="flex h-12 items-center justify-center rounded-2xl bg-destructive text-body font-medium text-white active:opacity-90"
            >
              {deleteConversation.isPending ? <Spinner className="h-4 w-4" /> : 'Delete'}
            </Pressable>
            <Pressable
              onPress={() => setView('menu')}
              className="flex h-12 items-center justify-center rounded-2xl text-body font-medium text-muted-foreground active:bg-muted"
            >
              Cancel
            </Pressable>
          </div>
        )}
      </div>
    </Sheet>
  )
}

function ActionRow({
  icon: Icon,
  label,
  onPress,
  chevron,
  destructive,
  active,
  busy,
}: {
  icon: typeof MailOpen
  label: string
  onPress: () => void
  chevron?: boolean
  destructive?: boolean
  active?: boolean
  busy?: boolean
}) {
  return (
    <Pressable
      onPress={onPress}
      disabled={busy}
      aria-label={label}
      className="flex items-center gap-3 rounded-xl px-2 py-3 text-left active:bg-muted disabled:opacity-50"
    >
      <Icon className={cn('h-5 w-5 shrink-0', destructive ? 'text-destructive' : 'text-muted-foreground')} />
      <span className={cn('min-w-0 flex-1 truncate text-body', destructive && 'text-destructive')}>{label}</span>
      {busy && <Spinner className="h-4 w-4" />}
      {active && <span className="text-footnote font-medium text-primary">Current</span>}
      {chevron && <ChevronRight className="h-4 w-4 shrink-0 text-muted-foreground/60" />}
    </Pressable>
  )
}

function SubViewHeader({ title, onBack }: { title: string; onBack: () => void }) {
  return (
    <div className="flex items-center gap-1">
      <Pressable aria-label="Back" onPress={onBack} className="-ml-1 flex h-8 w-8 items-center justify-center rounded-full active:bg-muted">
        <ChevronLeft className="h-5 w-5" />
      </Pressable>
      <h2 className="text-headline font-semibold">{title}</h2>
    </div>
  )
}

function PrimaryButton({
  children,
  onPress,
  busy,
  disabled,
}: {
  children: ReactNode
  onPress: () => void
  busy?: boolean
  disabled?: boolean
}) {
  return (
    <Pressable
      haptic="impactLight"
      onPress={onPress}
      disabled={disabled || busy}
      className="flex h-12 items-center justify-center rounded-2xl bg-primary text-body font-medium text-primary-foreground active:opacity-90 disabled:opacity-40"
    >
      {busy ? <Spinner className="h-4 w-4" /> : children}
    </Pressable>
  )
}
