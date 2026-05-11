import { useMemo, useState, type FormEvent, type ReactNode } from 'react'
import { Activity, AlertTriangle, CheckCircle2, Clock, Mail, RefreshCw, Search, ShieldCheck, XCircle } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { useEmailConversationDiagnostics, useEmailDiagnostics } from '@/hooks/queries/useEmailQueue'
import type {
  EmailFallbackConversationDiagnosticsResponse,
  EmailFallbackMessageDiagnostics,
  EmailQueueEntry,
  EmailLogCount,
  SupportEmailLog,
  WebhookEvent,
} from '@/lib/pmTypes'

function formatTime(iso?: string) {
  if (!iso) return '—'
  return new Date(iso).toLocaleString('en-US', {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  })
}

function truncate(value: string | undefined, max: number) {
  if (!value) return '—'
  return value.length > max ? value.slice(0, max) + '...' : value
}

function formatSeconds(seconds: number) {
  if (seconds <= 0) return 'now'
  if (seconds < 60) return `${seconds}s`
  const minutes = Math.floor(seconds / 60)
  const remaining = seconds % 60
  return remaining > 0 ? `${minutes}m ${remaining}s` : `${minutes}m`
}

function logStatusVariant(status: string): 'default' | 'secondary' | 'destructive' | 'outline' {
  switch (status) {
    case 'sent':
    case 'delivered':
    case 'open':
    case 'opened':
      return 'secondary'
    case 'bounced':
    case 'failed':
    case 'spam_complaint':
      return 'destructive'
    default:
      return 'outline'
  }
}

function webhookVariant(type: string): 'default' | 'secondary' | 'destructive' | 'outline' {
  switch (type) {
    case 'delivery':
    case 'open':
      return 'secondary'
    case 'bounce':
    case 'spam_complaint':
      return 'destructive'
    case 'inbound':
      return 'default'
    default:
      return 'outline'
  }
}

function messageDecision(message: EmailFallbackMessageDiagnostics) {
  if (message.email_notified_at) return { label: 'sent', variant: 'secondary' as const }
  if (message.queued) return { label: 'queued', variant: 'default' as const }
  if (message.eligible) return { label: message.due ? 'ready' : 'waiting', variant: 'default' as const }
  return { label: 'blocked', variant: 'outline' as const }
}

function StatusValue({ ok }: { ok: boolean }) {
  return (
    <span className={ok ? 'inline-flex items-center gap-1.5 text-emerald-700 dark:text-emerald-400' : 'inline-flex items-center gap-1.5 text-destructive'}>
      {ok ? <CheckCircle2 className="h-3.5 w-3.5" /> : <XCircle className="h-3.5 w-3.5" />}
      {ok ? 'Configured' : 'Missing'}
    </span>
  )
}

function Section({
  title,
  action,
  children,
  className = '',
}: {
  title: string
  action?: ReactNode
  children: ReactNode
  className?: string
}) {
  return (
    <section className={`rounded-md border bg-card ${className}`}>
      <div className="flex min-h-11 items-center justify-between border-b px-4 py-2">
        <div className="text-sm font-medium">{title}</div>
        {action}
      </div>
      {children}
    </section>
  )
}

function EmptyState({ children }: { children: ReactNode }) {
  return <div className="px-4 py-8 text-center text-sm text-muted-foreground">{children}</div>
}

function SummaryItem({
  icon,
  label,
  value,
  detail,
}: {
  icon: ReactNode
  label: string
  value: ReactNode
  detail: string
}) {
  return (
    <div className="rounded-md border bg-background px-4 py-3">
      <div className="flex items-center justify-between gap-3">
        <div className="text-sm text-muted-foreground">{label}</div>
        <div className="text-muted-foreground">{icon}</div>
      </div>
      <div className="mt-2 text-lg font-semibold">{value}</div>
      <div className="mt-1 text-xs text-muted-foreground">{detail}</div>
    </div>
  )
}

function ConfigItem({
  label,
  status,
  detail,
}: {
  label: string
  status: ReactNode
  detail: ReactNode
}) {
  return (
    <div className="border-b px-4 py-3 last:border-b-0 md:border-b-0 md:border-r md:last:border-r-0">
      <div className="text-sm font-medium">{label}</div>
      <div className="mt-1 text-sm">{status}</div>
      <div className="mt-2 break-all font-mono text-xs text-muted-foreground">{detail}</div>
    </div>
  )
}

export function EmailDiagnosticsPage() {
  const { data, isLoading, refetch, isFetching } = useEmailDiagnostics()
  const [conversationInput, setConversationInput] = useState('')
  const [conversationId, setConversationId] = useState('')
  const conversationDiagnostics = useEmailConversationDiagnostics(conversationId)

  const handleConversationLookup = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    setConversationId(conversationInput.trim())
  }

  if (isLoading) {
    return (
      <div className="mx-auto max-w-7xl py-12 text-center text-sm text-muted-foreground">
        Loading email diagnostics...
      </div>
    )
  }

  if (!data) {
    return (
      <div className="mx-auto max-w-7xl rounded-md border border-destructive/30 p-4 text-sm text-destructive">
        Email diagnostics could not be loaded.
      </div>
    )
  }

  const config = data.config
  const queueTotal = data.queue?.total ?? 0
  const sentCount = data.log_counts
    .filter((count) => count.direction === 'outbound' && ['sent', 'delivered', 'open', 'opened'].includes(count.status))
    .reduce((sum, count) => sum + count.count, 0)
  const failedCount = data.log_counts
    .filter((count) => ['failed', 'bounced', 'spam_complaint'].includes(count.status))
    .reduce((sum, count) => sum + count.count, 0)
  const configReady =
    config.reply_email_configured &&
    config.redis_configured &&
    config.fallback_poller_enabled &&
    config.reply_inbound_secret_set

  return (
    <div className="mx-auto max-w-7xl space-y-5">
      <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
        <div>
          <h2 className="text-base font-semibold">Email Diagnostics</h2>
          <p className="mt-1 text-sm text-muted-foreground">
            Support fallback delivery, Postmark configuration, queue state, and conversation-level decisions.
          </p>
        </div>
        <Button variant="outline" size="sm" onClick={() => void refetch()} disabled={isFetching}>
          <RefreshCw className={isFetching ? 'h-4 w-4 animate-spin' : 'h-4 w-4'} />
          Refresh
        </Button>
      </div>

      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        <SummaryItem
          icon={<ShieldCheck className="h-4 w-4" />}
          label="Fallback readiness"
          value={configReady ? 'Ready' : 'Needs attention'}
          detail={configReady ? 'Queue, poller, and reply mail are configured' : 'Configuration is incomplete'}
        />
        <SummaryItem
          icon={<Clock className="h-4 w-4" />}
          label="Queue"
          value={`${queueTotal} pending`}
          detail="Delayed fallback emails waiting to fire"
        />
        <SummaryItem
          icon={<Mail className="h-4 w-4" />}
          label="Recent sends"
          value={sentCount.toLocaleString()}
          detail="Outbound logs with accepted or delivered states"
        />
        <SummaryItem
          icon={<Activity className="h-4 w-4" />}
          label="Provider issues"
          value={failedCount.toLocaleString()}
          detail="Failed, bounced, or complaint logs"
        />
      </div>

      {!config.fallback_poller_enabled && (
        <div className="flex items-start gap-2 rounded-md border border-amber-300 bg-amber-50 px-3 py-2 text-sm text-amber-900 dark:border-amber-900/60 dark:bg-amber-950/30 dark:text-amber-200">
          <AlertTriangle className="mt-0.5 h-4 w-4" />
          <span>Fallback emails will not leave the queue until Redis and the reply Postmark client are both configured.</span>
        </div>
      )}

      <Section title="Conversation lookup">
        <div className="space-y-4 p-4">
          <form className="flex flex-col gap-2 md:flex-row" onSubmit={handleConversationLookup}>
            <Input
              value={conversationInput}
              onChange={(event) => setConversationInput(event.target.value)}
              placeholder="Conversation ID"
              className="font-mono text-xs"
            />
            <Button type="submit" size="sm" disabled={!conversationInput.trim() || conversationDiagnostics.isFetching}>
              <Search className="h-4 w-4" />
              Inspect
            </Button>
          </form>
          {conversationId && conversationDiagnostics.isLoading && (
            <div className="rounded-md border bg-muted/20 px-3 py-3 text-sm text-muted-foreground">
              Loading conversation diagnostics...
            </div>
          )}
          {conversationDiagnostics.isError && (
            <div className="rounded-md border border-destructive/30 px-3 py-2 text-sm text-destructive">
              Conversation diagnostics could not be loaded.
            </div>
          )}
          {conversationDiagnostics.data && (
            <ConversationDiagnostics diagnostics={conversationDiagnostics.data} />
          )}
        </div>
      </Section>

      <Section title="Configuration">
        <div className="border-b bg-muted/20 px-4 py-3">
          <div className="flex items-start gap-2 text-sm">
            <ShieldCheck className="mt-0.5 h-4 w-4 text-muted-foreground" />
            <div>
              <div className="font-medium">Outbound sender verification</div>
              <div className="mt-1 text-muted-foreground">
                Helpin tries the workspace mailbox sender first. If Postmark rejects it because the domain is not verified, delivery retries with the verified fallback sender while keeping the conversation Reply-To.
              </div>
            </div>
          </div>
        </div>
        <div className="grid md:grid-cols-2 xl:grid-cols-4">
          <ConfigItem label="App Postmark" status={<StatusValue ok={config.app_email_configured} />} detail={config.app_from_email || '—'} />
          <ConfigItem label="Reply Postmark" status={<StatusValue ok={config.reply_email_configured} />} detail={config.reply_from_email || '—'} />
          <ConfigItem label="Route token" status={<StatusValue ok={config.route_email_configured} />} detail="inbound routes" />
          <ConfigItem label="Redis and poller" status={<StatusValue ok={config.redis_configured && config.fallback_poller_enabled} />} detail="fallback queue storage" />
        </div>
        <div className="grid border-t md:grid-cols-2">
          <ConfigItem label="Reply webhook" status={<StatusValue ok={config.reply_inbound_secret_set} />} detail={config.support_email_reply_domain} />
          <ConfigItem label="Route webhook" status={<StatusValue ok={config.route_inbound_secret_set} />} detail={config.support_email_route_domain} />
        </div>
        <div className="grid border-t md:grid-cols-2">
          <div className="px-4 py-3">
            <div className="text-sm font-medium">Preferred branded From</div>
            <div className="mt-2 break-all font-mono text-xs text-muted-foreground">
              {config.expected_branded_from_shape || config.expected_fallback_from_shape}
            </div>
          </div>
          <div className="border-t px-4 py-3 md:border-l md:border-t-0">
            <div className="text-sm font-medium">Expected Reply-To</div>
            <div className="mt-2 break-all font-mono text-xs text-muted-foreground">{config.expected_reply_to_shape}</div>
          </div>
        </div>
        <div className="grid border-t md:grid-cols-2">
          <div className="px-4 py-3">
            <div className="text-sm font-medium">Verified fallback From</div>
            <div className="mt-2 break-all font-mono text-xs text-muted-foreground">
              {config.verified_fallback_from_email || config.reply_from_email || '—'}
            </div>
          </div>
          <div className="border-t px-4 py-3 md:border-l md:border-t-0">
            <div className="text-sm font-medium">Fallback behavior</div>
            <div className="mt-2 text-xs text-muted-foreground">
              {config.outbound_from_behavior || 'Retry with the verified sender when Postmark rejects the branded sender.'}
            </div>
          </div>
        </div>
      </Section>

      <div className="grid gap-5 lg:grid-cols-[minmax(0,1fr)_360px]">
        <Section title="Recent email logs" action={<Badge variant="outline">{data.recent_logs.length}</Badge>}>
          <EmailLogTable logs={data.recent_logs} />
        </Section>

        <Section title="Counts">
          <CountsTable counts={data.log_counts} queueTotal={queueTotal} />
        </Section>
      </div>

      <Section title="Pending queue" action={<Badge variant="outline">{queueTotal} pending</Badge>}>
        {queueTotal === 0 ? (
          <EmptyState>No pending fallback emails.</EmptyState>
        ) : (
          <QueueTable entries={data.queue.entries} />
        )}
      </Section>

      <Section title="Recent webhooks" action={<Badge variant="outline">{data.recent_webhooks.length}</Badge>}>
        <WebhookTable events={data.recent_webhooks} />
      </Section>
    </div>
  )
}

function ConversationDiagnostics({ diagnostics }: { diagnostics: EmailFallbackConversationDiagnosticsResponse }) {
  const queue = diagnostics.queue
  const stats = useMemo(() => {
    return diagnostics.messages.reduce(
      (acc, message) => {
        if (message.email_notified_at) acc.sent += 1
        if (message.queued) acc.queued += 1
        if (message.eligible) acc.eligible += 1
        if (!message.eligible) acc.blocked += 1
        return acc
      },
      { sent: 0, queued: 0, eligible: 0, blocked: 0 },
    )
  }, [diagnostics.messages])

  return (
    <div className="space-y-4 rounded-md border bg-background">
      <div className="grid border-b md:grid-cols-[minmax(0,1fr)_260px]">
        <div className="space-y-3 p-4">
          <div>
            <div className="text-sm font-medium">{diagnostics.subject || 'Untitled conversation'}</div>
            <div className="mt-1 break-all font-mono text-xs text-muted-foreground">{diagnostics.conversation_id}</div>
          </div>
          <div className="flex flex-wrap gap-2">
            <Badge variant="outline">{diagnostics.status}</Badge>
            <Badge variant={diagnostics.visitor_online ? 'secondary' : 'outline'}>
              {diagnostics.visitor_online ? 'visitor online' : 'visitor offline'}
            </Badge>
            {diagnostics.email_unsubscribed && <Badge variant="destructive">unsubscribed</Badge>}
            {queue.queued && <Badge variant="default">queued</Badge>}
          </div>
        </div>
        <div className="border-t p-4 md:border-l md:border-t-0">
          <div className="grid grid-cols-2 gap-3 text-sm">
            <div>
              <div className="text-xs text-muted-foreground">Eligible</div>
              <div className="mt-1 font-semibold">{stats.eligible}</div>
            </div>
            <div>
              <div className="text-xs text-muted-foreground">Blocked</div>
              <div className="mt-1 font-semibold">{stats.blocked}</div>
            </div>
            <div>
              <div className="text-xs text-muted-foreground">Queued</div>
              <div className="mt-1 font-semibold">{stats.queued}</div>
            </div>
            <div>
              <div className="text-xs text-muted-foreground">Sent</div>
              <div className="mt-1 font-semibold">{stats.sent}</div>
            </div>
          </div>
        </div>
      </div>

      <div className="grid gap-0 border-b text-sm md:grid-cols-4">
        <DiagnosticField label="Customer" value={diagnostics.customer_email || '—'} mono />
        <DiagnosticField
          label="Queue"
          value={queue.queued ? `Fires ${formatTime(queue.fire_at)}` : 'Not queued'}
        />
        <DiagnosticField label="Delay" value={formatSeconds(diagnostics.settings.email_fallback_delay_secs)} />
        <DiagnosticField
          label="Send window"
          value={`${Math.round(diagnostics.settings.email_fallback_max_delivery_age_secs / 60)} minutes`}
        />
      </div>

      {queue.redis_error && (
        <div className="mx-4 rounded-md border border-amber-300 bg-amber-50 px-3 py-2 text-sm text-amber-900 dark:border-amber-900/60 dark:bg-amber-950/30 dark:text-amber-200">
          Redis queue check failed: {queue.redis_error}
        </div>
      )}

      <div className="pb-4">
        <MessageDiagnosticsTable messages={diagnostics.messages} />
      </div>
    </div>
  )
}

function DiagnosticField({ label, value, mono = false }: { label: string; value: ReactNode; mono?: boolean }) {
  return (
    <div className="border-b px-4 py-3 last:border-b-0 md:border-b-0 md:border-r md:last:border-r-0">
      <div className="text-xs text-muted-foreground">{label}</div>
      <div className={mono ? 'mt-1 break-all font-mono text-xs' : 'mt-1'}>{value}</div>
    </div>
  )
}

function MessageDiagnosticsTable({ messages }: { messages: EmailFallbackMessageDiagnostics[] }) {
  if (messages.length === 0) {
    return <EmptyState>No messages found.</EmptyState>
  }

  return (
    <div className="overflow-x-auto">
      <table className="w-full min-w-[1040px] text-sm">
        <thead>
          <tr className="border-b bg-muted/40 text-left text-xs font-medium text-muted-foreground">
            <th className="px-4 py-2">Created</th>
            <th className="px-4 py-2">Sender</th>
            <th className="px-4 py-2">Message</th>
            <th className="px-4 py-2">Delivery</th>
            <th className="px-4 py-2">Decision</th>
            <th className="px-4 py-2">Reason</th>
          </tr>
        </thead>
        <tbody>
          {messages.map((message) => {
            const decision = messageDecision(message)
            const primaryReason = message.reasons[0] ?? 'No blocking reason recorded'
            return (
              <tr key={message.id} className="border-b align-top last:border-0 hover:bg-muted/20">
                <td className="whitespace-nowrap px-4 py-3 text-xs text-muted-foreground">{formatTime(message.created_at)}</td>
                <td className="px-4 py-3">
                  <div>{message.sender_type}</div>
                  <div className="text-xs text-muted-foreground">{message.message_type}</div>
                </td>
                <td className="max-w-sm px-4 py-3">
                  <div>{message.content_preview || '—'}</div>
                  <div className="mt-1 font-mono text-[11px] text-muted-foreground">{message.id}</div>
                </td>
                <td className="px-4 py-3">
                  <div className="flex flex-wrap gap-1">
                    {message.email_notified_at && <Badge variant="secondary">sent</Badge>}
                    {message.email_log_status && <Badge variant={logStatusVariant(message.email_log_status)}>{message.email_log_status}</Badge>}
                    {message.email_read_at && <Badge variant="outline">read</Badge>}
                    {message.queued && <Badge variant="default">queued</Badge>}
                    {!message.queued && !message.email_notified_at && !message.email_log_status && <span className="text-muted-foreground">—</span>}
                  </div>
                </td>
                <td className="px-4 py-3">
                  <div className="flex flex-wrap gap-1">
                    <Badge variant={decision.variant}>{decision.label}</Badge>
                    {message.reconcile_candidate && <Badge variant="secondary">reconcile</Badge>}
                    {!message.due && <Badge variant="outline">waiting</Badge>}
                  </div>
                  {message.cancellable_until && (
                    <div className="mt-2 text-xs text-muted-foreground">Cancelable until {formatTime(message.cancellable_until)}</div>
                  )}
                </td>
                <td className="px-4 py-3">
                  <div className="text-sm">{primaryReason}</div>
                  {message.reasons.length > 1 && (
                    <div className="mt-1 space-y-1">
                      {message.reasons.slice(1).map((reason) => (
                        <div key={reason} className="text-xs text-muted-foreground">{reason}</div>
                      ))}
                    </div>
                  )}
                </td>
              </tr>
            )
          })}
        </tbody>
      </table>
    </div>
  )
}

function CountsTable({ counts, queueTotal }: { counts: EmailLogCount[]; queueTotal: number }) {
  return (
    <table className="w-full text-sm">
      <tbody>
        <tr className="border-b">
          <td className="px-4 py-3 text-muted-foreground">Queued</td>
          <td className="px-4 py-3 text-right font-mono">{queueTotal}</td>
        </tr>
        {counts.map((count) => (
          <tr key={`${count.direction}-${count.status}`} className="border-b last:border-0">
            <td className="px-4 py-3 text-muted-foreground">
              {count.direction} / {count.status}
            </td>
            <td className="px-4 py-3 text-right font-mono">{count.count}</td>
          </tr>
        ))}
      </tbody>
    </table>
  )
}

function EmailLogTable({ logs }: { logs: SupportEmailLog[] }) {
  if (logs.length === 0) {
    return <EmptyState>No email logs found.</EmptyState>
  }

  return (
    <div className="overflow-x-auto">
      <table className="w-full min-w-[860px] text-sm">
        <thead>
          <tr className="border-b bg-muted/40 text-left text-xs font-medium text-muted-foreground">
            <th className="px-4 py-2">Created</th>
            <th className="px-4 py-2">Direction</th>
            <th className="px-4 py-2">Status</th>
            <th className="px-4 py-2">From</th>
            <th className="px-4 py-2">To</th>
            <th className="px-4 py-2">Subject</th>
            <th className="px-4 py-2">Postmark ID</th>
          </tr>
        </thead>
        <tbody>
          {logs.map((log) => (
            <tr key={log.id} className="border-b last:border-0 hover:bg-muted/20">
              <td className="whitespace-nowrap px-4 py-3 text-xs text-muted-foreground">{formatTime(log.created_at)}</td>
              <td className="px-4 py-3">{log.direction}</td>
              <td className="px-4 py-3">
                <Badge variant={logStatusVariant(log.status)}>{log.status}</Badge>
              </td>
              <td className="px-4 py-3 font-mono text-xs">{truncate(log.from_email, 34)}</td>
              <td className="px-4 py-3 font-mono text-xs">{truncate(log.to_email, 34)}</td>
              <td className="px-4 py-3">{truncate(log.subject, 44)}</td>
              <td className="px-4 py-3 font-mono text-xs">{truncate(log.postmark_message_id, 18)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

function QueueTable({ entries }: { entries: EmailQueueEntry[] }) {
  if (entries.length === 0) {
    return <EmptyState>No pending fallback emails.</EmptyState>
  }

  return (
    <div className="overflow-x-auto">
      <table className="w-full min-w-[760px] text-sm">
        <thead>
          <tr className="border-b bg-muted/40 text-left text-xs font-medium text-muted-foreground">
            <th className="px-4 py-2">Customer</th>
            <th className="px-4 py-2">Subject</th>
            <th className="px-4 py-2">Messages</th>
            <th className="px-4 py-2">Fires in</th>
            <th className="px-4 py-2">Fire at</th>
          </tr>
        </thead>
        <tbody>
          {entries.map((entry) => (
            <tr key={entry.conversation_id} className="border-b last:border-0 hover:bg-muted/20">
              <td className="px-4 py-3">
                <div>{entry.customer_name || '—'}</div>
                <div className="font-mono text-xs text-muted-foreground">{entry.customer_email || '—'}</div>
              </td>
              <td className="px-4 py-3">{truncate(entry.subject, 72)}</td>
              <td className="px-4 py-3 font-mono">{entry.message_count}</td>
              <td className="px-4 py-3">{formatSeconds(entry.delay_remaining_secs)}</td>
              <td className="whitespace-nowrap px-4 py-3 text-xs text-muted-foreground">{formatTime(entry.fire_at)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

function WebhookTable({ events }: { events: WebhookEvent[] }) {
  if (events.length === 0) {
    return <EmptyState>No webhook events found.</EmptyState>
  }

  return (
    <div className="overflow-x-auto">
      <table className="w-full min-w-[760px] text-sm">
        <thead>
          <tr className="border-b bg-muted/40 text-left text-xs font-medium text-muted-foreground">
            <th className="px-4 py-2">Created</th>
            <th className="px-4 py-2">Type</th>
            <th className="px-4 py-2">Stream</th>
            <th className="px-4 py-2">Message ID</th>
            <th className="px-4 py-2">Conversation</th>
          </tr>
        </thead>
        <tbody>
          {events.map((event) => (
            <tr key={event.id} className="border-b last:border-0 hover:bg-muted/20">
              <td className="whitespace-nowrap px-4 py-3 text-xs text-muted-foreground">{formatTime(event.created_at)}</td>
              <td className="px-4 py-3">
                <Badge variant={webhookVariant(event.event_type)}>{event.event_type}</Badge>
              </td>
              <td className="px-4 py-3">{event.message_stream || '—'}</td>
              <td className="px-4 py-3 font-mono text-xs">{truncate(event.postmark_message_id, 24)}</td>
              <td className="px-4 py-3 font-mono text-xs">{truncate(event.conversation_id, 18)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
