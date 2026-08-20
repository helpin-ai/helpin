import { useDeferredValue, useEffect, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Check, Link2, Mail, Pencil, Plus, Search, UserRoundCheck, X } from 'lucide-react'
import { toast } from 'sonner'
import {
  useUpdateConversationCRMContact,
  useUpdateConversationEmailRecipients,
  type SupportConversation,
} from '@helpin-ai/support-core'
import {
  mobileContactsService,
  type CreateMobileCRMContactRequest,
  type MobileCRMContact,
  type MobileLeadStatus,
  type MobileLifecycleStage,
  type UpdateMobileCRMContactRequest,
} from '@mobile/lib/services/mobile-contacts-service'
import { haptic } from '@mobile/lib/haptics'
import { getUpgradeRequiredReason, type UpgradeRequiredReason } from '@mobile/lib/upgrade-required'
import { Pressable } from '@mobile/ui/pressable'
import { Spinner } from '@mobile/ui/spinner'
import { UpgradeRequiredSheet } from '@mobile/ui/upgrade-required-sheet'

interface ConversationContactToolsProps {
  workspaceId: string
  conversation: SupportConversation
  canEdit: boolean
  canReadCRM: boolean
  canEditCRM: boolean
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

interface ContactFormValues {
  first_name: string
  last_name: string
  email: string
  phone: string
  job_title: string
  lifecycle_stage: MobileLifecycleStage
  lead_status: MobileLeadStatus
  source: string
}

const LIFECYCLE_OPTIONS: Array<{ value: MobileLifecycleStage; label: string }> = [
  { value: 'subscriber', label: 'Subscriber' },
  { value: 'lead', label: 'Lead' },
  { value: 'marketing_qualified', label: 'Marketing Qualified' },
  { value: 'sales_qualified', label: 'Sales Qualified' },
  { value: 'opportunity', label: 'Opportunity' },
  { value: 'customer', label: 'Customer' },
  { value: 'evangelist', label: 'Evangelist' },
]

const LEAD_STATUS_OPTIONS: Array<{ value: MobileLeadStatus; label: string }> = [
  { value: 'new', label: 'New' },
  { value: 'open', label: 'Open' },
  { value: 'in_progress', label: 'In Progress' },
  { value: 'unqualified', label: 'Unqualified' },
]

const SOURCE_OPTIONS = ['web', 'support', 'referral', 'social', 'event', 'cold_outreach', 'other']

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

export function splitContactName(value?: string | null): { firstName: string; lastName: string } {
  const parts = value?.trim().replace(/\s+/g, ' ').split(' ').filter(Boolean) ?? []
  return {
    firstName: parts[0] ?? '',
    lastName: parts.slice(1).join(' '),
  }
}

export function initialContactValues(
  conversation: Pick<SupportConversation, 'customer_name' | 'customer_email'>,
): ContactFormValues {
  const fallbackName = conversation.customer_name?.trim()
    || conversation.customer_email?.split('@')[0]
    || 'Customer'
  const { firstName, lastName } = splitContactName(fallbackName)
  return {
    first_name: firstName || 'Customer',
    last_name: lastName,
    email: conversation.customer_email?.trim() ?? '',
    phone: '',
    job_title: '',
    lifecycle_stage: 'subscriber',
    lead_status: 'new',
    source: 'support',
  }
}

function contactFormValues(contact: MobileCRMContact): ContactFormValues {
  return {
    first_name: contact.first_name,
    last_name: contact.last_name ?? '',
    email: contact.email ?? '',
    phone: contact.phone ?? '',
    job_title: contact.job_title ?? '',
    lifecycle_stage: contact.lifecycle_stage,
    lead_status: contact.lead_status,
    source: contact.source ?? '',
  }
}

function contactPayload(values: ContactFormValues): UpdateMobileCRMContactRequest {
  return {
    first_name: values.first_name.trim(),
    last_name: values.last_name.trim() || undefined,
    email: values.email.trim() || undefined,
    phone: values.phone.trim() || undefined,
    job_title: values.job_title.trim() || undefined,
    lifecycle_stage: values.lifecycle_stage,
    lead_status: values.lead_status,
    source: values.source.trim() || undefined,
  }
}

export function ConversationContactTools({
  workspaceId,
  conversation,
  canEdit,
  canReadCRM,
  canEditCRM,
}: ConversationContactToolsProps) {
  const queryClient = useQueryClient()
  const [addingCC, setAddingCC] = useState(false)
  const [ccDraft, setCCDraft] = useState('')
  const [ccError, setCCError] = useState('')
  const [contactSearchOpen, setContactSearchOpen] = useState(false)
  const [contactSearch, setContactSearch] = useState('')
  const [contactFormMode, setContactFormMode] = useState<'create' | 'edit' | null>(null)
  const [upgradeReason, setUpgradeReason] = useState<UpgradeRequiredReason | null>(null)
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
    enabled: canEdit && canReadCRM && contactSearchOpen && !!workspaceId && deferredContactSearch.length >= 2,
    staleTime: 30_000,
  })
  const linkedContactQuery = useQuery({
    queryKey: ['mobile-crm-contact', workspaceId, conversation.crm_contact_id],
    queryFn: async () => {
      const { data, error } = await mobileContactsService.get(workspaceId, conversation.crm_contact_id ?? '')
      if (error || !data) throw new Error(error ?? 'Could not load CRM contact')
      return data
    },
    enabled: canReadCRM && !!workspaceId && !!conversation.crm_contact_id,
    staleTime: 30_000,
  })
  const createContact = useMutation({
    mutationFn: async (payload: CreateMobileCRMContactRequest) => {
      const { data, error } = await mobileContactsService.create(payload)
      if (error || !data) throw new Error(error ?? 'Could not create CRM contact')
      return data
    },
  })
  const updateCRMContact = useMutation({
    mutationFn: async ({ contactId, payload }: { contactId: string; payload: UpdateMobileCRMContactRequest }) => {
      const { data, error } = await mobileContactsService.update(workspaceId, contactId, payload)
      if (error || !data) throw new Error(error ?? 'Could not update CRM contact')
      return data
    },
  })

  useEffect(() => {
    setAddingCC(false)
    setCCDraft('')
    setCCError('')
    setContactSearchOpen(false)
    setContactSearch('')
    setContactFormMode(null)
    setUpgradeReason(null)
  }, [conversation.id])

  const handleContactError = (error: unknown, fallback: string) => {
    const reason = getUpgradeRequiredReason(error)
    if (reason) setUpgradeReason(reason)
    else toast.error(fallback)
    haptic('notificationError')
  }

  const invalidateContacts = async (contactId?: string) => {
    await queryClient.invalidateQueries({ queryKey: ['mobile-crm-contacts', workspaceId] })
    if (contactId) {
      await queryClient.invalidateQueries({ queryKey: ['mobile-crm-contact', workspaceId, contactId] })
    }
  }

  const createAndLinkContact = async (values: ContactFormValues) => {
    try {
      const contact = await createContact.mutateAsync({
        workspace_id: workspaceId,
        ...contactPayload(values),
        first_name: values.first_name.trim(),
        email: values.email.trim(),
        lifecycle_stage: values.lifecycle_stage,
        lead_status: values.lead_status,
      })
      await updateContact.mutateAsync({ conversationId: conversation.id, contactId: contact.id })
      await invalidateContacts(contact.id)
      setContactFormMode(null)
      haptic('notificationSuccess')
    } catch (error) {
      handleContactError(error, 'Could not create or link CRM contact')
    }
  }

  const saveContact = async (values: ContactFormValues) => {
    const contactId = conversation.crm_contact_id
    if (!contactId) return
    try {
      await updateCRMContact.mutateAsync({ contactId, payload: contactPayload(values) })
      await invalidateContacts(contactId)
      setContactFormMode(null)
      haptic('notificationSuccess')
    } catch (error) {
      handleContactError(error, 'Could not update CRM contact')
    }
  }

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
        onError: (error) => handleContactError(error, 'Could not update CRM contact'),
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

      {canReadCRM && (conversation.crm_contact_id || canEdit) && (
        <section className="border-t border-border/60 px-4 py-3">
          <div className="mb-2 flex items-center gap-2 text-footnote font-semibold">
            <UserRoundCheck className="h-4 w-4 text-muted-foreground" />
            CRM contact
          </div>
          {contactFormMode === 'create' && (
            <ContactForm
              initialValues={initialContactValues(conversation)}
              submitLabel="Create and link"
              requireEmail
              sourceSelect
              pending={createContact.isPending || updateContact.isPending}
              onCancel={() => setContactFormMode(null)}
              onSubmit={createAndLinkContact}
            />
          )}

          {contactFormMode === 'edit' && linkedContactQuery.data && (
            <ContactForm
              initialValues={contactFormValues(linkedContactQuery.data)}
              submitLabel="Save contact"
              pending={updateCRMContact.isPending}
              onCancel={() => setContactFormMode(null)}
              onSubmit={saveContact}
            />
          )}

          {!contactFormMode && conversation.crm_contact_id && !contactSearchOpen && (
            <div className="space-y-3">
              {linkedContactQuery.isPending && <div className="flex justify-center py-3"><Spinner size={16} /></div>}
              {linkedContactQuery.isError && (
                <div className="flex items-center justify-between gap-2">
                  <p className="text-footnote text-muted-foreground">CRM contact could not be loaded.</p>
                  <Pressable
                    onPress={() => void linkedContactQuery.refetch()}
                    className="h-auto min-h-8 w-auto min-w-0 text-footnote font-medium text-primary"
                  >
                    Retry
                  </Pressable>
                </div>
              )}
              {linkedContactQuery.data && (
                <>
                  <div className="flex items-start justify-between gap-2">
                    <div className="min-w-0">
                      <p className="truncate text-footnote font-medium text-foreground">{contactName(linkedContactQuery.data)}</p>
                      <p className="text-caption text-muted-foreground">
                        {linkedContactQuery.data.display_id ? `#${linkedContactQuery.data.display_id}` : 'Linked to this conversation'}
                      </p>
                    </div>
                    {canEditCRM && (
                      <Pressable
                        aria-label="Edit CRM contact"
                        onPress={() => setContactFormMode('edit')}
                        className="flex h-8 w-8 items-center justify-center rounded-full text-primary"
                      >
                        <Pencil className="h-3.5 w-3.5" />
                      </Pressable>
                    )}
                  </div>
                  <div className="space-y-1.5 rounded-xl bg-muted/40 px-3 py-2.5">
                    <ContactDetail label="Email" value={linkedContactQuery.data.email} />
                    <ContactDetail label="Phone" value={linkedContactQuery.data.phone} />
                    <ContactDetail label="Title" value={linkedContactQuery.data.job_title} />
                    <ContactDetail label="Source" value={linkedContactQuery.data.source} />
                    <ContactDetail label="Stage" value={humanize(linkedContactQuery.data.lifecycle_stage)} />
                    <ContactDetail label="Status" value={humanize(linkedContactQuery.data.lead_status)} />
                  </div>
                </>
              )}
              {canEdit && (
                <div className="flex items-center gap-3">
                  <Pressable
                    onPress={() => setContactSearchOpen(true)}
                    className="h-auto min-h-8 w-auto min-w-0 text-footnote font-medium text-primary"
                  >
                    Change contact
                  </Pressable>
                  <Pressable
                    aria-label="Unlink CRM contact"
                    disabled={updateContact.isPending}
                    onPress={() => chooseContact(null)}
                    className="h-auto min-h-8 w-auto min-w-0 text-footnote font-medium text-muted-foreground"
                  >
                    Unlink
                  </Pressable>
                </div>
              )}
            </div>
          )}

          {!contactFormMode && canEdit && !conversation.crm_contact_id && !contactSearchOpen && (
            <div className="space-y-2">
              {canEditCRM && (
                <Pressable
                  onPress={() => setContactFormMode('create')}
                  className="flex h-auto min-h-10 w-full items-center justify-center gap-2 rounded-xl bg-primary px-3 text-footnote font-medium text-primary-foreground"
                >
                  <Plus className="h-4 w-4" />
                  Create and link contact
                </Pressable>
              )}
              <Pressable
                onPress={() => setContactSearchOpen(true)}
                className="flex h-auto min-h-9 w-auto min-w-0 items-center gap-2 text-footnote font-medium text-primary"
              >
                <Link2 className="h-4 w-4" />
                Link existing contact
              </Pressable>
            </div>
          )}

          {!contactFormMode && canEdit && contactSearchOpen && (
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
      <UpgradeRequiredSheet
        reason={upgradeReason}
        onOpenChange={(open) => { if (!open) setUpgradeReason(null) }}
      />
    </>
  )
}

function humanize(value: string): string {
  return value.replace(/_/g, ' ').replace(/\b\w/g, (letter) => letter.toUpperCase())
}

function ContactDetail({ label, value }: { label: string; value?: string | null }) {
  return (
    <div className="grid grid-cols-[72px_minmax(0,1fr)] gap-2 text-footnote">
      <span className="text-muted-foreground">{label}</span>
      <span className="truncate text-right text-foreground">{value?.trim() || '—'}</span>
    </div>
  )
}

function ContactForm({
  initialValues: values,
  submitLabel,
  requireEmail = false,
  sourceSelect = false,
  pending,
  onCancel,
  onSubmit,
}: {
  initialValues: ContactFormValues
  submitLabel: string
  requireEmail?: boolean
  sourceSelect?: boolean
  pending: boolean
  onCancel: () => void
  onSubmit: (values: ContactFormValues) => void | Promise<void>
}) {
  const [form, setForm] = useState(values)
  const emailValid = !form.email.trim() ? !requireEmail : isEmail(form.email)
  const canSubmit = !!form.first_name.trim() && emailValid
  const set = <K extends keyof ContactFormValues>(key: K, value: ContactFormValues[K]) => {
    setForm((current) => ({ ...current, [key]: value }))
  }

  return (
    <div className="space-y-3">
      <div className="grid grid-cols-2 gap-2">
        <ContactInput
          label="First name"
          required
          value={form.first_name}
          onChange={(value) => set('first_name', value)}
        />
        <ContactInput label="Last name" value={form.last_name} onChange={(value) => set('last_name', value)} />
      </div>
      <ContactInput
        label="Email"
        required={requireEmail}
        type="email"
        value={form.email}
        onChange={(value) => set('email', value)}
      />
      <div className="grid grid-cols-2 gap-2">
        <ContactInput label="Phone" type="tel" value={form.phone} onChange={(value) => set('phone', value)} />
        <ContactInput label="Job title" value={form.job_title} onChange={(value) => set('job_title', value)} />
      </div>
      <ContactSelect
        label="Lifecycle stage"
        value={form.lifecycle_stage}
        options={LIFECYCLE_OPTIONS}
        onChange={(value) => set('lifecycle_stage', value as MobileLifecycleStage)}
      />
      <ContactSelect
        label="Lead status"
        value={form.lead_status}
        options={LEAD_STATUS_OPTIONS}
        onChange={(value) => set('lead_status', value as MobileLeadStatus)}
      />
      {sourceSelect ? (
        <ContactSelect
          label="Source"
          value={form.source}
          options={SOURCE_OPTIONS.map((source) => ({ value: source, label: humanize(source) }))}
          onChange={(value) => set('source', value)}
        />
      ) : (
        <ContactInput label="Source" value={form.source} onChange={(value) => set('source', value)} />
      )}
      <div className="flex gap-2 pt-1">
        <Pressable
          disabled={pending}
          onPress={onCancel}
          className="flex h-auto min-h-10 flex-1 items-center justify-center rounded-xl border border-border text-footnote font-medium"
        >
          Cancel
        </Pressable>
        <Pressable
          disabled={!canSubmit || pending}
          onPress={() => void onSubmit(form)}
          className="flex h-auto min-h-10 flex-1 items-center justify-center rounded-xl bg-primary text-footnote font-medium text-primary-foreground"
        >
          {pending ? <Spinner size={16} /> : submitLabel}
        </Pressable>
      </div>
    </div>
  )
}

function ContactInput({
  label,
  value,
  onChange,
  type = 'text',
  required = false,
}: {
  label: string
  value: string
  onChange: (value: string) => void
  type?: 'text' | 'email' | 'tel'
  required?: boolean
}) {
  return (
    <label className="space-y-1 text-caption text-muted-foreground">
      <span>{label}{required ? ' *' : ''}</span>
      <input
        aria-label={label}
        type={type}
        value={value}
        required={required}
        onChange={(event) => onChange(event.target.value)}
        className="h-10 w-full rounded-xl border border-input bg-background px-3 text-footnote text-foreground outline-none focus:border-ring"
      />
    </label>
  )
}

function ContactSelect({
  label,
  value,
  options,
  onChange,
}: {
  label: string
  value: string
  options: Array<{ value: string; label: string }>
  onChange: (value: string) => void
}) {
  return (
    <label className="space-y-1 text-caption text-muted-foreground">
      <span>{label}</span>
      <select
        aria-label={label}
        value={value}
        onChange={(event) => onChange(event.target.value)}
        className="h-10 w-full rounded-xl border border-input bg-background px-3 text-footnote text-foreground outline-none focus:border-ring"
      >
        {options.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
      </select>
    </label>
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
