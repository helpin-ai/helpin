import { useDeferredValue, useEffect, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Check, Link2, Mail, Search, UserRoundCheck, X } from 'lucide-react'
import { toast } from 'sonner'
import {
  useUpdateConversationCRMContact,
  useUpdateConversationEmailRecipients,
  type SupportConversation,
} from '@helpin-ai/support-core'
import {
  mobileContactsService,
  type MobileCRMContact,
} from '@mobile/lib/services/mobile-contacts-service'
import { haptic } from '@mobile/lib/haptics'
import { Pressable } from '@mobile/ui/pressable'
import { Spinner } from '@mobile/ui/spinner'

interface ConversationContactToolsProps {
  workspaceId: string
  conversation: SupportConversation
  canEdit: boolean
}

interface RecipientSummary {
  primary: string[]
  cc: string[]
  alsoOnThread: string[]
}

export interface AddCCResult {
  cc: string[]
  error?: string
}

const isEmail = (value: string) => /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value)

function normalizeEmails(values?: string[] | null): string[] {
  const seen = new Set<string>()
  const result: string[] = []
  for (const value of values ?? []) {
    const trimmed = value.trim()
    const key = trimmed.toLowerCase()
    if (!trimmed || seen.has(key)) continue
    seen.add(key)
    result.push(trimmed)
  }
  return result
}

export function conversationRecipients(conversation: SupportConversation): RecipientSummary {
  const primary = normalizeEmails(conversation.customer_email ? [conversation.customer_email] : [])
  const cc = normalizeEmails(conversation.email_cc)
  const excluded = new Set([...primary, ...cc].map((email) => email.toLowerCase()))
  const alsoOnThread = normalizeEmails(conversation.email_thread_participants)
    .filter((email) => !excluded.has(email.toLowerCase()))
  return { primary, cc, alsoOnThread }
}

export function addCCRecipient(recipients: RecipientSummary, rawEmail: string): AddCCResult {
  const email = rawEmail.trim()
  if (!isEmail(email)) return { cc: recipients.cc, error: 'Enter a valid email address' }
  const key = email.toLowerCase()
  if (recipients.primary.some((value) => value.toLowerCase() === key)) {
    return { cc: recipients.cc, error: 'This is already the primary recipient' }
  }
  if (recipients.cc.some((value) => value.toLowerCase() === key)) {
    return { cc: recipients.cc, error: 'This recipient is already in Cc' }
  }
  return { cc: [...recipients.cc, email] }
}

function contactName(contact: MobileCRMContact): string {
  return [contact.first_name, contact.last_name].filter(Boolean).join(' ') || contact.email || 'Unnamed contact'
}

export function ConversationContactTools({
  workspaceId,
  conversation,
  canEdit,
}: ConversationContactToolsProps) {
  const [addingCC, setAddingCC] = useState(false)
  const [ccDraft, setCCDraft] = useState('')
  const [ccError, setCCError] = useState('')
  const [contactSearchOpen, setContactSearchOpen] = useState(false)
  const [contactSearch, setContactSearch] = useState('')
  const deferredContactSearch = useDeferredValue(contactSearch.trim())
  const updateRecipients = useUpdateConversationEmailRecipients(workspaceId)
  const updateContact = useUpdateConversationCRMContact(workspaceId)
  const recipients = conversationRecipients(conversation)
  const contactsQuery = useQuery({
    queryKey: ['mobile-crm-contacts', workspaceId, deferredContactSearch],
    queryFn: async () => {
      const { data, error } = await mobileContactsService.search(workspaceId, deferredContactSearch)
      if (error || !data) throw new Error(error ?? 'Could not search contacts')
      return data.data
    },
    enabled: canEdit && contactSearchOpen && !!workspaceId && deferredContactSearch.length >= 2,
    staleTime: 30_000,
  })

  useEffect(() => {
    setAddingCC(false)
    setCCDraft('')
    setCCError('')
    setContactSearchOpen(false)
    setContactSearch('')
  }, [conversation.id])

  const updateCC = (ccEmails: string[], onSuccess?: () => void) => {
    updateRecipients.mutate(
      { conversationId: conversation.id, payload: { cc_emails: ccEmails } },
      {
        onSuccess: () => {
          onSuccess?.()
          haptic('notificationSuccess')
        },
        onError: () => {
          toast.error('Could not update Cc recipients')
          haptic('notificationError')
        },
      },
    )
  }

  const saveCC = () => {
    const result = addCCRecipient(recipients, ccDraft)
    if (result.error) {
      setCCError(result.error)
      return
    }
    updateCC(result.cc, () => {
      setAddingCC(false)
      setCCDraft('')
      setCCError('')
    })
  }

  const chooseContact = (contactId: string | null) => {
    updateContact.mutate(
      { conversationId: conversation.id, contactId },
      {
        onSuccess: () => {
          setContactSearchOpen(false)
          setContactSearch('')
          haptic('notificationSuccess')
        },
        onError: () => {
          toast.error('Could not update CRM contact')
          haptic('notificationError')
        },
      },
    )
  }

  return (
    <>
      {(recipients.primary.length > 0 || recipients.cc.length > 0 || recipients.alsoOnThread.length > 0 || canEdit) && (
        <section className="border-t border-border/60 px-4 py-3">
          <div className="mb-2 flex items-center gap-2 text-footnote font-semibold">
            <Mail className="h-4 w-4 text-muted-foreground" />
            Email recipients
          </div>
          <div className="space-y-2">
            {recipients.primary.map((email) => (
              <RecipientRow key={email} label="Primary" email={email} />
            ))}
            {recipients.cc.map((email) => (
              <RecipientRow
                key={email}
                label="Cc"
                email={email}
                remove={canEdit ? () => updateCC(
                  recipients.cc.filter((value) => value.toLowerCase() !== email.toLowerCase()),
                ) : undefined}
                removing={updateRecipients.isPending}
              />
            ))}
            {recipients.alsoOnThread.map((email) => (
              <RecipientRow key={email} label="On thread" email={email} />
            ))}

            {canEdit && (addingCC ? (
              <div className="space-y-1.5 pt-1">
                <div className="flex items-center gap-1">
                  <input
                    autoFocus
                    aria-label="Cc email address"
                    type="email"
                    value={ccDraft}
                    onChange={(event) => {
                      setCCDraft(event.target.value)
                      setCCError('')
                    }}
                    onKeyDown={(event) => {
                      if (event.key === 'Enter') saveCC()
                      if (event.key === 'Escape') setAddingCC(false)
                    }}
                    placeholder="name@example.com"
                    className="h-10 min-w-0 flex-1 rounded-xl border border-input bg-background px-3 text-footnote outline-none"
                  />
                  <Pressable
                    aria-label="Save Cc recipient"
                    disabled={!ccDraft.trim() || updateRecipients.isPending}
                    onPress={saveCC}
                    className="flex h-9 w-9 items-center justify-center rounded-full text-primary"
                  >
                    <Check className="h-4 w-4" />
                  </Pressable>
                  <Pressable
                    aria-label="Cancel adding Cc recipient"
                    onPress={() => {
                      setAddingCC(false)
                      setCCError('')
                    }}
                    className="flex h-9 w-9 items-center justify-center rounded-full text-muted-foreground"
                  >
                    <X className="h-4 w-4" />
                  </Pressable>
                </div>
                {ccError && <p role="alert" className="text-caption text-destructive">{ccError}</p>}
              </div>
            ) : (
              <Pressable
                onPress={() => setAddingCC(true)}
                className="h-auto min-h-8 w-auto min-w-0 text-footnote font-medium text-primary"
              >
                Add Cc recipient
              </Pressable>
            ))}
          </div>
        </section>
      )}

      {(conversation.crm_contact_id || canEdit) && (
        <section className="border-t border-border/60 px-4 py-3">
          <div className="mb-2 flex items-center gap-2 text-footnote font-semibold">
            <UserRoundCheck className="h-4 w-4 text-muted-foreground" />
            CRM contact
          </div>
          {conversation.crm_contact_id && !contactSearchOpen && (
            <div className="flex items-center justify-between gap-2">
              <div className="min-w-0">
                <p className="truncate text-footnote text-foreground">
                  {conversation.customer_name || conversation.customer_email || 'Linked contact'}
                </p>
                <p className="text-caption text-muted-foreground">Linked to this conversation</p>
              </div>
              {canEdit && (
                <div className="flex items-center gap-1">
                  <Pressable
                    onPress={() => setContactSearchOpen(true)}
                    className="h-auto min-h-8 w-auto min-w-0 px-2 text-footnote font-medium text-primary"
                  >
                    Change
                  </Pressable>
                  <Pressable
                    aria-label="Unlink CRM contact"
                    disabled={updateContact.isPending}
                    onPress={() => chooseContact(null)}
                    className="flex h-8 w-8 items-center justify-center rounded-full text-muted-foreground"
                  >
                    <X className="h-4 w-4" />
                  </Pressable>
                </div>
              )}
            </div>
          )}

          {canEdit && !conversation.crm_contact_id && !contactSearchOpen && (
            <Pressable
              onPress={() => setContactSearchOpen(true)}
              className="flex h-auto min-h-9 w-auto min-w-0 items-center gap-2 text-footnote font-medium text-primary"
            >
              <Link2 className="h-4 w-4" />
              Link CRM contact
            </Pressable>
          )}

          {canEdit && contactSearchOpen && (
            <div className="space-y-2">
              <div className="relative">
                <Search className="pointer-events-none absolute left-3 top-3 h-4 w-4 text-muted-foreground" />
                <input
                  autoFocus
                  aria-label="Search CRM contacts"
                  value={contactSearch}
                  onChange={(event) => setContactSearch(event.target.value)}
                  placeholder="Search name or email"
                  className="h-10 w-full rounded-xl border border-input bg-background pl-9 pr-10 text-footnote outline-none"
                />
                <button
                  type="button"
                  aria-label="Close contact search"
                  onClick={() => setContactSearchOpen(false)}
                  className="absolute right-2 top-2 flex h-6 w-6 items-center justify-center text-muted-foreground"
                >
                  <X className="h-4 w-4" />
                </button>
              </div>
              {contactsQuery.isFetching && (
                <div className="flex justify-center py-3"><Spinner size={16} /></div>
              )}
              {!contactsQuery.isFetching && deferredContactSearch.length >= 2 && (contactsQuery.data ?? []).length === 0 && (
                <p className="py-2 text-footnote text-muted-foreground">No contacts found.</p>
              )}
              {(contactsQuery.data ?? []).map((contact) => (
                <Pressable
                  key={contact.id}
                  aria-label={`Link CRM contact ${contactName(contact)}`}
                  disabled={updateContact.isPending}
                  onPress={() => chooseContact(contact.id)}
                  className="flex h-auto min-h-11 w-full items-center justify-between gap-2 rounded-xl bg-muted/50 px-3 py-2 text-left"
                >
                  <span className="min-w-0">
                    <span className="block truncate text-footnote font-medium">{contactName(contact)}</span>
                    {contact.email && <span className="block truncate text-caption text-muted-foreground">{contact.email}</span>}
                  </span>
                  <Link2 className="h-4 w-4 shrink-0 text-primary" />
                </Pressable>
              ))}
            </div>
          )}
        </section>
      )}
    </>
  )
}

function RecipientRow({
  label,
  email,
  remove,
  removing,
}: {
  label: string
  email: string
  remove?: () => void
  removing?: boolean
}) {
  return (
    <div className="grid grid-cols-[64px_minmax(0,1fr)_32px] items-center gap-2 text-footnote">
      <span className="text-muted-foreground">{label}</span>
      <span className="truncate text-right text-foreground">{email}</span>
      {remove ? (
        <button
          type="button"
          aria-label={`Remove Cc recipient ${email}`}
          disabled={removing}
          onClick={remove}
          className="flex h-8 w-8 items-center justify-center rounded-full text-muted-foreground disabled:opacity-40"
        >
          <X className="h-3.5 w-3.5" />
        </button>
      ) : <span />}
    </div>
  )
}
