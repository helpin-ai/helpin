import { useMemo, useState } from 'react'
import { formatDistanceToNowStrict } from 'date-fns'
import { toast } from 'sonner'
import { QuietEmptyState } from '@/components/design-system/quiet'
import { Button } from '@/components/ui/button'
import { useConfirm } from '@/components/ui/confirm-dialog'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { Skeleton } from '@/components/ui/skeleton'
import { Switch } from '@/components/ui/switch'
import { useInboxScopes } from '@/hooks/queries/useSupport'
import {
  useCreateWebhook,
  useDeleteWebhook,
  useRotateWebhookSecret,
  useSendWebhookTest,
  useSupportWebhooks,
  useUpdateWebhook,
} from '@/hooks/queries/useSupportWebhooks'
import { MoreHorizontalIcon, PlusSignIcon } from '@/lib/icons'
import type { WebhookEndpoint, WebhookEndpointInput } from '@/lib/pmTypes'
import { describeWebhookEvents } from '@/lib/supportWebhooks'
import { SupportWebhookDialog } from './SupportWebhookDialog'
import { WebhookDeliveriesDialog } from './WebhookDeliveriesDialog'
import { WebhookSecretDialog } from './WebhookSecretDialog'

function endpointInput(webhook: WebhookEndpoint, enabled: boolean): WebhookEndpointInput {
  return { url: webhook.url, description: webhook.description, events: webhook.events, mailbox_ids: webhook.mailbox_ids, enabled }
}

function health(webhook: WebhookEndpoint): { text: string; tone: 'muted' | 'warning' | 'error' } {
  if (!webhook.enabled && webhook.disabled_reason) return { text: webhook.disabled_reason, tone: 'error' }
  if (!webhook.enabled) return { text: 'Off', tone: 'muted' }
  if (webhook.failure_count > 0 && webhook.last_failure_at) {
    return { text: `Last ${webhook.failure_count === 1 ? 'delivery' : `${webhook.failure_count} deliveries`} failed`, tone: 'warning' }
  }
  if (webhook.last_success_at) return { text: `Delivered ${formatDistanceToNowStrict(new Date(webhook.last_success_at), { addSuffix: true })}`, tone: 'muted' }
  return { text: 'No events sent yet', tone: 'muted' }
}

const TONE_CLASS = { muted: 'text-muted-foreground', warning: 'text-amber-700 dark:text-amber-300', error: 'text-destructive' }

export function SupportWebhooks({ workspaceId, editable }: { workspaceId: string; editable: boolean }) {
  const { data: webhooks, isLoading, error } = useSupportWebhooks(workspaceId)
  const { data: scopes } = useInboxScopes(workspaceId)
  const createWebhook = useCreateWebhook(workspaceId)
  const updateWebhook = useUpdateWebhook(workspaceId)
  const deleteWebhook = useDeleteWebhook(workspaceId)
  const rotateSecret = useRotateWebhookSecret(workspaceId)
  const sendTest = useSendWebhookTest(workspaceId)
  const confirm = useConfirm()
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editing, setEditing] = useState<WebhookEndpoint | null>(null)
  const [secret, setSecret] = useState<string | null>(null)
  const [deliveriesFor, setDeliveriesFor] = useState<WebhookEndpoint | null>(null)

  const inboxes = useMemo(() => (scopes?.mailboxes ?? []).map((mailbox) => ({ id: mailbox.id, name: mailbox.name })), [scopes])
  const inboxNames = useMemo(() => new Map([['shared', 'Main inbox'], ...inboxes.map((inbox) => [inbox.id, inbox.name] as const)]), [inboxes])

  const openEditor = (webhook: WebhookEndpoint | null) => {
    setEditing(webhook)
    setDialogOpen(true)
  }

  const save = async (input: WebhookEndpointInput) => {
    try {
      if (editing) {
        await updateWebhook.mutateAsync({ id: editing.id, input })
      } else {
        const created = await createWebhook.mutateAsync(input)
        setSecret(created.secret)
      }
      return true
    } catch {
      return false
    }
  }

  const test = async (webhook: WebhookEndpoint) => {
    try {
      const delivery = await sendTest.mutateAsync(webhook.id)
      if (delivery.status === 'succeeded') toast.success('Test event delivered', { description: `The endpoint answered HTTP ${delivery.last_status_code}.` })
      else toast.error('Test event failed', { description: delivery.last_error || 'The endpoint did not accept the event.' })
    } catch {
      // The mutation shows the error.
    }
  }

  const rotate = async (webhook: WebhookEndpoint) => {
    const confirmed = await confirm({
      title: 'Create a new signing secret?',
      description: 'The current secret stops working right away. Update your endpoint with the new one.',
      confirmText: 'Create new secret',
    })
    if (!confirmed) return
    try {
      const rotated = await rotateSecret.mutateAsync(webhook.id)
      setSecret(rotated.secret)
    } catch {
      // The mutation shows the error.
    }
  }

  const remove = async (webhook: WebhookEndpoint) => {
    const confirmed = await confirm({
      title: 'Delete this webhook?',
      description: 'Helpin stops sending events to it, and its delivery log is deleted.',
      confirmText: 'Delete webhook',
      variant: 'destructive',
    })
    if (confirmed) deleteWebhook.mutate(webhook.id)
  }

  if (isLoading) return <Skeleton className="h-40 w-full" />
  if (error) {
    return <p className="text-sm text-destructive">Could not load webhooks: {error instanceof Error ? error.message : 'unknown error'}</p>
  }

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <p className="max-w-2xl text-sm text-muted-foreground">
          Each request is signed with the webhook's secret. Failed deliveries are retried for about a day; an endpoint that keeps failing is turned off.
        </p>
        {editable ? (
          <Button size="sm" onClick={() => openEditor(null)} disabled={(webhooks?.length ?? 0) >= 10}>
            <PlusSignIcon className="h-4 w-4" />
            Add webhook
          </Button>
        ) : null}
      </div>

      {!webhooks || webhooks.length === 0 ? (
        <QuietEmptyState
          title="No webhooks yet"
          description="Send new conversations, replies, missed SLAs and ratings to a CRM, a data warehouse or an automation tool."
          action={editable ? <Button size="sm" variant="outline" onClick={() => openEditor(null)}>Add a webhook</Button> : undefined}
        />
      ) : (
        <ul className="divide-y divide-border rounded-lg border border-border" aria-label="Webhooks">
          {webhooks.map((webhook) => {
            const status = health(webhook)
            const scope = webhook.mailbox_ids.length ? webhook.mailbox_ids.map((id) => inboxNames.get(id) ?? 'Archived inbox').join(', ') : 'All inboxes'
            return (
              <li key={webhook.id} className="flex flex-wrap items-center gap-3 px-4 py-3" data-testid="support-webhook">
                <div className="min-w-0 flex-1">
                  <div className="truncate font-mono text-xs text-foreground" title={webhook.url}>{webhook.url}</div>
                  <div className="mt-0.5 text-xs text-muted-foreground">
                    {[webhook.description, describeWebhookEvents(webhook.events), scope, `Secret ending in ${webhook.secret_hint}`].filter(Boolean).join(' · ')}
                  </div>
                  <div className={`mt-0.5 text-xs ${TONE_CLASS[status.tone]}`}>{status.text}</div>
                </div>
                <div className="flex items-center gap-1">
                  <Button variant="outline" size="sm" className="h-7 text-xs" onClick={() => setDeliveriesFor(webhook)}>Deliveries</Button>
                  {editable ? (
                    <>
                      <Switch
                        checked={webhook.enabled}
                        disabled={updateWebhook.isPending}
                        onCheckedChange={(enabled) => updateWebhook.mutate({ id: webhook.id, input: endpointInput(webhook, enabled) })}
                        aria-label={`${webhook.enabled ? 'Turn off' : 'Turn on'} webhook ${webhook.url}`}
                      />
                      <DropdownMenu>
                        <DropdownMenuTrigger asChild>
                          <Button variant="ghost" size="icon" className="h-7 w-7" aria-label={`Actions for webhook ${webhook.url}`}>
                            <MoreHorizontalIcon className="h-4 w-4" />
                          </Button>
                        </DropdownMenuTrigger>
                        <DropdownMenuContent align="end">
                          <DropdownMenuItem disabled={sendTest.isPending} onSelect={() => void test(webhook)}>Send test event</DropdownMenuItem>
                          <DropdownMenuItem onSelect={() => openEditor(webhook)}>Edit</DropdownMenuItem>
                          <DropdownMenuItem onSelect={() => void rotate(webhook)}>Create new secret</DropdownMenuItem>
                          <DropdownMenuSeparator />
                          <DropdownMenuItem className="text-destructive focus:text-destructive" onSelect={() => void remove(webhook)}>Delete</DropdownMenuItem>
                        </DropdownMenuContent>
                      </DropdownMenu>
                    </>
                  ) : null}
                </div>
              </li>
            )
          })}
        </ul>
      )}

      <SupportWebhookDialog
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        webhook={editing}
        inboxes={inboxes}
        saving={createWebhook.isPending || updateWebhook.isPending}
        onSave={save}
      />
      <WebhookSecretDialog secret={secret} onClose={() => setSecret(null)} />
      <WebhookDeliveriesDialog workspaceId={workspaceId} webhook={deliveriesFor} editable={editable} onClose={() => setDeliveriesFor(null)} />
    </div>
  )
}
