import { useEffect, useMemo, useState } from 'react'
import { Link } from '@tanstack/react-router'
import { QuietEmptyState, QuietIconAction, QuietSearchInput, QuietStatusText, QuietTextAction } from '@/components/design-system/quiet'
import { Button } from '@/components/ui/button'
import { ArrowReloadHorizontalIcon, ArrowRight01Icon } from '@/lib/icons'
import type { CustomerPortalRequestScope, CustomerPortalRequest } from '@/lib/services/customerPortalService'
import { cn } from '@/lib/utils'
import { requestedBy, requestNumber } from './portalRequestFormat'
import { formatPortalReplyBy, formatPortalAbsoluteTime, formatPortalRelativeTime } from './portalTime'
import { usePortalRequestSearch } from './usePortalRequestSearch'

type RequestStatus = CustomerPortalRequest['status']
type RequestFilter = 'all' | RequestStatus

const requestStatus: Record<RequestStatus, { label: string; group: string; tone: 'current' | 'blocker' | 'positive' }> = {
  waiting_on_customer: { label: 'Waiting on you', group: 'Waiting on you', tone: 'blocker' },
  active: { label: 'Active', group: 'Open', tone: 'current' },
  resolved: { label: 'Resolved', group: 'Resolved', tone: 'positive' },
}
const groupOrder: RequestStatus[] = ['waiting_on_customer', 'active', 'resolved']
const filters: { value: RequestFilter; label: string }[] = [
  { value: 'all', label: 'All' },
  { value: 'waiting_on_customer', label: 'Waiting on you' },
  { value: 'active', label: 'Active' },
  { value: 'resolved', label: 'Resolved' },
]
const PAGE_SIZE = 10

function statusOf(request: CustomerPortalRequest): RequestStatus {
  return request.status in requestStatus ? request.status : 'active'
}

export function RequestStatusText({ status }: { status: RequestStatus }) {
  const meta = requestStatus[status] ?? requestStatus.active
  return <QuietStatusText tone={meta.tone}>{meta.label}</QuietStatusText>
}

/** Whose requests a list shows: the customer's, a company's, or those copied to them. */
type RequestListKind = 'own' | 'company' | 'copied'

function repliedBy(request: CustomerPortalRequest, supportName: string, kind: RequestListKind) {
  // In a shared list someone else may have written the latest customer reply.
  if (request.last_message_from === 'customer') return kind === 'own' ? 'You replied' : kind === 'company' ? 'Your team replied' : 'Customer replied'
  if (request.last_message_from === 'support') return `${supportName} replied`
  return null
}

function matchesSearch(request: CustomerPortalRequest, search: string) {
  if (!search) return true
  const haystack = [request.subject, request.last_message_preview, requestNumber(request), request.number ? String(request.number) : '', request.requested_by]
  return haystack.some((value) => value?.toLowerCase().includes(search))
}

/**
 * PortalRequestTable lists the customer's requests the way My Work lists
 * tasks: filter pills with counts and a search in a hairline toolbar, then
 * collapsible status groups of table rows. Filtering happens here so counts,
 * groups and the status column always agree. A company list (companyName set)
 * and a copied list also say who asked for each request.
 */
export function PortalRequestTable({ requests, slug, supportName, onRefresh, companyName, copied = false, scope = { kind: 'mine' } }: {
  requests: CustomerPortalRequest[]
  slug: string
  supportName: string
  onRefresh: () => void
  companyName?: string
  copied?: boolean
  /** The list shown, so search covers its messages on the server. */
  scope?: CustomerPortalRequestScope
}) {
  const kind: RequestListKind = copied ? 'copied' : companyName ? 'company' : 'own'
  const [filter, setFilter] = useState<RequestFilter>('all')
  const [search, setSearch] = useState('')
  const [now, setNow] = useState(() => Date.now())
  useEffect(() => {
    const timer = window.setInterval(() => setNow(Date.now()), 60_000)
    return () => window.clearInterval(timer)
  }, [])

  const query = search.trim().toLowerCase()
  // Two characters or more also search every message on the server.
  const matches = usePortalRequestSearch(slug, scope, query)
  const searched = useMemo(
    () => requests.filter((request) => matchesSearch(request, query) || Boolean(matches?.has(request.reference))),
    [requests, query, matches],
  )
  const count = (value: RequestFilter) => value === 'all' ? searched.length : searched.filter((request) => statusOf(request) === value).length
  const groups = groupOrder
    .filter((status) => filter === 'all' || filter === status)
    .map((status) => ({ status, requests: searched.filter((request) => statusOf(request) === status) }))
    .filter((group) => group.requests.length > 0)
  // As with completed work, resolved requests start collapsed unless they are all there is.
  const collapseResolved = groups.length > 1 && !query

  return (
    <div className="@container">
      <div className="flex flex-wrap items-center gap-x-3 gap-y-3 border-b border-quiet-divider-strong py-4">
        <div className="flex flex-wrap items-center gap-1" role="group" aria-label="Filter requests by status">
          {filters.map((item) => {
            const total = count(item.value)
            return (
              <button
                key={item.value}
                type="button"
                aria-pressed={filter === item.value}
                onClick={() => setFilter(item.value)}
                className={cn(
                  'rounded-md px-2.5 py-1.5 text-xs transition-colors hover:bg-quiet-hover focus-visible:outline-2 focus-visible:outline-ring',
                  filter === item.value
                    ? 'bg-quiet-icon-well font-medium text-quiet-text-primary'
                    : item.value === 'waiting_on_customer' && total > 0 ? 'text-quiet-accent' : 'text-quiet-text-secondary',
                )}
              >
                {item.label} <span className="ml-1 tabular-nums">({total})</span>
              </button>
            )
          })}
        </div>
        <div className="ml-auto flex min-w-0 items-center gap-2">
          <QuietSearchInput aria-label="Search requests" placeholder="Search requests…" value={search} onChange={(event) => setSearch(event.target.value)} containerClassName="w-44 max-w-full" />
          <QuietIconAction aria-label="Refresh requests" title="Refresh requests" onClick={onRefresh}>
            <ArrowReloadHorizontalIcon className="size-[15px]" />
          </QuietIconAction>
        </div>
      </div>

      {requests.length === 0 ? (
        copied ? (
          <QuietEmptyState title="Nothing copied to you yet" description="Requests other people copy you on will appear here with their latest status." />
        ) : companyName ? (
          <QuietEmptyState title="No company requests yet" description={`Requests anyone at ${companyName} sends to support will appear here with their latest status.`} />
        ) : (
          <QuietEmptyState title="No requests yet" description="Requests you send to support will appear here with their latest status." />
        )
      ) : groups.length === 0 ? (
        <QuietEmptyState
          title="No matching requests"
          description={query.length >= 2 ? 'Search looks through every message in these requests.' : null}
          action={<QuietTextAction onClick={() => { setFilter('all'); setSearch('') }}>Clear filters</QuietTextAction>}
        />
      ) : groups.map((group) => (
        <RequestGroup
          key={`${group.status}:${query}`}
          label={requestStatus[group.status].group}
          requests={group.requests}
          initiallyCollapsed={group.status === 'resolved' && collapseResolved}
          showAll={Boolean(query)}
          slug={slug}
          supportName={supportName}
          now={now}
          kind={kind}
          snippets={matches}
        />
      ))}
    </div>
  )
}

function RequestGroup({ label, requests, initiallyCollapsed, showAll: searching, slug, supportName, now, kind, snippets }: {
  label: string
  requests: CustomerPortalRequest[]
  initiallyCollapsed: boolean
  showAll: boolean
  slug: string
  supportName: string
  now: number
  kind: RequestListKind
  snippets: Map<string, string> | null
}) {
  const [collapsed, setCollapsed] = useState(initiallyCollapsed)
  const [showAll, setShowAll] = useState(false)
  const visible = searching || showAll ? requests : requests.slice(0, PAGE_SIZE)
  return (
    <section aria-label={label} className="border-b border-quiet-divider-strong">
      <div className="flex min-h-12 items-center">
        <button
          type="button"
          aria-expanded={!collapsed}
          onClick={() => setCollapsed((value) => !value)}
          className="flex items-center gap-2 rounded-sm py-2 text-xs font-semibold text-quiet-text-primary focus-visible:outline-2 focus-visible:outline-ring"
        >
          <ArrowRight01Icon className={cn('size-3 text-quiet-text-tertiary', !collapsed && 'rotate-90')} />
          {label} <span className="font-normal tabular-nums text-quiet-text-tertiary">({requests.length})</span>
        </button>
      </div>
      {collapsed ? null : (
        <div className="pb-3">
          <ul className="divide-y divide-quiet-divider-light" aria-label={`${label} requests`}>
            {visible.map((request) => <RequestRow key={request.reference} request={request} slug={slug} supportName={supportName} now={now} kind={kind} snippet={snippets?.get(request.reference)} />)}
          </ul>
          {!searching && requests.length > PAGE_SIZE ? (
            <QuietTextAction onClick={() => setShowAll((value) => !value)} className="mt-3">
              {showAll ? 'Show less' : `Show ${requests.length - PAGE_SIZE} more`}
            </QuietTextAction>
          ) : null}
        </div>
      )}
    </section>
  )
}

function RequestRow({ request, slug, supportName, now, kind, snippet }: { request: CustomerPortalRequest; slug: string; supportName: string; now: number; kind: RequestListKind; snippet?: string }) {
  const status = statusOf(request)
  const who = repliedBy(request, supportName, kind)
  const when = formatPortalRelativeTime(request.last_activity_at, now)
  const absolute = request.last_activity_at ? formatPortalAbsoluteTime(request.last_activity_at) : undefined
  const replyBy = formatPortalReplyBy(request.reply_expected_by)
  const params = { slug, reference: request.reference }
  return (
    <li className="group relative grid grid-cols-[minmax(0,1fr)] items-center gap-x-4 px-2 py-3 hover:bg-quiet-row-hover @min-[720px]:grid-cols-[minmax(220px,1fr)_120px_120px_64px]">
      <div className="grid min-w-0 grid-cols-[3.5rem_minmax(0,1fr)] gap-x-3">
        <Link
          to="/portal/$slug/requests/$reference"
          params={params}
          aria-label={request.unread ? `${request.subject}, new reply` : request.subject}
          className="col-span-2 grid grid-cols-subgrid items-baseline rounded-sm text-left focus-visible:outline-2 focus-visible:outline-ring"
        >
          <span className="min-w-0 truncate font-mono text-[10.5px] tabular-nums text-quiet-muted">{requestNumber(request)}</span>
          <span className="flex min-w-0 items-center gap-2">
            {request.unread ? <span className="size-2 shrink-0 rounded-full bg-quiet-accent" aria-hidden="true" /> : null}
            <span className={cn('min-w-0 truncate text-[13.5px] leading-5', request.unread ? 'font-semibold' : 'font-medium')} title={request.subject}>{request.subject}</span>
          </span>
        </Link>
        <div className="col-start-2 mt-1 flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1 text-[11.5px] text-quiet-text-tertiary">
          {request.unread ? <span className="shrink-0 font-semibold uppercase tracking-[0.03em] text-quiet-accent">New reply</span> : null}
          {kind !== 'own' ? <span className="shrink-0 font-medium text-quiet-text-secondary">Asked by {requestedBy(request)}</span> : null}
          {who ? <span className="shrink-0">{who}</span> : null}
          {replyBy ? <span className="shrink-0 font-medium text-quiet-text-secondary">Reply by {replyBy}</span> : null}
          {snippet ? (
            <span className="min-w-0 flex-1 basis-40 truncate italic" title={snippet}>{snippet}</span>
          ) : request.last_message_preview ? <span className="min-w-0 flex-1 basis-40 truncate">{request.last_message_preview}</span> : null}
          <span className="flex w-full items-center gap-2 @min-[720px]:hidden">
            <RequestStatusText status={status} />
            {when ? <time dateTime={request.last_activity_at ?? undefined} title={absolute}>· {when}</time> : null}
          </span>
        </div>
      </div>
      <span className="hidden @min-[720px]:block"><RequestStatusText status={status} /></span>
      <span className="hidden text-xs text-quiet-text-tertiary @min-[720px]:block">
        {when ? <time dateTime={request.last_activity_at ?? undefined} title={absolute}>{when}</time> : '—'}
      </span>
      {/* A mouse shortcut: the title link above is the accessible way in. */}
      <Button asChild variant="secondary" size="xs" className="hidden justify-self-center @min-[720px]:inline-flex">
        <Link to="/portal/$slug/requests/$reference" params={params} tabIndex={-1} aria-hidden="true">
          {status === 'waiting_on_customer' ? 'Reply' : 'Open'}
        </Link>
      </Button>
    </li>
  )
}
