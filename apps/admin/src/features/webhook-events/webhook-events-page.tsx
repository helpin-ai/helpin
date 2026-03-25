import { useState } from 'react'
import { ChevronLeft, ChevronRight, Eye } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { useWebhookEvents } from '@/hooks/queries/useWebhookEvents'
import type { WebhookEvent } from '@/lib/pmTypes'

const EVENT_TYPES = ['', 'inbound', 'open', 'delivery', 'bounce', 'spam_complaint'] as const
const PER_PAGE = 25

function eventVariant(type: string): 'default' | 'secondary' | 'destructive' | 'outline' {
  switch (type) {
    case 'inbound':
      return 'default'
    case 'open':
      return 'secondary'
    case 'bounce':
    case 'spam_complaint':
      return 'destructive'
    default:
      return 'outline'
  }
}

function formatDate(iso: string) {
  return new Date(iso).toLocaleString('en-US', {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  })
}

function truncate(str: string, max: number) {
  return str.length > max ? str.slice(0, max) + '...' : str
}

export function WebhookEventsPage() {
  const [page, setPage] = useState(1)
  const [eventType, setEventType] = useState('all')
  const [expandedId, setExpandedId] = useState<string | null>(null)

  const activeFilter = eventType === 'all' ? undefined : eventType
  const { data, isLoading } = useWebhookEvents({
    page,
    per_page: PER_PAGE,
    event_type: activeFilter,
  })

  const events = data?.data ?? []
  const totalPages = data?.total_pages ?? 0
  const total = data?.total ?? 0

  const handleFilterChange = (type: string) => {
    setEventType(type)
    setPage(1)
  }

  return (
    <div className="mx-auto max-w-6xl space-y-4">
      {/* Filters */}
      <div className="flex items-end gap-4">
        <div className="space-y-1.5">
          <Label>Event Type</Label>
          <Select value={eventType} onValueChange={handleFilterChange}>
            <SelectTrigger className="w-[180px]">
              <SelectValue placeholder="All events" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">All events</SelectItem>
              {EVENT_TYPES.filter(Boolean).map((t) => (
                <SelectItem key={t} value={t}>
                  {t}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div className="flex-1" />
        <span className="text-sm text-muted-foreground">
          {total} event{total !== 1 ? 's' : ''}
        </span>
      </div>

      {/* Table */}
      <div className="rounded-md border">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b bg-muted/40 text-left text-xs font-medium text-muted-foreground">
              <th className="px-3 py-2">Type</th>
              <th className="px-3 py-2">Provider</th>
              <th className="px-3 py-2">Message ID</th>
              <th className="px-3 py-2">Stream</th>
              <th className="px-3 py-2">Workspace</th>
              <th className="px-3 py-2">Created</th>
              <th className="px-3 py-2 w-10" />
            </tr>
          </thead>
          <tbody>
            {isLoading ? (
              <tr>
                <td colSpan={7} className="px-3 py-8 text-center text-muted-foreground">
                  Loading...
                </td>
              </tr>
            ) : events.length === 0 ? (
              <tr>
                <td colSpan={7} className="px-3 py-8 text-center text-muted-foreground">
                  No webhook events found.
                </td>
              </tr>
            ) : (
              events.map((event) => (
                <EventRow
                  key={event.id}
                  event={event}
                  expanded={expandedId === event.id}
                  onToggle={() => setExpandedId(expandedId === event.id ? null : event.id)}
                />
              ))
            )}
          </tbody>
        </table>
      </div>

      {/* Pagination */}
      {totalPages > 1 && (
        <div className="flex items-center justify-between">
          <span className="text-sm text-muted-foreground">
            Page {page} of {totalPages}
          </span>
          <div className="flex gap-1">
            <Button
              variant="outline"
              size="sm"
              disabled={page <= 1}
              onClick={() => setPage((p) => p - 1)}
            >
              <ChevronLeft className="h-4 w-4" />
            </Button>
            <Button
              variant="outline"
              size="sm"
              disabled={page >= totalPages}
              onClick={() => setPage((p) => p + 1)}
            >
              <ChevronRight className="h-4 w-4" />
            </Button>
          </div>
        </div>
      )}
    </div>
  )
}

function EventRow({
  event,
  expanded,
  onToggle,
}: {
  event: WebhookEvent
  expanded: boolean
  onToggle: () => void
}) {
  return (
    <>
      <tr className="border-b last:border-0 hover:bg-muted/20">
        <td className="px-3 py-2">
          <Badge variant={eventVariant(event.event_type)} className="text-xs">
            {event.event_type}
          </Badge>
        </td>
        <td className="px-3 py-2 text-muted-foreground">{event.provider}</td>
        <td className="px-3 py-2 font-mono text-xs">
          {event.postmark_message_id ? truncate(event.postmark_message_id, 24) : '—'}
        </td>
        <td className="px-3 py-2 text-muted-foreground">{event.message_stream ?? '—'}</td>
        <td className="px-3 py-2 font-mono text-xs">
          {event.workspace_id ? truncate(event.workspace_id, 8) : '—'}
        </td>
        <td className="px-3 py-2 text-muted-foreground">{formatDate(event.created_at)}</td>
        <td className="px-3 py-2">
          <Button variant="ghost" size="icon" className="h-7 w-7" onClick={onToggle}>
            <Eye className="h-3.5 w-3.5" />
          </Button>
        </td>
      </tr>
      {expanded && (
        <tr className="border-b last:border-0">
          <td colSpan={7} className="bg-muted/10 px-3 py-3">
            <div className="space-y-2">
              <div className="flex flex-wrap gap-2 text-xs">
                <span className="text-muted-foreground">ID:</span>
                <span className="font-mono">{event.id}</span>
                {event.conversation_id && (
                  <>
                    <span className="text-muted-foreground ml-2">Conversation:</span>
                    <span className="font-mono">{event.conversation_id}</span>
                  </>
                )}
                {event.email_log_id && (
                  <>
                    <span className="text-muted-foreground ml-2">Email Log:</span>
                    <span className="font-mono">{event.email_log_id}</span>
                  </>
                )}
              </div>
              <details open>
                <summary className="cursor-pointer text-xs font-medium text-muted-foreground hover:text-foreground">
                  Raw Payload
                </summary>
                <pre className="mt-1 max-h-[300px] overflow-auto rounded-md border bg-background p-3 text-xs leading-5">
                  {formatPayload(event.raw_payload)}
                </pre>
              </details>
            </div>
          </td>
        </tr>
      )}
    </>
  )
}

function formatPayload(raw: string): string {
  try {
    return JSON.stringify(JSON.parse(raw), null, 2)
  } catch {
    return raw
  }
}
