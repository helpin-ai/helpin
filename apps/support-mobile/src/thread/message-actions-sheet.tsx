import { useEffect, useState, type ReactNode } from 'react'
import {
  ChevronLeft,
  Copy,
  Info,
  MessageSquareQuote,
  Pencil,
  Trash2,
} from 'lucide-react'
import { toast } from 'sonner'
import {
  useDeleteSupportMessage,
  useMessageInfo,
  type SupportMessage,
} from '@helpin-ai/support-core'
import { haptic } from '@mobile/lib/haptics'
import { Pressable } from '@mobile/ui/pressable'
import { Sheet } from '@mobile/ui/sheet'
import { Spinner } from '@mobile/ui/spinner'

interface MessageActionsSheetProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  workspaceId: string
  message: SupportMessage
  currentUserId?: string
  canEditSupport: boolean
  onRestoreDraft: (text: string) => void
}

type View = 'menu' | 'info' | 'delete'

export function messageActionText(message: SupportMessage): string {
  return (
    message.email_visible_text?.trim() ||
    message.stripped_text?.trim() ||
    message.content.trim()
  )
}

function formatDetailDate(value?: string | null): string | null {
  if (!value) return null
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(date)
}

function humanize(value?: string | null): string | null {
  if (!value) return null
  return value.replaceAll('_', ' ').replace(/\b\w/g, (letter) => letter.toUpperCase())
}

export function MessageActionsSheet({
  open,
  onOpenChange,
  workspaceId,
  message,
  currentUserId,
  canEditSupport,
  onRestoreDraft,
}: MessageActionsSheetProps) {
  const [view, setView] = useState<View>('menu')
  const deleteMessage = useDeleteSupportMessage(workspaceId, message.conversation_id)
  const infoQuery = useMessageInfo(
    workspaceId,
    message.conversation_id,
    message.id,
    open && view === 'info',
  )
  const text = messageActionText(message)
  const canMutateOwnReply = canEditSupport
    && message.sender_type === 'user'
    && message.sender_user_id === currentUserId
    && message.message_type !== 'system'
    && !message.is_internal
  const cancellableUntil = message.cancellable_until ? Date.parse(message.cancellable_until) : 0
  const canEdit = canMutateOwnReply
    && Number.isFinite(cancellableUntil)
    && cancellableUntil > Date.now()

  useEffect(() => {
    if (!open) setView('menu')
  }, [message.id, open])

  const close = () => {
    onOpenChange(false)
    setView('menu')
  }

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(text)
      toast.success('Message copied')
      haptic('notificationSuccess')
      close()
    } catch {
      toast.error('Could not copy message')
    }
  }

  const handleQuote = () => {
    const quoted = text
      .split('\n')
      .map((line) => `> ${line}`)
      .join('\n')
    onRestoreDraft(`${quoted}\n\n`)
    toast.success('Quoted in reply')
    haptic('selection')
    close()
  }

  const handleEdit = () => {
    deleteMessage.mutate(
      { messageId: message.id, undo: true },
      {
        onSuccess: (result) => {
          onRestoreDraft(result.markdown?.trim() || text)
          toast.success('Message moved back to the composer')
          haptic('notificationSuccess')
          close()
        },
        onError: () => {
          toast.error('This message can no longer be edited')
          haptic('notificationError')
        },
      },
    )
  }

  const handleDelete = () => {
    deleteMessage.mutate(
      { messageId: message.id },
      {
        onSuccess: (result) => {
          if (result.email_already_sent) {
            toast('Message removed from chat', {
              description: 'The email may already have been delivered.',
            })
          } else {
            toast.success('Message deleted')
          }
          haptic('notificationSuccess')
          close()
        },
        onError: () => {
          toast.error('Could not delete message')
          haptic('notificationError')
        },
      },
    )
  }

  const info = infoQuery.data

  return (
    <Sheet open={open} onOpenChange={onOpenChange} title="Message actions">
      <div className="px-4 pb-3">
        {view === 'menu' && (
          <div className="flex flex-col">
            {canEdit && (
              <ActionRow
                icon={<Pencil className="h-5 w-5" />}
                label="Edit message"
                busy={deleteMessage.isPending}
                onPress={handleEdit}
              />
            )}
            <ActionRow
              icon={<Copy className="h-5 w-5" />}
              label="Copy"
              disabled={!text}
              onPress={() => void handleCopy()}
            />
            <ActionRow
              icon={<MessageSquareQuote className="h-5 w-5" />}
              label="Reply with quote"
              disabled={!text || !canEditSupport}
              onPress={handleQuote}
            />
            {canMutateOwnReply && (
              <ActionRow
                icon={<Trash2 className="h-5 w-5" />}
                label="Delete message"
                destructive
                onPress={() => setView('delete')}
              />
            )}
            <ActionRow
              icon={<Info className="h-5 w-5" />}
              label="Message info"
              onPress={() => setView('info')}
            />
          </div>
        )}

        {view === 'delete' && (
          <div className="space-y-3">
            <SubViewHeader title="Delete message?" onBack={() => setView('menu')} />
            <p className="text-footnote text-muted-foreground">
              This removes the message from the conversation. An email that already left Helpin cannot be recalled.
            </p>
            <Pressable
              disabled={deleteMessage.isPending}
              onPress={handleDelete}
              className="flex h-12 w-full items-center justify-center rounded-2xl bg-destructive text-body font-medium text-white"
            >
              {deleteMessage.isPending ? <Spinner size={16} /> : 'Delete message'}
            </Pressable>
          </div>
        )}

        {view === 'info' && (
          <div className="space-y-3">
            <SubViewHeader title="Message info" onBack={() => setView('menu')} />
            {infoQuery.isPending ? (
              <div className="flex justify-center py-8" aria-label="Loading message info">
                <Spinner size={18} />
              </div>
            ) : infoQuery.isError || !info ? (
              <div className="flex items-center justify-between gap-3 py-3">
                <span className="text-footnote text-muted-foreground">Message details could not be loaded.</span>
                <Pressable
                  onPress={() => void infoQuery.refetch()}
                  className="h-auto min-h-8 w-auto min-w-0 rounded-full px-3 text-footnote font-medium text-primary"
                >
                  Retry
                </Pressable>
              </div>
            ) : (
              <div className="space-y-2 rounded-2xl bg-muted/50 px-3 py-3">
                <DetailRow label="Sent" value={formatDetailDate(info.sent_at)} />
                <DetailRow label="Sender" value={info.sender.name} />
                <DetailRow label="From" value={info.from} />
                <DetailRow label="To" value={info.to_email} />
                <DetailRow label="Cc" value={info.cc_emails?.join(', ')} />
                <DetailRow label="Bcc" value={info.bcc_emails?.join(', ')} />
                <DetailRow label="Origin" value={humanize(info.origin)} />
                <DetailRow label="Type" value={humanize(info.type)} />
                <DetailRow
                  label="Delivery"
                  value={info.email_delivery_status_label ?? humanize(info.email_delivery_status)}
                />
                <DetailRow label="Delivered" value={formatDetailDate(info.delivered?.delivered_at)} />
                <DetailRow label="Read" value={info.read ? formatDetailDate(info.read_at) ?? 'Yes' : 'No'} />
                <DetailRow label="Edited" value={info.edited ? 'Yes' : 'No'} />
                <DetailRow label="Automated" value={info.automated ? 'Yes' : 'No'} />
                <DetailRow label="Not delivered" value={info.not_delivered_reason} />
              </div>
            )}
          </div>
        )}
      </div>
    </Sheet>
  )
}

function ActionRow({
  icon,
  label,
  onPress,
  destructive,
  busy,
  disabled,
}: {
  icon: ReactNode
  label: string
  onPress: () => void
  destructive?: boolean
  busy?: boolean
  disabled?: boolean
}) {
  return (
    <Pressable
      aria-label={label}
      disabled={busy || disabled}
      onPress={onPress}
      className="flex items-center gap-3 rounded-xl px-2 py-3 text-left active:bg-muted"
    >
      <span className={destructive ? 'text-destructive' : 'text-muted-foreground'}>{icon}</span>
      <span className={`min-w-0 flex-1 text-body ${destructive ? 'text-destructive' : ''}`}>{label}</span>
      {busy && <Spinner size={15} />}
    </Pressable>
  )
}

function SubViewHeader({ title, onBack }: { title: string; onBack: () => void }) {
  return (
    <div className="flex items-center gap-1">
      <Pressable
        aria-label="Back"
        onPress={onBack}
        className="-ml-1 flex h-8 w-8 items-center justify-center rounded-full active:bg-muted"
      >
        <ChevronLeft className="h-5 w-5" />
      </Pressable>
      <h2 className="text-headline font-semibold">{title}</h2>
    </div>
  )
}

function DetailRow({ label, value }: { label: string; value?: string | null }) {
  if (!value) return null
  return (
    <div className="grid grid-cols-[88px_minmax(0,1fr)] gap-2 text-footnote">
      <span className="text-muted-foreground">{label}</span>
      <span className="break-words text-right text-foreground">{value}</span>
    </div>
  )
}
