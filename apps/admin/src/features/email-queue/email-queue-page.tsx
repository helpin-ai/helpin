import { useState } from 'react'
import { Clock, Eye, Inbox } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { useEmailQueue } from '@/hooks/queries/useEmailQueue'
import type { EmailQueueEntry } from '@/lib/pmTypes'

function formatCountdown(secs: number) {
  if (secs <= 0) return 'Firing now'
  const m = Math.floor(secs / 60)
  const s = secs % 60
  if (m > 0) return `${m}m ${s}s`
  return `${s}s`
}

function formatTime(iso: string) {
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

export function EmailQueuePage() {
  const { data, isLoading } = useEmailQueue()
  const [expandedId, setExpandedId] = useState<string | null>(null)

  const entries = data?.entries ?? []
  const total = data?.total ?? 0

  return (
    <div className="mx-auto max-w-6xl space-y-4">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <h2 className="text-sm font-medium">Email Delivery Queue</h2>
          <Badge variant="outline" className="text-xs">
            {total} pending
          </Badge>
        </div>
        <span className="text-xs text-muted-foreground">Auto-refreshes every 10s</span>
      </div>

      {isLoading ? (
        <div className="flex items-center justify-center py-12 text-muted-foreground">
          Loading queue...
        </div>
      ) : entries.length === 0 ? (
        <div className="flex flex-col items-center justify-center gap-2 rounded-md border border-dashed py-12 text-muted-foreground">
          <Inbox className="h-8 w-8 opacity-40" />
          <p className="text-sm">No emails pending delivery</p>
          <p className="text-xs">
            When a visitor goes offline and an agent replies, emails queue here before sending.
          </p>
        </div>
      ) : (
        <div className="rounded-md border">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b bg-muted/40 text-left text-xs font-medium text-muted-foreground">
                <th className="px-3 py-2">Customer</th>
                <th className="px-3 py-2">Subject</th>
                <th className="px-3 py-2">Status</th>
                <th className="px-3 py-2">Messages</th>
                <th className="px-3 py-2">Fires In</th>
                <th className="px-3 py-2">Fire At</th>
                <th className="w-10 px-3 py-2" />
              </tr>
            </thead>
            <tbody>
              {entries.map((entry) => (
                <QueueRow
                  key={entry.conversation_id}
                  entry={entry}
                  expanded={expandedId === entry.conversation_id}
                  onToggle={() =>
                    setExpandedId(
                      expandedId === entry.conversation_id ? null : entry.conversation_id,
                    )
                  }
                />
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}

function QueueRow({
  entry,
  expanded,
  onToggle,
}: {
  entry: EmailQueueEntry
  expanded: boolean
  onToggle: () => void
}) {
  const isOverdue = entry.delay_remaining_secs <= 0

  return (
    <>
      <tr className="border-b last:border-0 hover:bg-muted/20">
        <td className="px-3 py-2">
          <div>
            <span className="font-medium">{entry.customer_name || '—'}</span>
            <p className="text-xs text-muted-foreground">{entry.customer_email || '—'}</p>
          </div>
        </td>
        <td className="px-3 py-2 text-muted-foreground">{truncate(entry.subject, 40)}</td>
        <td className="px-3 py-2">
          <Badge variant="outline" className="text-xs">
            {entry.status}
          </Badge>
        </td>
        <td className="px-3 py-2">{entry.message_count}</td>
        <td className="px-3 py-2">
          <span className={isOverdue ? 'font-medium text-orange-500' : ''}>
            <Clock className="mr-1 inline h-3 w-3" />
            {formatCountdown(entry.delay_remaining_secs)}
          </span>
        </td>
        <td className="px-3 py-2 text-xs text-muted-foreground">{formatTime(entry.fire_at)}</td>
        <td className="px-3 py-2">
          <Button variant="ghost" size="icon" className="h-7 w-7" onClick={onToggle}>
            <Eye className="h-3.5 w-3.5" />
          </Button>
        </td>
      </tr>
      {expanded && (
        <tr className="border-b last:border-0">
          <td colSpan={7} className="bg-muted/10 px-3 py-3">
            <div className="space-y-3">
              <div className="flex flex-wrap gap-3 text-xs">
                <span>
                  <span className="text-muted-foreground">Conversation:</span>{' '}
                  <span className="font-mono">{entry.conversation_id}</span>
                </span>
                <span>
                  <span className="text-muted-foreground">Workspace:</span>{' '}
                  <span className="font-mono">{truncate(entry.workspace_id, 8)}</span>
                </span>
              </div>

              {entry.messages.length > 0 && (
                <div className="space-y-2">
                  <p className="text-xs font-medium text-muted-foreground">
                    Queued Messages ({entry.messages.length})
                  </p>
                  {entry.messages.map((msg) => (
                    <div key={msg.id} className="rounded-md border bg-background p-2 text-sm">
                      <div className="mb-1 flex items-center gap-2 text-xs text-muted-foreground">
                        <span className="font-medium text-foreground">
                          {msg.sender_display_name || 'Agent'}
                        </span>
                        <span>{formatTime(msg.created_at)}</span>
                      </div>
                      <p>{msg.content}</p>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </td>
        </tr>
      )}
    </>
  )
}
