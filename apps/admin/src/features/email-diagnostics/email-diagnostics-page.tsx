import { AlertTriangle, CheckCircle2, XCircle } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { useEmailDiagnostics } from '@/hooks/queries/useEmailQueue'
import type { EmailLogCount, SupportEmailLog, WebhookEvent } from '@/lib/pmTypes'

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

function StatusValue({ ok }: { ok: boolean }) {
  return (
    <span className={ok ? 'inline-flex items-center gap-1 text-emerald-600' : 'inline-flex items-center gap-1 text-destructive'}>
      {ok ? <CheckCircle2 className="h-3.5 w-3.5" /> : <XCircle className="h-3.5 w-3.5" />}
      {ok ? 'Configured' : 'Missing'}
    </span>
  )
}

export function EmailDiagnosticsPage() {
  const { data, isLoading, refetch, isFetching } = useEmailDiagnostics()

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
  const configRows = [
    ['App Postmark client', <StatusValue ok={config.app_email_configured} />, config.app_from_email || '—'],
    ['Reply Postmark client', <StatusValue ok={config.reply_email_configured} />, config.reply_from_email || '—'],
    ['Route Postmark token', <StatusValue ok={config.route_email_configured} />, 'inbound routes'],
    ['Redis', <StatusValue ok={config.redis_configured} />, 'fallback queue storage'],
    ['Fallback poller', <StatusValue ok={config.fallback_poller_enabled} />, 'requires Redis and reply Postmark'],
    ['Reply webhook secret', <StatusValue ok={config.reply_inbound_secret_set} />, config.support_email_reply_domain],
    ['Route webhook secret', <StatusValue ok={config.route_inbound_secret_set} />, config.support_email_route_domain],
  ] as const

  return (
    <div className="mx-auto max-w-7xl space-y-5">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-sm font-semibold">Email Diagnostics</h2>
          <p className="mt-1 text-sm text-muted-foreground">
            Support reply configuration, queue state, recent sends, and provider webhooks.
          </p>
        </div>
        <Button variant="outline" size="sm" onClick={() => void refetch()} disabled={isFetching}>
          Refresh
        </Button>
      </div>

      {!config.fallback_poller_enabled && (
        <div className="flex items-start gap-2 rounded-md border border-amber-300 bg-amber-50 px-3 py-2 text-sm text-amber-900 dark:border-amber-900/60 dark:bg-amber-950/30 dark:text-amber-200">
          <AlertTriangle className="mt-0.5 h-4 w-4" />
          <span>Offline fallback emails will not leave the queue until Redis and the reply Postmark client are both configured.</span>
        </div>
      )}

      <section className="rounded-md border">
        <div className="border-b px-3 py-2 text-sm font-medium">Configuration</div>
        <table className="w-full text-sm">
          <tbody>
            {configRows.map(([label, status, detail]) => (
              <tr key={label} className="border-b last:border-0">
                <td className="w-56 px-3 py-2 text-muted-foreground">{label}</td>
                <td className="w-44 px-3 py-2">{status}</td>
                <td className="px-3 py-2 font-mono text-xs">{detail}</td>
              </tr>
            ))}
            <tr className="border-b">
              <td className="px-3 py-2 text-muted-foreground">Expected From</td>
              <td className="px-3 py-2" />
              <td className="px-3 py-2 font-mono text-xs">{config.expected_fallback_from_shape}</td>
            </tr>
            <tr>
              <td className="px-3 py-2 text-muted-foreground">Expected Reply-To</td>
              <td className="px-3 py-2" />
              <td className="px-3 py-2 font-mono text-xs">{config.expected_reply_to_shape}</td>
            </tr>
          </tbody>
        </table>
      </section>

      <div className="grid gap-5 lg:grid-cols-[minmax(0,1fr)_360px]">
        <section className="rounded-md border">
          <div className="flex items-center justify-between border-b px-3 py-2">
            <span className="text-sm font-medium">Recent email logs</span>
            <Badge variant="outline">{data.recent_logs.length}</Badge>
          </div>
          <EmailLogTable logs={data.recent_logs} />
        </section>

        <section className="rounded-md border">
          <div className="border-b px-3 py-2 text-sm font-medium">Counts</div>
          <CountsTable counts={data.log_counts} queueTotal={queueTotal} />
        </section>
      </div>

      <section className="rounded-md border">
        <div className="flex items-center justify-between border-b px-3 py-2">
          <span className="text-sm font-medium">Pending queue</span>
          <Badge variant="outline">{queueTotal} pending</Badge>
        </div>
        {queueTotal === 0 ? (
          <div className="px-3 py-8 text-center text-sm text-muted-foreground">No pending fallback emails.</div>
        ) : (
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b bg-muted/40 text-left text-xs font-medium text-muted-foreground">
                <th className="px-3 py-2">Customer</th>
                <th className="px-3 py-2">Subject</th>
                <th className="px-3 py-2">Messages</th>
                <th className="px-3 py-2">Fire at</th>
              </tr>
            </thead>
            <tbody>
              {data.queue.entries.map((entry) => (
                <tr key={entry.conversation_id} className="border-b last:border-0">
                  <td className="px-3 py-2">{entry.customer_email || '—'}</td>
                  <td className="px-3 py-2">{truncate(entry.subject, 72)}</td>
                  <td className="px-3 py-2">{entry.message_count}</td>
                  <td className="px-3 py-2 text-xs text-muted-foreground">{formatTime(entry.fire_at)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </section>

      <section className="rounded-md border">
        <div className="flex items-center justify-between border-b px-3 py-2">
          <span className="text-sm font-medium">Recent webhooks</span>
          <Badge variant="outline">{data.recent_webhooks.length}</Badge>
        </div>
        <WebhookTable events={data.recent_webhooks} />
      </section>
    </div>
  )
}

function CountsTable({ counts, queueTotal }: { counts: EmailLogCount[]; queueTotal: number }) {
  return (
    <table className="w-full text-sm">
      <tbody>
        <tr className="border-b">
          <td className="px-3 py-2 text-muted-foreground">Queued</td>
          <td className="px-3 py-2 text-right font-mono">{queueTotal}</td>
        </tr>
        {counts.map((count) => (
          <tr key={`${count.direction}-${count.status}`} className="border-b last:border-0">
            <td className="px-3 py-2 text-muted-foreground">
              {count.direction} / {count.status}
            </td>
            <td className="px-3 py-2 text-right font-mono">{count.count}</td>
          </tr>
        ))}
      </tbody>
    </table>
  )
}

function EmailLogTable({ logs }: { logs: SupportEmailLog[] }) {
  if (logs.length === 0) {
    return <div className="px-3 py-8 text-center text-sm text-muted-foreground">No email logs found.</div>
  }

  return (
    <table className="w-full text-sm">
      <thead>
        <tr className="border-b bg-muted/40 text-left text-xs font-medium text-muted-foreground">
          <th className="px-3 py-2">Created</th>
          <th className="px-3 py-2">Direction</th>
          <th className="px-3 py-2">Status</th>
          <th className="px-3 py-2">From</th>
          <th className="px-3 py-2">To</th>
          <th className="px-3 py-2">Subject</th>
          <th className="px-3 py-2">Postmark ID</th>
        </tr>
      </thead>
      <tbody>
        {logs.map((log) => (
          <tr key={log.id} className="border-b last:border-0">
            <td className="px-3 py-2 text-xs text-muted-foreground">{formatTime(log.created_at)}</td>
            <td className="px-3 py-2">{log.direction}</td>
            <td className="px-3 py-2">
              <Badge variant="outline">{log.status}</Badge>
            </td>
            <td className="px-3 py-2 font-mono text-xs">{truncate(log.from_email, 34)}</td>
            <td className="px-3 py-2 font-mono text-xs">{truncate(log.to_email, 34)}</td>
            <td className="px-3 py-2">{truncate(log.subject, 44)}</td>
            <td className="px-3 py-2 font-mono text-xs">{truncate(log.postmark_message_id, 18)}</td>
          </tr>
        ))}
      </tbody>
    </table>
  )
}

function WebhookTable({ events }: { events: WebhookEvent[] }) {
  if (events.length === 0) {
    return <div className="px-3 py-8 text-center text-sm text-muted-foreground">No webhook events found.</div>
  }

  return (
    <table className="w-full text-sm">
      <thead>
        <tr className="border-b bg-muted/40 text-left text-xs font-medium text-muted-foreground">
          <th className="px-3 py-2">Created</th>
          <th className="px-3 py-2">Type</th>
          <th className="px-3 py-2">Stream</th>
          <th className="px-3 py-2">Message ID</th>
          <th className="px-3 py-2">Conversation</th>
        </tr>
      </thead>
      <tbody>
        {events.map((event) => (
          <tr key={event.id} className="border-b last:border-0">
            <td className="px-3 py-2 text-xs text-muted-foreground">{formatTime(event.created_at)}</td>
            <td className="px-3 py-2">{event.event_type}</td>
            <td className="px-3 py-2">{event.message_stream || '—'}</td>
            <td className="px-3 py-2 font-mono text-xs">{truncate(event.postmark_message_id, 24)}</td>
            <td className="px-3 py-2 font-mono text-xs">{truncate(event.conversation_id, 18)}</td>
          </tr>
        ))}
      </tbody>
    </table>
  )
}
