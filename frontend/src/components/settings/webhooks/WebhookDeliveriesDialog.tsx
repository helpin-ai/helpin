import { useState } from 'react'
import { formatDistanceToNowStrict } from 'date-fns'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Skeleton } from '@/components/ui/skeleton'
import { useRedeliverWebhook, useWebhookDeliveries } from '@/hooks/queries/useSupportWebhooks'
import type { WebhookDelivery, WebhookEndpoint } from '@/lib/pmTypes'
import { webhookEventLabel } from '@/lib/supportWebhooks'
import { cn } from '@/lib/utils'

const STATUS_STYLES: Record<WebhookDelivery['status'], string> = {
  succeeded: 'bg-emerald-50 text-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-300',
  pending: 'bg-amber-50 text-amber-800 dark:bg-amber-950/40 dark:text-amber-200',
  failed: 'bg-red-50 text-red-700 dark:bg-red-950/40 dark:text-red-300',
}

function statusLabel(delivery: WebhookDelivery): string {
  if (delivery.status === 'succeeded') return 'Delivered'
  if (delivery.status === 'failed') return 'Failed'
  return delivery.attempts > 0 ? 'Retrying' : 'Sending'
}

/** The recent delivery log of one webhook, with payloads and resend. */
export function WebhookDeliveriesDialog({ workspaceId, webhook, editable, onClose }: { workspaceId: string; webhook: WebhookEndpoint | null; editable: boolean; onClose: () => void }) {
  const { data: deliveries, isLoading, error } = useWebhookDeliveries(workspaceId, webhook?.id ?? null)
  const redeliver = useRedeliverWebhook(workspaceId)
  const [expanded, setExpanded] = useState<string | null>(null)

  return (
    <Dialog open={Boolean(webhook)} onOpenChange={(open) => { if (!open) { setExpanded(null); onClose() } }}>
      <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-2xl [&>*]:min-w-0">
        <DialogHeader>
          <DialogTitle>Recent deliveries</DialogTitle>
          <DialogDescription className="break-all">{webhook?.url}</DialogDescription>
        </DialogHeader>
        {isLoading ? <Skeleton className="h-32 w-full" /> : null}
        {error ? <p className="text-sm text-destructive">Could not load deliveries: {error instanceof Error ? error.message : 'unknown error'}</p> : null}
        {deliveries && deliveries.length === 0 ? (
          <p className="text-sm text-muted-foreground">No events sent yet. Use “Send test event” to check the endpoint.</p>
        ) : null}
        {deliveries && deliveries.length > 0 ? (
          <ul className="divide-y divide-border rounded-md border border-border" aria-label="Webhook deliveries">
            {deliveries.map((delivery) => {
              const open = expanded === delivery.id
              return (
                <li key={delivery.id} className="px-3 py-2" data-testid="webhook-delivery">
                  <div className="flex flex-wrap items-center gap-2 text-xs">
                    <span className={cn('rounded-full px-2 py-0.5 font-medium', STATUS_STYLES[delivery.status])}>{statusLabel(delivery)}</span>
                    <span className="font-medium text-foreground">{webhookEventLabel(delivery.event_type)}</span>
                    {delivery.last_status_code ? <span className="tabular-nums text-muted-foreground">HTTP {delivery.last_status_code}</span> : null}
                    {delivery.attempts > 1 ? <span className="text-muted-foreground">{delivery.attempts} attempts</span> : null}
                    <span className="ml-auto text-muted-foreground" title={new Date(delivery.created_at).toLocaleString()}>
                      {formatDistanceToNowStrict(new Date(delivery.created_at), { addSuffix: true })}
                    </span>
                  </div>
                  {delivery.last_error && delivery.status !== 'succeeded' ? (
                    <p className="mt-1 break-words text-xs text-muted-foreground">{delivery.last_error}</p>
                  ) : null}
                  {delivery.status === 'pending' && delivery.attempts > 0 ? (
                    <p className="mt-1 text-xs text-muted-foreground">Next attempt {formatDistanceToNowStrict(new Date(delivery.next_attempt_at), { addSuffix: true })}.</p>
                  ) : null}
                  <div className="mt-1 flex gap-1">
                    <Button type="button" variant="ghost" size="sm" className="h-6 px-2 text-xs" aria-expanded={open} onClick={() => setExpanded(open ? null : delivery.id)}>
                      {open ? 'Hide payload' : 'Show payload'}
                    </Button>
                    {editable && delivery.event_type !== 'ping' && delivery.status !== 'pending' && webhook?.enabled ? (
                      <Button type="button" variant="ghost" size="sm" className="h-6 px-2 text-xs" disabled={redeliver.isPending}
                        onClick={() => redeliver.mutate({ webhookId: delivery.endpoint_id, deliveryId: delivery.id })}>
                        Send again
                      </Button>
                    ) : null}
                  </div>
                  {open ? (
                    <pre className="mt-1 max-h-64 overflow-auto rounded bg-muted px-2 py-1.5 text-[11px] leading-relaxed">{JSON.stringify(delivery.payload, null, 2)}</pre>
                  ) : null}
                </li>
              )
            })}
          </ul>
        ) : null}
      </DialogContent>
    </Dialog>
  )
}
