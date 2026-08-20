import { Building2, ExternalLink, MessageCircle, UserRound } from 'lucide-react'
import { toast } from 'sonner'
import {
  useUpdateConversationCRMCompany,
  useVisitorContext,
  type ConversationStatus,
} from '@helpin-ai/support-core'
import { formatRelativeTime } from '@mobile/inbox/inbox-helpers'
import { haptic } from '@mobile/lib/haptics'
import { Badge, type BadgeTone } from '@mobile/ui/badge'
import { Pressable } from '@mobile/ui/pressable'
import { Spinner } from '@mobile/ui/spinner'

interface CustomerContextProps {
  workspaceId: string
  conversationId: string
  canEdit: boolean
  channel: string
  onOpenConversation: (conversationId: string) => void
}

const STATUS_LABELS: Record<ConversationStatus, string> = {
  open: 'Open',
  waiting_on_customer: 'Waiting',
  resolved: 'Resolved',
  spam: 'Spam',
}

const STATUS_TONES: Record<ConversationStatus, BadgeTone> = {
  open: 'primary',
  waiting_on_customer: 'warning',
  resolved: 'success',
  spam: 'neutral',
}

const CHANNEL_LABELS: Record<string, string> = {
  widget: 'Chat',
  email: 'Email',
  api: 'API',
  internal: 'Internal',
}

const PRIORITY_COMPANY_KEYS = [
  'plan',
  'subscription_status',
  'support_tier',
  'seats_used',
  'seats_total',
  'trial_ends_at',
  'renewal_date',
  'mrr',
  'arr',
] as const
const HIDDEN_COMPANY_KEYS = new Set(['sdk_company_id', 'sdk_created_at', 'currency'])
const SYSTEM_CRM_CUSTOM_PROPERTY_KEYS = new Set(['agent_enrichment'])
const isSystemCRMCustomProperty = (key: string) => SYSTEM_CRM_CUSTOM_PROPERTY_KEYS.has(key)

export function formatContextValue(value: unknown): string | null {
  if (value === null || value === undefined || value === '') return null
  if (typeof value === 'boolean') return value ? 'Yes' : 'No'
  if (typeof value === 'string' || typeof value === 'number') return String(value)
  return JSON.stringify(value)
}

export function formatLocale(locale?: string | null): string | null {
  if (!locale?.trim()) return null
  try {
    const parsed = new Intl.Locale(locale)
    const language = new Intl.DisplayNames(undefined, { type: 'language' }).of(parsed.language)
    const region = parsed.region
      ? new Intl.DisplayNames(undefined, { type: 'region' }).of(parsed.region)
      : null
    return language && region ? `${language} (${region})` : language ?? locale
  } catch {
    return locale
  }
}

export function formatLocalTime(timezone?: string | null): string | null {
  if (!timezone) return null
  try {
    return new Intl.DateTimeFormat(undefined, {
      timeZone: timezone,
      hour: 'numeric',
      minute: '2-digit',
    }).format(new Date())
  } catch {
    return null
  }
}

function humanize(value: string): string {
  return value.replaceAll('_', ' ').replace(/\b\w/g, (letter) => letter.toUpperCase())
}

function formatCompanyValue(key: string, value: unknown, currency: string): string | null {
  if (value === null || value === undefined || value === '') return null
  if (typeof value === 'boolean') return value ? 'Yes' : 'No'
  if ((key === 'mrr' || key === 'arr') && typeof value === 'number') {
    return new Intl.NumberFormat(undefined, {
      style: 'currency',
      currency,
      maximumFractionDigits: 0,
    }).format(value)
  }
  if (key.endsWith('_at') || key === 'renewal_date') {
    const date = new Date(String(value))
    if (!Number.isNaN(date.getTime())) {
      return new Intl.DateTimeFormat(undefined, { dateStyle: 'medium' }).format(date)
    }
  }
  if (typeof value === 'number') return new Intl.NumberFormat().format(value)
  return humanize(String(value))
}

function Section({
  title,
  icon,
  children,
}: {
  title: string
  icon: React.ReactNode
  children: React.ReactNode
}) {
  return (
    <section className="border-t border-border/60 px-4 py-3">
      <div className="mb-2 flex items-center gap-2 text-footnote font-semibold text-foreground">
        <span className="text-muted-foreground">{icon}</span>
        {title}
      </div>
      <div className="space-y-2">{children}</div>
    </section>
  )
}

function DetailRow({
  label,
  value,
  href,
}: {
  label: string
  value?: string | null
  href?: string
}) {
  if (!value) return null
  return (
    <div className="grid grid-cols-[100px_minmax(0,1fr)] items-start gap-2 text-footnote">
      <span className="text-muted-foreground">{label}</span>
      {href ? (
        <a
          href={href}
          target="_blank"
          rel="noopener noreferrer"
          className="flex min-w-0 items-center justify-end gap-1 text-right text-primary"
        >
          <span className="truncate">{value}</span>
          <ExternalLink className="h-3 w-3 shrink-0" />
        </a>
      ) : (
        <span className="min-w-0 break-words text-right text-foreground">{value}</span>
      )}
    </div>
  )
}

export function CustomerContext({
  workspaceId,
  conversationId,
  canEdit,
  channel,
  onOpenConversation,
}: CustomerContextProps) {
  const contextQuery = useVisitorContext(workspaceId, conversationId)
  const updateCompany = useUpdateConversationCRMCompany(workspaceId)
  const visitor = contextQuery.data

  if (contextQuery.isPending) {
    return (
      <div className="flex justify-center border-t border-border/60 py-5" aria-label="Loading customer context">
        <Spinner size={16} />
      </div>
    )
  }
  if (contextQuery.isError || !visitor) {
    return (
      <div className="flex items-center justify-between gap-3 border-t border-border/60 px-4 py-3">
        <span className="text-footnote text-muted-foreground">Customer context could not be loaded.</span>
        <Pressable
          onPress={() => void contextQuery.refetch()}
          className="h-auto min-h-8 w-auto min-w-0 rounded-full px-3 text-footnote font-medium text-primary"
        >
          Retry
        </Pressable>
      </div>
    )
  }

  const { contact, device, location, company } = visitor
  const customContactEntries = Object.entries(contact?.custom_properties ?? {})
    .filter(([key, value]) => !isSystemCRMCustomProperty(key) && formatContextValue(value))
  const locationValue = [location?.city_name, location?.region_name, location?.country_name]
    .filter(Boolean)
    .join(', ')
  const browserAndOS = device
    ? [
        [device.browser, device.browser_version].filter(Boolean).join(' '),
        [device.os, device.os_version].filter(Boolean).join(' '),
      ].filter(Boolean).join(' · ')
    : null
  const hasContactDetails = Boolean(
    contact ||
    locationValue ||
    location?.last_page_url ||
    location?.timezone ||
    location?.locale ||
    browserAndOS,
  )
  const properties = company?.custom_properties ?? {}
  const currency = typeof properties.currency === 'string' ? properties.currency : 'USD'
  const companyEntries = [
    ...PRIORITY_COMPANY_KEYS
      .map((key) => [key, properties[key]] as const)
      .filter(([, value]) => value !== null && value !== undefined && value !== ''),
    ...Object.entries(properties).filter(([key, value]) =>
      !PRIORITY_COMPANY_KEYS.includes(key as typeof PRIORITY_COMPANY_KEYS[number]) &&
      !HIDDEN_COMPANY_KEYS.has(key) &&
      !isSystemCRMCustomProperty(key) &&
      value !== null &&
      value !== undefined &&
      value !== ''),
  ]
  const companyOptions = visitor.company_options ?? []

  const chooseCompany = (companyId: string | null) => {
    updateCompany.mutate(
      { conversationId, companyId },
      {
        onSuccess: () => haptic('notificationSuccess'),
        onError: () => {
          haptic('notificationError')
          toast.error('Could not update company')
        },
      },
    )
  }

  return (
    <>
      {hasContactDetails && (
        <Section title="Contact details" icon={<UserRound className="h-4 w-4" />}>
          <DetailRow label="Job title" value={contact?.job_title} />
          <DetailRow label="Lifecycle" value={contact?.lifecycle_stage ? humanize(contact.lifecycle_stage) : null} />
          <DetailRow label="Lead status" value={contact?.lead_status ? humanize(contact.lead_status) : null} />
          <DetailRow label="Phone" value={contact?.phone} />
          {customContactEntries.map(([key, value]) => (
            <DetailRow key={key} label={humanize(key)} value={formatContextValue(value)} />
          ))}

          <div className="pt-1 text-caption font-semibold uppercase tracking-wide text-muted-foreground">
            Current visit
          </div>
          <DetailRow label="Channel" value={CHANNEL_LABELS[channel] ?? humanize(channel)} />
          <DetailRow
            label="Current page"
            value={location?.last_page_url?.replace(/^https?:\/\//, '')}
            href={location?.last_page_url ?? undefined}
          />
          <DetailRow label="Location" value={locationValue} />
          <DetailRow label="Local time" value={formatLocalTime(location?.timezone)} />
          <DetailRow label="Language" value={formatLocale(location?.locale)} />
          <DetailRow label="Device" value={device?.device_type ? humanize(device.device_type) : null} />
          <DetailRow label="Browser & OS" value={browserAndOS} />
          <DetailRow
            label="First seen"
            value={visitor.session_created_at ? formatRelativeTime(visitor.session_created_at) : null}
          />
        </Section>
      )}

      {(company || companyOptions.length > 0 || visitor.company_context_status === 'error') && (
        <Section title="Company details" icon={<Building2 className="h-4 w-4" />}>
          {visitor.company_context_status === 'error' ? (
            <div className="flex items-center justify-between gap-2">
              <span className="text-footnote text-muted-foreground">Company details could not be loaded.</span>
              <Pressable
                onPress={() => void contextQuery.refetch()}
                className="h-auto min-h-8 w-auto min-w-0 rounded-full px-3 text-footnote font-medium text-primary"
              >
                Retry
              </Pressable>
            </div>
          ) : (
            <>
              <DetailRow label="Company" value={company?.name} />
              <DetailRow label="Domain" value={company?.domain} />
              <DetailRow label="Industry" value={company?.industry} />
              <DetailRow
                label="Employees"
                value={company?.employee_count != null ? new Intl.NumberFormat().format(company.employee_count) : null}
              />
              {companyEntries.map(([key, value]) => (
                <DetailRow
                  key={key}
                  label={humanize(key)}
                  value={formatCompanyValue(key, value, currency)}
                />
              ))}
              {canEdit && companyOptions.length > 0 && (
                <select
                  aria-label="Conversation company"
                  value={company?.id ?? ''}
                  disabled={updateCompany.isPending}
                  onChange={(event) => chooseCompany(event.target.value || null)}
                  className="h-10 w-full rounded-xl border border-input bg-background px-3 text-footnote outline-none"
                >
                  <option value="">No linked company</option>
                  {companyOptions.map((option) => (
                    <option key={option.id} value={option.id}>{option.name}</option>
                  ))}
                </select>
              )}
              {canEdit && company && (
                <Pressable
                  disabled={updateCompany.isPending}
                  onPress={() => chooseCompany(null)}
                  className="h-auto min-h-8 w-auto min-w-0 text-footnote font-medium text-primary"
                >
                  Clear company
                </Pressable>
              )}
            </>
          )}
        </Section>
      )}

      {visitor.other_conversations.length > 0 && (
        <Section title={`Other conversations (${Math.max(visitor.total_conversations - 1, visitor.other_conversations.length)})`} icon={<MessageCircle className="h-4 w-4" />}>
          <div className="space-y-1">
            {visitor.other_conversations.map((conversation) => (
              <Pressable
                key={conversation.id}
                aria-label={`Open conversation ${conversation.display_id}: ${conversation.subject}`}
                onPress={() => onOpenConversation(conversation.id)}
                className="flex h-auto min-h-11 w-full items-center gap-2 rounded-xl bg-muted/50 px-3 py-2 text-left"
              >
                <span className="shrink-0 text-caption text-muted-foreground">#{conversation.display_id}</span>
                <span className="min-w-0 flex-1 truncate text-footnote font-medium">{conversation.subject}</span>
                <Badge tone={STATUS_TONES[conversation.status]}>
                  {STATUS_LABELS[conversation.status]}
                </Badge>
              </Pressable>
            ))}
          </div>
        </Section>
      )}

      {!hasContactDetails && !company && companyOptions.length === 0 && visitor.other_conversations.length === 0 && (
        <div className="border-t border-border/60 px-4 py-4 text-footnote text-muted-foreground">
          No additional customer context yet.
        </div>
      )}
    </>
  )
}
