import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import type { WebhookEndpoint, WebhookEndpointInput, WebhookEventType } from '@/lib/pmTypes'
import { toggleListValue } from '@/lib/toggleListValue'
import { WEBHOOK_EVENTS, webhookURLProblem } from '@/lib/supportWebhooks'
import { SettingsPill } from '../SettingsPill'

interface SupportWebhookDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  webhook: WebhookEndpoint | null
  inboxes: { id: string; name: string }[]
  saving: boolean
  onSave: (input: WebhookEndpointInput) => Promise<boolean>
}

/** Add or edit one webhook endpoint. */
export function SupportWebhookDialog({ open, onOpenChange, webhook, inboxes, saving, onSave }: SupportWebhookDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>{webhook ? 'Edit webhook' : 'Add webhook'}</DialogTitle>
          <DialogDescription>Helpin sends a signed POST request to this URL for each event, and retries failed deliveries for about a day.</DialogDescription>
        </DialogHeader>
        {open ? (
          <SupportWebhookForm
            // A fresh form for each opening starts from the webhook being edited.
            key={webhook?.id ?? 'new'}
            webhook={webhook}
            inboxes={inboxes}
            saving={saving}
            onCancel={() => onOpenChange(false)}
            onSave={async (input) => {
              if (await onSave(input)) onOpenChange(false)
            }}
          />
        ) : null}
      </DialogContent>
    </Dialog>
  )
}

function SupportWebhookForm({
  webhook,
  inboxes,
  saving,
  onCancel,
  onSave,
}: {
  webhook: WebhookEndpoint | null
  inboxes: { id: string; name: string }[]
  saving: boolean
  onCancel: () => void
  onSave: (input: WebhookEndpointInput) => Promise<void>
}) {
  const [url, setURL] = useState(webhook?.url ?? '')
  const [description, setDescription] = useState(webhook?.description ?? '')
  const [events, setEvents] = useState<WebhookEventType[]>(webhook?.events ?? [])
  const [mailboxIds, setMailboxIds] = useState<string[]>(webhook?.mailbox_ids ?? [])
  const [error, setError] = useState<string | null>(null)

  const submit = async () => {
    const problem = webhookURLProblem(url)
    setError(problem)
    if (problem) return
    await onSave({ url: url.trim(), description: description.trim(), events, mailbox_ids: mailboxIds, ...(webhook ? { enabled: webhook.enabled } : {}) })
  }

  return (
    <form
      className="space-y-5"
      onSubmit={(event) => {
        event.preventDefault()
        void submit()
      }}
    >
      <div className="space-y-1.5">
        <Label htmlFor="webhook-url">Endpoint URL</Label>
        <Input id="webhook-url" type="url" inputMode="url" autoComplete="off" value={url} placeholder="https://example.com/helpin/webhooks" onChange={(event) => setURL(event.target.value)} aria-invalid={Boolean(error)} aria-describedby={error ? 'webhook-url-error' : undefined} />
        {error ? <p id="webhook-url-error" role="alert" className="text-xs text-destructive">{error}</p> : null}
      </div>
      <div className="space-y-1.5">
        <Label htmlFor="webhook-description">Description <span className="font-normal text-muted-foreground">(optional)</span></Label>
        <Input id="webhook-description" value={description} maxLength={200} placeholder="For example: CRM sync" onChange={(event) => setDescription(event.target.value)} />
      </div>
      <fieldset className="space-y-2">
        <legend className="text-sm font-medium">Events</legend>
        <p className="text-xs text-muted-foreground">Leave all unselected to receive every event, including ones added later.</p>
        <div className="flex flex-wrap gap-1.5">
          {WEBHOOK_EVENTS.map((option) => (
            <SettingsPill key={option.value} selected={events.includes(option.value)} onClick={() => setEvents(toggleListValue(events, option.value))}>
              <span title={option.description}>{option.label}</span>
            </SettingsPill>
          ))}
        </div>
      </fieldset>
      <fieldset className="space-y-2">
        <legend className="text-sm font-medium">Inboxes</legend>
        <p className="text-xs text-muted-foreground">Leave all unselected to send events from every inbox.</p>
        <div className="flex flex-wrap gap-1.5">
          {[{ id: 'shared', name: 'Main inbox' }, ...inboxes].map((inbox) => (
            <SettingsPill key={inbox.id} selected={mailboxIds.includes(inbox.id)} onClick={() => setMailboxIds(toggleListValue(mailboxIds, inbox.id))}>{inbox.name}</SettingsPill>
          ))}
        </div>
      </fieldset>
      <DialogFooter>
        <Button type="button" variant="ghost" onClick={onCancel}>Cancel</Button>
        <Button type="submit" disabled={saving}>{saving ? 'Saving…' : webhook ? 'Save webhook' : 'Add webhook'}</Button>
      </DialogFooter>
    </form>
  )
}
