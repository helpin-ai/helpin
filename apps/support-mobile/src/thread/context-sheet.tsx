import { useEffect, useState } from 'react'
import { useParams, useRouter } from '@tanstack/react-router'
import {
  Check,
  CheckCircle2,
  ChevronRight,
  Copy,
  Pencil,
  RotateCcw,
  Tag,
  UserRoundPlus,
  X,
} from 'lucide-react'
import { toast } from 'sonner'
import {
  useAssignConversationUser,
  useConversation,
  useUpdateConversationStatus,
  useUpdateConversationCustomerName,
  type ConversationStatus,
} from '@helpin-ai/support-core'
import { Sheet } from '@mobile/ui/sheet'
import { Avatar } from '@mobile/ui/avatar'
import { Badge, type BadgeTone } from '@mobile/ui/badge'
import { Pressable } from '@mobile/ui/pressable'
import { Spinner } from '@mobile/ui/spinner'
import { haptic } from '@mobile/lib/haptics'
import { cn } from '@mobile/lib/cn'
import { displayNameFor } from '@mobile/inbox/conversation-cell'
import { formatRelativeTime } from '@mobile/inbox/inbox-helpers'
import { AssignList } from './assign-list'
import { ConversationTagEditor } from './conversation-tag-editor'
import { ConversationContactTools } from './conversation-contact-tools'
import { CustomerContext } from './customer-context'

export interface ContextSheetProps {
  workspaceId: string
  conversationId: string | null
  open: boolean
  onOpenChange: (open: boolean) => void
  canEdit: boolean
  canReadCRM?: boolean
  canEditCRM?: boolean
}

const STATUS_LABELS: Record<ConversationStatus, string> = {
  open: 'Open',
  waiting_on_customer: 'Waiting on customer',
  resolved: 'Resolved',
  spam: 'Spam',
}

const STATUS_TONES: Record<ConversationStatus, BadgeTone> = {
  open: 'primary',
  waiting_on_customer: 'warning',
  resolved: 'success',
  spam: 'neutral',
}


/**
 * Bottom sheet opened from the conversation header (Task 13's `onTitlePress`
 * seam): customer identity, status toggle, inline assignment, existing-tag
 * editing, and visitor context.
 */
export function ContextSheet({
  workspaceId,
  conversationId,
  open,
  onOpenChange,
  canEdit,
  canReadCRM = false,
  canEditCRM = false,
}: ContextSheetProps) {
  const [assignExpanded, setAssignExpanded] = useState(false)
  const router = useRouter()
  const { slug } = useParams({ strict: false })
  const [tagsExpanded, setTagsExpanded] = useState(false)
  const [editingCustomerName, setEditingCustomerName] = useState(false)
  const [customerNameDraft, setCustomerNameDraft] = useState('')

  const conversationQuery = useConversation(workspaceId, conversationId)
  const updateStatus = useUpdateConversationStatus(workspaceId)
  const assignUser = useAssignConversationUser(workspaceId)

  const updateCustomerName = useUpdateConversationCustomerName(workspaceId)
  const conversation = conversationQuery.data

  useEffect(() => {
    setAssignExpanded(false)
    setTagsExpanded(false)
    setEditingCustomerName(false)
    setCustomerNameDraft('')
  }, [conversationId, open])

  const handleCopyEmail = async () => {
    const email = conversation?.customer_email
    if (!email) return
    try {
      await navigator.clipboard.writeText(email)
      toast.success('Copied')
    } catch {
      toast.error('Could not copy email')
    }
  }

  const handleToggleStatus = () => {
    if (!canEdit || !conversation || !conversationId) return
    const nextStatus: ConversationStatus = conversation.status === 'resolved' ? 'open' : 'resolved'
    haptic('selection')
    updateStatus.mutate(
      { conversationId, status: nextStatus },
      {
        onSuccess: () => haptic('notificationSuccess'),
        onError: () => {
          haptic('notificationError')
          toast.error('Could not update status')
        },
      },
    )
  }

  const handleAssign = (userId: string | null) => {
    if (!canEdit || !conversationId) return
    haptic('selection')
    assignUser.mutate(
      { conversationId, userId },
      {
        onSuccess: () => haptic('notificationSuccess'),
        onError: () => {
          haptic('notificationError')
          toast.error('Could not update assignment')
        },
      },
    )
    setAssignExpanded(false)
  }

  const handleSaveCustomerName = () => {
    const customerName = customerNameDraft.trim()
    if (!conversationId || !customerName || updateCustomerName.isPending) return
    updateCustomerName.mutate(
      { conversationId, customerName },
      {
        onSuccess: () => {
          setEditingCustomerName(false)
          haptic('notificationSuccess')
        },
        onError: () => {
          haptic('notificationError')
          toast.error('Could not update customer name')
        },
      },
    )
  }

  if (!conversation) {
    return (
      <Sheet open={open} onOpenChange={onOpenChange} fullScreen title="Conversation details">
        <header className="flex h-[56px] shrink-0 items-center justify-between border-b border-border/60 px-4">
          <h2 className="text-headline">Details</h2>
          <Pressable
            aria-label="Close conversation details"
            onPress={() => onOpenChange(false)}
            className="flex h-10 min-h-10 w-10 min-w-10 items-center justify-center rounded-full bg-muted/70 text-muted-foreground"
          >
            <X className="h-5 w-5" />
          </Pressable>
        </header>
        <div className="flex min-h-0 flex-1 items-center justify-center px-4 pb-10">
          <Spinner size={20} />
        </div>
      </Sheet>
    )
  }

  const customerName = displayNameFor(conversation)
  const isResolved = conversation.status === 'resolved'
  const lastActive = conversation.contact_last_seen_at
    ? `Active ${formatRelativeTime(conversation.contact_last_seen_at)}`
    : undefined


  return (
    <Sheet
      open={open}
      onOpenChange={onOpenChange}
      fullScreen
      className="overflow-hidden"
      title="Conversation details"
    >
      <header className="flex h-[56px] shrink-0 items-center justify-between border-b border-border/60 px-4">
        <div className="min-w-0">
          <h2 className="text-headline">Details</h2>
          <p className="text-caption text-muted-foreground">Conversation #{conversation.display_id}</p>
        </div>
        <Pressable
          aria-label="Close conversation details"
          onPress={() => onOpenChange(false)}
          className="flex h-10 min-h-10 w-10 min-w-10 items-center justify-center rounded-full bg-muted/70 text-muted-foreground"
        >
          <X className="h-5 w-5" />
        </Pressable>
      </header>

      <div className="flex min-h-0 flex-1 flex-col overflow-y-auto overscroll-contain pb-6">
        <section className="flex items-start gap-3.5 px-4 py-5">
          <Avatar name={customerName} size={52} />
          <div className="min-w-0 flex-1 space-y-1">
            {editingCustomerName ? (
              <div className="flex items-center gap-1.5">
                <input
                  autoFocus
                  aria-label="Customer name"
                  value={customerNameDraft}
                  onChange={(event) => setCustomerNameDraft(event.target.value)}
                  onKeyDown={(event) => {
                    if (event.key === 'Enter') handleSaveCustomerName()
                    if (event.key === 'Escape') setEditingCustomerName(false)
                  }}
                  className="h-10 min-w-0 flex-1 rounded-xl border border-input bg-background px-3 text-body outline-none"
                />
                <Pressable
                  aria-label="Save customer name"
                  disabled={!customerNameDraft.trim() || updateCustomerName.isPending}
                  onPress={handleSaveCustomerName}
                  className="flex h-10 min-h-10 w-10 min-w-10 items-center justify-center rounded-full bg-primary/10 text-primary disabled:opacity-40"
                >
                  <Check className="h-4 w-4" />
                </Pressable>
                <Pressable
                  aria-label="Cancel editing customer name"
                  onPress={() => setEditingCustomerName(false)}
                  className="flex h-10 min-h-10 w-10 min-w-10 items-center justify-center rounded-full bg-muted/70 text-muted-foreground"
                >
                  <X className="h-4 w-4" />
                </Pressable>
              </div>
            ) : (
              <div className="flex items-start gap-1.5">
                <div className="min-w-0 flex-1">
                  <p className="truncate text-headline">{customerName}</p>
                  <div className="mt-1 flex">
                    <Badge tone={STATUS_TONES[conversation.status]}>{STATUS_LABELS[conversation.status]}</Badge>
                  </div>
                </div>
                {canEdit && (
                  <Pressable
                    aria-label="Edit customer name"
                    onPress={() => {
                      setCustomerNameDraft(customerName)
                      setEditingCustomerName(true)
                    }}
                    className="flex h-9 min-h-9 w-9 min-w-9 items-center justify-center rounded-full text-muted-foreground"
                  >
                    <Pencil className="h-3.5 w-3.5" />
                  </Pressable>
                )}
              </div>
            )}
            {conversation.customer_email && (
              <Pressable
                haptic="selection"
                onPress={handleCopyEmail}
                aria-label="Copy email"
                className="flex h-auto min-h-8 w-auto min-w-0 items-center gap-1.5 text-footnote text-muted-foreground"
              >
                <span className="truncate">{conversation.customer_email}</span>
                <Copy className="h-3 w-3 shrink-0" />
              </Pressable>
            )}
            {lastActive && <p className="text-footnote text-muted-foreground">{lastActive}</p>}
          </div>
        </section>

        {canEdit ? (
          <section className="border-y border-border/60 bg-muted/20 px-4 py-4">
            <p className="mb-2.5 text-caption font-semibold uppercase tracking-wide text-muted-foreground">Quick actions</p>
            <div className="grid grid-cols-3 gap-2.5">
              <Pressable
                onPress={handleToggleStatus}
                disabled={updateStatus.isPending}
                className="flex h-12 min-h-12 items-center justify-center gap-1.5 rounded-xl border border-border/70 bg-background px-2 text-footnote font-medium text-foreground"
              >
                {isResolved
                  ? <RotateCcw className="h-4 w-4 shrink-0" />
                  : <CheckCircle2 className="h-4 w-4 shrink-0" />}
                {isResolved ? 'Reopen' : 'Resolve'}
              </Pressable>
              <Pressable
                haptic="selection"
                onPress={() => setAssignExpanded((value) => !value)}
                aria-expanded={assignExpanded}
                className="flex h-12 min-h-12 items-center justify-center gap-1 rounded-xl border border-border/70 bg-background px-2 text-footnote font-medium text-foreground"
              >
                <UserRoundPlus className="h-4 w-4 shrink-0" />
                Assign
                <ChevronRight className={cn('h-3.5 w-3.5 transition-transform', assignExpanded && 'rotate-90')} />
              </Pressable>
              <Pressable
                haptic="selection"
                onPress={() => setTagsExpanded((value) => !value)}
                aria-label="Tags"
                aria-expanded={tagsExpanded}
                className="flex h-12 min-h-12 items-center justify-center gap-1 rounded-xl border border-border/70 bg-background px-2 text-footnote font-medium text-foreground"
              >
                <Tag className="h-4 w-4 shrink-0" />
                Tags
                {(conversation.tags?.length ?? 0) > 0 && (
                  <span className="text-caption text-muted-foreground">{conversation.tags?.length}</span>
                )}
                <ChevronRight className={cn('h-3.5 w-3.5 transition-transform', tagsExpanded && 'rotate-90')} />
              </Pressable>
            </div>
          </section>
        ) : (
          <div className="border-y border-border/60 bg-muted/20 px-4 py-4 text-footnote text-muted-foreground">
            Read-only access
          </div>
        )}

        {canEdit && assignExpanded && (
          <div className="border-t border-border/60">
            <AssignList
              workspaceId={workspaceId}
              conversationId={conversationId ?? ''}
              currentUserId={conversation.assigned_user_id ?? null}
              onSelect={handleAssign}
            />
          </div>
        )}

        {canEdit && tagsExpanded && conversationId && (
          <ConversationTagEditor
            workspaceId={workspaceId}
            conversationId={conversationId}
            selectedTags={conversation.tags ?? []}
          />
        )}

        <ConversationContactTools
          workspaceId={workspaceId}
          conversation={conversation}
          canEdit={canEdit}
          canReadCRM={canReadCRM}
          canEditCRM={canEditCRM}
        />

        {conversationId && (
          <CustomerContext
            workspaceId={workspaceId}
            conversationId={conversationId}
            canEdit={canEdit}
            channel={conversation.source}
            onOpenConversation={(nextConversationId) => {
              onOpenChange(false)
              router.navigate({
                to: '/w/$slug/support/$conversationId',
                params: { slug: slug ?? '', conversationId: nextConversationId },
              })
            }}
          />
        )}
      </div>
    </Sheet>
  )
}
