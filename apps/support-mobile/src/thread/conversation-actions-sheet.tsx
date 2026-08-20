import { useState, type ReactNode } from 'react'
import { AlertOctagon, Check, ChevronLeft, ChevronRight, Inbox, Link, Mail, MailOpen, Pencil, Trash2 } from 'lucide-react'
import {
  useDeleteConversation,
  useMarkConversationUnread,
  useMoveConversation,
  useInboxScopes,
  useSendConversationTranscript,
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

type View = 'menu' | 'subject' | 'move' | 'transcript' | 'delete'

export function ConversationActionsSheet({
  open,
  onOpenChange,
  workspaceId,
  conversation,
  onLeave,
}: ConversationActionsSheetProps) {
  const [view, setView] = useState<View>('menu')
  const [subjectDraft, setSubjectDraft] = useState(conversation.subject)
  const [transcriptEmail, setTranscriptEmail] = useState(
    conversation.customer_email ?? conversation.suggested_primary_recipient_email ?? '',
  )
  const [updateCustomerEmail, setUpdateCustomerEmail] = useState(!conversation.customer_email)

  const markUnread = useMarkConversationUnread(workspaceId)
  const updateSubject = useUpdateConversationSubject(workspaceId)
  const moveConversation = useMoveConversation(workspaceId)
  const updateStatus = useUpdateConversationStatus(workspaceId)
  const deleteConversation = useDeleteConversation(workspaceId)
  const sendTranscript = useSendConversationTranscript(workspaceId)
  const inboxScopes = useInboxScopes(workspaceId)
  const mailboxes = inboxScopes.data?.mailboxes ?? []

  const isSpam = conversation.status === 'spam'
  const conversationId = conversation.id
  const transcriptRecipients = Array.from(new Map([
    conversation.customer_email,
    conversation.suggested_primary_recipient_email,
    ...(conversation.email_cc ?? []),
    ...(conversation.email_thread_participants ?? []),
  ].filter((email): email is string => !!email?.trim()).map((email) => [email.trim().toLowerCase(), email.trim()])).values())

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

  const handleCopyLink = async () => {
    try {
      await navigator.clipboard.writeText(window.location.href)
      toast.success('Link copied')
      haptic('notificationSuccess')
      close()
    } catch {
      toast.error('Could not copy link')
    }
  }

  const openTranscript = () => {
    if (!transcriptEmail) setTranscriptEmail(transcriptRecipients[0] ?? '')
    setView('transcript')
  }

  const handleSendTranscript = () => {
    const email = transcriptEmail.trim()
    if (!email) return
    sendTranscript.mutate(
      { conversationId, email, updateCustomerEmail },
      {
        onSuccess: (data) => {
          toast.success(data.message || 'Transcript sent')
          haptic('notificationSuccess')
          close()
        },
        onError: () => toast.error('Could not send transcript'),
      },
    )
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
            <ActionRow icon={Link} label="Copy link" onPress={() => void handleCopyLink()} />
            <ActionRow icon={Mail} label="Email transcript" chevron onPress={openTranscript} />
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

        {view === 'transcript' && (
          <div className="flex flex-col gap-3">
            <SubViewHeader title="Email transcript" onBack={() => setView('menu')} />
            {transcriptRecipients.length > 0 && (
              <div className="flex flex-col gap-1">
                <span className="text-footnote font-medium">Recent recipients</span>
                {transcriptRecipients.map((email) => (
                  <Pressable
                    key={email}
                    aria-label={`Send transcript to ${email}`}
                    aria-pressed={transcriptEmail === email}
                    onPress={() => setTranscriptEmail(email)}
                    className="flex h-11 items-center gap-2 rounded-xl px-2 text-left active:bg-muted"
                  >
                    <span className="min-w-0 flex-1 truncate text-body">{email}</span>
                    {transcriptEmail === email && <Check className="h-4 w-4 text-primary" />}
                  </Pressable>
                ))}
              </div>
            )}
            <label className="flex flex-col gap-1.5">
              <span className="text-footnote font-medium">Email address</span>
              <input
                type="email"
                value={transcriptEmail}
                onChange={(event) => setTranscriptEmail(event.target.value)}
                placeholder="recipient@example.com"
                className="rounded-xl border border-input bg-background px-3 py-2.5 text-body outline-none placeholder:text-muted-foreground"
              />
            </label>
            {(!conversation.customer_email || transcriptEmail.trim().toLowerCase() !== conversation.customer_email.toLowerCase()) && (
              <label className="flex items-start gap-2 text-footnote text-muted-foreground">
                <input
                  type="checkbox"
                  checked={updateCustomerEmail}
                  onChange={(event) => setUpdateCustomerEmail(event.target.checked)}
                  className="mt-0.5 h-4 w-4"
                />
                Also save this email to the customer profile
              </label>
            )}
            <PrimaryButton
              onPress={handleSendTranscript}
              busy={sendTranscript.isPending}
              disabled={!transcriptEmail.trim()}
            >
              Send transcript
            </PrimaryButton>
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
