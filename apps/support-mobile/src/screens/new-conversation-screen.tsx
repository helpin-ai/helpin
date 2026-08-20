import { useDeferredValue, useEffect, useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useParams, useRouter } from '@tanstack/react-router'
import { Check, Mail, MessageCircle, Search, Tag, X } from 'lucide-react'
import {
  useCreateConversationWithMessage,
  useInboxScopes,
  useSupportTags,
  type SupportInboxScope,
} from '@helpin-ai/support-core'
import { parseSupportInboxViewFilters } from '@/lib/supportInboxFilters'
import { cn } from '@mobile/lib/cn'
import { haptic } from '@mobile/lib/haptics'
import {
  mobileContactsService,
  type MobileCRMContact,
} from '@mobile/lib/services/mobile-contacts-service'
import { workspacesService } from '@mobile/lib/services/workspaces-service'
import { useWorkspacePermissions } from '@mobile/lib/use-workspace-permissions'
import { useSupportViewStore } from '@mobile/stores/support-view-store'
import { useWorkspaceStore } from '@mobile/stores/workspace-store'
import { EmptyState } from '@mobile/ui/empty-state'
import { OfflineBanner } from '@mobile/ui/offline-banner'
import { Pressable } from '@mobile/ui/pressable'
import { Spinner } from '@mobile/ui/spinner'
import { TextField } from '@mobile/ui/text-field'
import { TopBar } from '@mobile/ui/top-bar'
import { toast } from 'sonner'
import { parseEmailList } from './new-conversation-helpers'


function contactName(contact: MobileCRMContact): string {
  return [contact.first_name, contact.last_name].filter(Boolean).join(' ').trim() || contact.email || 'Unnamed contact'
}

function mailboxValue(value: string): string | null {
  return value === 'shared' || value === 'all' ? null : value
}

function isMailbox(value: SupportInboxScope | null | undefined): value is SupportInboxScope {
  return Boolean(value)
}

function isEmail(value: string): boolean {
  return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value)
}

export function NewConversationScreen() {
  const { slug } = useParams({ strict: false })
  const router = useRouter()
  const selection = useSupportViewStore((state) => state.selection)
  const setCurrentWorkspace = useWorkspaceStore((state) => state.setCurrentWorkspace)
  const workspaceQuery = useQuery({
    queryKey: ['workspace', slug],
    queryFn: async () => {
      const { data, error } = await workspacesService.getBySlug(slug ?? '')
      if (error || !data) throw new Error(error ?? 'Failed to load workspace')
      return data
    },
    enabled: Boolean(slug),
  })
  const workspace = workspaceQuery.data
  const workspaceId = workspace?.id ?? ''
  const { accessQuery, canReadSupport, canEditSupport } = useWorkspacePermissions(workspaceId)
  const scopesQuery = useInboxScopes(workspaceId)
  const tagsQuery = useSupportTags(workspaceId)
  const createConversation = useCreateConversationWithMessage(workspaceId)

  const [recipient, setRecipient] = useState('')
  const [selectedContact, setSelectedContact] = useState<MobileCRMContact | null>(null)
  const deferredRecipient = useDeferredValue(recipient.trim())
  const contactsQuery = useQuery({
    queryKey: ['crm', workspaceId, 'contacts', 'search', deferredRecipient],
    queryFn: async () => {
      const { data, error } = await mobileContactsService.search(workspaceId, deferredRecipient)
      if (error || !data) throw new Error(error ?? 'Failed to search contacts')
      return data.data
    },
    enabled: Boolean(workspaceId && deferredRecipient),
    staleTime: 30_000,
  })
  const [showContacts, setShowContacts] = useState(false)
  const [subject, setSubject] = useState('')
  const [message, setMessage] = useState('')
  const [sendEmail, setSendEmail] = useState(true)
  const [sendChat, setSendChat] = useState(false)
  const [cc, setCc] = useState('')
  const [bcc, setBcc] = useState('')
  const [showCopies, setShowCopies] = useState(false)
  const [mailboxId, setMailboxId] = useState('shared')
  const [tagIds, setTagIds] = useState<string[]>([])

  useEffect(() => {
    if (!workspace) return
    setCurrentWorkspace({ id: workspace.id, slug: workspace.slug, name: workspace.name })
  }, [setCurrentWorkspace, workspace])

  useEffect(() => {
    if (selection.kind === 'mailbox') {
      setMailboxId(selection.mailboxId)
    } else if (selection.kind === 'builtin' && selection.mailboxId !== 'all') {
      setMailboxId(selection.mailboxId)
    } else if (selection.kind === 'custom') {
      const selectedMailboxId = parseSupportInboxViewFilters(selection.filters, 'inbox').selectedMailboxId
      setMailboxId(selectedMailboxId === 'all' ? 'shared' : selectedMailboxId)
    }
  }, [selection])

  const mailboxOptions = useMemo(() => [
    scopesQuery.data?.shared_inbox,
    ...(scopesQuery.data?.mailboxes ?? []),
  ].filter(isMailbox), [scopesQuery.data])
  const selectedEmail = selectedContact?.email ?? recipient.trim()
  const ccEmails = parseEmailList(cc)
  const bccEmails = parseEmailList(bcc)
  const copiesValid = [...ccEmails, ...bccEmails].every(isEmail)
  const customerEmail = sendEmail ? selectedEmail : selectedContact?.email
  const channels = [
    ...(sendChat ? ['chat' as const] : []),
    ...(sendEmail ? ['email' as const] : []),
  ]
  const canSend = canEditSupport &&
    subject.trim().length > 0 &&
    message.trim().length > 0 &&
    channels.length > 0 &&
    (!sendEmail || isEmail(selectedEmail)) &&
    (!sendEmail || copiesValid) &&
    (!sendChat || Boolean(selectedContact))

  const goBack = () => {
    if (router.history.canGoBack()) router.history.back()
    else router.navigate({ to: '/w/$slug/support', params: { slug: slug ?? '' } })
  }

  const chooseContact = (contact: MobileCRMContact) => {
    setSelectedContact(contact)
    setRecipient(contact.email ?? contactName(contact))
    setSendChat(true)
    setSendEmail(Boolean(contact.email))
    setShowContacts(false)
    haptic('selection')
  }

  const send = async () => {
    if (!canSend || createConversation.isPending) return
    try {
      const result = await createConversation.mutateAsync({
        subject: subject.trim(),
        content: message.trim(),
        customer_name: selectedContact ? contactName(selectedContact) : undefined,
        customer_email: customerEmail || undefined,
        crm_contact_id: selectedContact?.id,
        mailbox_id: mailboxValue(mailboxId),
        channels,
        tag_ids: tagIds,
        cc_emails: sendEmail ? ccEmails : [],
        bcc_emails: sendEmail ? bccEmails : [],
      })
      haptic('notificationSuccess')
      router.navigate({
        to: '/w/$slug/support/$conversationId',
        params: { slug: slug ?? '', conversationId: result.conversation.id },
        replace: true,
      })
    } catch (error) {
      haptic('notificationError')
      toast.error('Could not send conversation', {
        description: error instanceof Error ? error.message : 'Please try again.',
      })
    }
  }

  const loading = workspaceQuery.isPending || accessQuery.isPending
  const denied = accessQuery.isSuccess && (!canReadSupport || !canEditSupport)

  return (
    <div className="flex h-dvh flex-col">
      <TopBar
        title="New conversation"
        subtitle={workspace?.name}
        onBack={goBack}
        trailing={
          <Pressable
            aria-label="Send conversation"
            disabled={!canSend || createConversation.isPending}
            onPress={() => void send()}
            className="flex h-auto min-h-9 w-auto min-w-0 items-center justify-center rounded-full bg-primary px-3 text-footnote font-semibold text-primary-foreground disabled:opacity-40"
          >
            {createConversation.isPending ? <Spinner size={15} /> : 'Send'}
          </Pressable>
        }
      />
      <OfflineBanner />

      {loading ? (
        <div className="flex min-h-48 flex-1 items-center justify-center"><Spinner /></div>
      ) : denied ? (
        <EmptyState
          icon={<MessageCircle className="h-6 w-6" />}
          title="Support editing unavailable"
          body="Ask a workspace admin to grant you permission to create conversations."
        />
      ) : (
        <div className="min-h-0 flex-1 space-y-5 overflow-y-auto px-4 py-4 pb-[max(var(--safe-bottom),24px)]">
          <section className="space-y-2">
            <label className="text-footnote font-medium">To</label>
            <div className="relative">
              <div className="flex min-h-[52px] items-center gap-2 rounded-xl border border-input bg-background px-3.5 focus-within:border-ring">
                <Search className="h-4 w-4 shrink-0 text-muted-foreground" />
                <input
                  autoFocus
                  value={recipient}
                  onChange={(event) => {
                    setRecipient(event.target.value)
                    setSelectedContact(null)
                    setSendChat(false)
                    setSendEmail(true)
                    setShowContacts(Boolean(event.target.value.trim()))
                  }}
                  onFocus={() => setShowContacts(Boolean(recipient.trim()))}
                  placeholder="Search contacts or enter email"
                  aria-label="Recipient"
                  className="min-w-0 flex-1 bg-transparent text-body outline-none placeholder:text-muted-foreground"
                />
                {recipient && (
                  <Pressable
                    aria-label="Clear recipient"
                    onPress={() => {
                      setRecipient('')
                      setSelectedContact(null)
                      setSendChat(false)
                      setShowContacts(false)
                    }}
                    className="flex items-center justify-center rounded-full text-muted-foreground"
                  >
                    <X className="h-4 w-4" />
                  </Pressable>
                )}
              </div>
              {showContacts && deferredRecipient && (
                <div className="absolute inset-x-0 top-full z-30 mt-1 max-h-60 overflow-y-auto rounded-xl border border-border bg-popover p-1 shadow-lg">
                  {contactsQuery.isPending && <div className="flex justify-center py-4"><Spinner size={16} /></div>}
                  {(contactsQuery.data ?? []).map((contact) => (
                    <Pressable
                      key={contact.id}
                      onPress={() => chooseContact(contact)}
                      className="flex h-auto min-h-12 w-full items-center gap-3 rounded-lg px-3 py-2 text-left active:bg-muted"
                    >
                      <span className="min-w-0 flex-1">
                        <span className="block truncate text-body font-medium">{contactName(contact)}</span>
                        {contact.email && <span className="block truncate text-footnote text-muted-foreground">{contact.email}</span>}
                      </span>
                      <span className="text-caption text-muted-foreground">{contact.lifecycle_stage.replaceAll('_', ' ')}</span>
                    </Pressable>
                  ))}
                  {isEmail(recipient.trim()) && (
                    <Pressable
                      aria-label={`Use ${recipient.trim()} as email recipient`}
                      onPress={() => {
                        setSelectedContact(null)
                        setSendEmail(true)
                        setSendChat(false)
                        setShowContacts(false)
                      }}
                      className="flex h-auto min-h-12 w-full items-center rounded-lg px-3 py-2 text-left text-primary active:bg-primary/10"
                    >
                      <span className="min-w-0 flex-1">
                        <span className="block truncate text-body font-medium">{recipient.trim()}</span>
                        <span className="block text-footnote">Use as email recipient</span>
                      </span>
                    </Pressable>
                  )}
                  {!contactsQuery.isPending && !isEmail(recipient.trim()) && (contactsQuery.data ?? []).length === 0 && (
                    <div className="px-3 py-3 text-footnote text-muted-foreground">
                      No matching contacts.
                    </div>
                  )}
                </div>
              )}
            </div>
          </section>

          <section className="space-y-2">
            <span className="text-footnote font-medium">Channel</span>
            <div className="flex gap-2">
              <Pressable
                aria-pressed={sendEmail}
                onPress={() => setSendEmail((value) => !value)}
                className={cn(
                  'flex h-auto min-h-10 w-auto min-w-0 items-center gap-2 rounded-full border px-3 text-footnote font-medium',
                  sendEmail ? 'border-primary/30 bg-primary/10 text-primary' : 'border-border text-muted-foreground',
                )}
              >
                <Mail className="h-4 w-4" /> Email
              </Pressable>
              <Pressable
                aria-pressed={sendChat}
                disabled={!selectedContact}
                onPress={() => setSendChat((value) => !value)}
                className={cn(
                  'flex h-auto min-h-10 w-auto min-w-0 items-center gap-2 rounded-full border px-3 text-footnote font-medium disabled:opacity-40',
                  sendChat ? 'border-primary/30 bg-primary/10 text-primary' : 'border-border text-muted-foreground',
                )}
              >
                <MessageCircle className="h-4 w-4" /> Chat
              </Pressable>
              {sendEmail && (
                <Pressable
                  onPress={() => setShowCopies((value) => !value)}
                  className="h-auto min-h-10 w-auto min-w-0 rounded-full px-3 text-footnote font-medium text-primary"
                >
                  Cc/Bcc
                </Pressable>
              )}
            </div>
          </section>

          {sendEmail && showCopies && (
            <div className="grid gap-3">
              <TextField
                label="Cc emails"
                value={cc}
                onChange={setCc}
                error={ccEmails.some((email) => !isEmail(email)) ? 'Enter valid email addresses.' : undefined}
              />
              <TextField
                label="Bcc emails"
                value={bcc}
                onChange={setBcc}
                error={bccEmails.some((email) => !isEmail(email)) ? 'Enter valid email addresses.' : undefined}
              />
              <p className="text-caption text-muted-foreground">Separate multiple addresses with commas.</p>
            </div>
          )}

          <TextField label="Subject" value={subject} onChange={setSubject} />

          <section className="space-y-2">
            <label htmlFor="new-conversation-message" className="text-footnote font-medium">Message</label>
            <textarea
              id="new-conversation-message"
              value={message}
              onChange={(event) => setMessage(event.target.value)}
              placeholder="Write your message…"
              rows={7}
              className="w-full resize-none rounded-xl border border-input bg-background px-3.5 py-3 text-body text-foreground outline-none placeholder:text-muted-foreground focus:border-ring"
            />
          </section>

          <section className="space-y-2">
            <label htmlFor="new-conversation-inbox" className="text-footnote font-medium">Inbox</label>
            <select
              id="new-conversation-inbox"
              value={mailboxId}
              onChange={(event) => setMailboxId(event.target.value)}
              className="h-[52px] w-full rounded-xl border border-input bg-background px-3.5 text-body outline-none focus:border-ring"
            >
              {mailboxOptions.map((mailbox) => (
                <option key={mailbox.id} value={mailbox.id}>{mailbox.name}</option>
              ))}
            </select>
          </section>

          <section className="space-y-2">
            <span className="text-footnote font-medium">Tags <span className="font-normal text-muted-foreground">(optional)</span></span>
            {tagsQuery.isPending ? (
              <Spinner size={16} />
            ) : (
              <div className="flex flex-wrap gap-2">
                {(tagsQuery.data ?? []).map((tag) => {
                  const selected = tagIds.includes(tag.id)
                  return (
                    <Pressable
                      key={tag.id}
                      aria-pressed={selected}
                      onPress={() => setTagIds((current) => selected
                        ? current.filter((id) => id !== tag.id)
                        : [...current, tag.id])}
                      className={cn(
                        'flex h-auto min-h-10 w-auto min-w-0 items-center gap-1.5 rounded-full border px-3 text-footnote font-medium',
                        selected ? 'border-primary/30 bg-primary/10 text-primary' : 'border-border',
                      )}
                    >
                      {selected ? <Check className="h-3.5 w-3.5" /> : <Tag className="h-3.5 w-3.5" />}
                      {tag.name}
                    </Pressable>
                  )
                })}
              </div>
            )}
          </section>
        </div>
      )}
    </div>
  )
}
