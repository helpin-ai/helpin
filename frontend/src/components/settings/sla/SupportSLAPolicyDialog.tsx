import { useState, type ReactNode } from 'react'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import type { SupportSLAPolicy, SupportSLAPolicyInput } from '@/lib/pmTypes'
import {
  draftToPolicyInput,
  emptySLAPolicyDraft,
  policyToDraft,
  type SLAPolicyDraft,
  type SLATargetDraft,
  type SLATargetUnit,
} from '@/lib/supportSLAPolicyForm'
import { cn } from '@/lib/utils'

const PRIORITIES = [
  { value: 'urgent', label: 'Urgent' },
  { value: 'high', label: 'High' },
  { value: 'medium', label: 'Medium' },
  { value: 'low', label: 'Low' },
]

const CHANNELS = [
  { value: 'widget', label: 'Chat' },
  { value: 'email', label: 'Email' },
  { value: 'portal', label: 'Portal' },
  { value: 'api', label: 'API' },
  { value: 'internal', label: 'Started by team' },
]

function Pill({ selected, onClick, children }: { selected: boolean; onClick: () => void; children: ReactNode }) {
  return (
    <button
      type="button"
      aria-pressed={selected}
      onClick={onClick}
      className={cn(
        'rounded-full border px-2.5 py-1 text-xs transition-colors focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-primary',
        selected ? 'border-foreground bg-foreground text-background' : 'border-border text-muted-foreground hover:bg-muted',
      )}
    >
      {children}
    </button>
  )
}

function toggle(list: string[], value: string): string[] {
  return list.includes(value) ? list.filter((item) => item !== value) : [...list, value]
}

function TargetRow({
  id,
  label,
  help,
  value,
  onChange,
}: {
  id: string
  label: string
  help: string
  value: SLATargetDraft
  onChange: (value: SLATargetDraft) => void
}) {
  return (
    <div className="flex flex-wrap items-center justify-between gap-3 rounded-md border border-border/70 px-3 py-2.5">
      <div className="min-w-0 flex-1">
        <Label htmlFor={`${id}-enabled`} className="text-sm">{label}</Label>
        <p className="text-xs text-muted-foreground">{help}</p>
      </div>
      <div className="flex items-center gap-2">
        <Input
          id={`${id}-amount`}
          aria-label={`${label} target`}
          type="number"
          min={1}
          step={1}
          className="h-8 w-20"
          value={value.amount}
          disabled={!value.enabled}
          onChange={(event) => onChange({ ...value, amount: event.target.value })}
        />
        <Select value={value.unit} onValueChange={(unit) => onChange({ ...value, unit: unit as SLATargetUnit })} disabled={!value.enabled}>
          <SelectTrigger aria-label={`${label} unit`} className="h-8 w-[100px]"><SelectValue /></SelectTrigger>
          <SelectContent>
            <SelectItem value="minutes">minutes</SelectItem>
            <SelectItem value="hours">hours</SelectItem>
            <SelectItem value="days">days</SelectItem>
          </SelectContent>
        </Select>
        <Switch id={`${id}-enabled`} checked={value.enabled} onCheckedChange={(enabled) => onChange({ ...value, enabled })} aria-label={`Use a ${label.toLowerCase()} target`} />
      </div>
    </div>
  )
}

interface SupportSLAPolicyDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  policy: SupportSLAPolicy | null
  inboxes: { id: string; name: string }[]
  businessHoursConfigured: boolean
  saving: boolean
  onSave: (input: SupportSLAPolicyInput) => Promise<boolean>
}

/** Create or edit one SLA policy. */
export function SupportSLAPolicyDialog({ open, onOpenChange, policy, inboxes, businessHoursConfigured, saving, onSave }: SupportSLAPolicyDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>{policy ? 'Edit SLA policy' : 'New SLA policy'}</DialogTitle>
          <DialogDescription>
            Targets apply to conversations created after the policy is turned on. The first enabled policy that matches a conversation applies.
            {policy ? ' Changed targets apply to conversations that start this policy from now on; results already recorded never change.' : null}
          </DialogDescription>
        </DialogHeader>
        {open ? (
          <SupportSLAPolicyForm
            // A fresh form for each opening starts from the policy being edited.
            key={policy?.id ?? 'new'}
            policy={policy}
            inboxes={inboxes}
            businessHoursConfigured={businessHoursConfigured}
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

function SupportSLAPolicyForm({
  policy,
  inboxes,
  businessHoursConfigured,
  saving,
  onCancel,
  onSave,
}: {
  policy: SupportSLAPolicy | null
  inboxes: { id: string; name: string }[]
  businessHoursConfigured: boolean
  saving: boolean
  onCancel: () => void
  onSave: (input: SupportSLAPolicyInput) => Promise<void>
}) {
  const [draft, setDraft] = useState<SLAPolicyDraft>(() => (policy ? policyToDraft(policy) : emptySLAPolicyDraft()))
  const [errors, setErrors] = useState<string[]>([])

  const update = (patch: Partial<SLAPolicyDraft>) => setDraft((current) => ({ ...current, ...patch }))

  const submit = async () => {
    const { input, errors: nextErrors } = draftToPolicyInput(draft)
    setErrors(nextErrors)
    if (input) await onSave(input)
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
        <Label htmlFor="sla-policy-name">Name</Label>
        <Input id="sla-policy-name" value={draft.name} maxLength={120} placeholder="For example: Urgent customers" onChange={(event) => update({ name: event.target.value })} />
      </div>

      <fieldset className="space-y-3">
        <legend className="text-sm font-medium">Applies to</legend>
        <p className="text-xs text-muted-foreground">Leave a group empty to match everything.</p>
        <div className="space-y-1.5">
          <div className="text-xs font-medium text-muted-foreground">Priority</div>
          <div className="flex flex-wrap gap-1.5">
            {PRIORITIES.map((option) => (
              <Pill key={option.value} selected={draft.priorities.includes(option.value)} onClick={() => update({ priorities: toggle(draft.priorities, option.value) })}>{option.label}</Pill>
            ))}
          </div>
        </div>
        <div className="space-y-1.5">
          <div className="text-xs font-medium text-muted-foreground">Inbox</div>
          <div className="flex flex-wrap gap-1.5">
            {[{ id: 'shared', name: 'Main inbox' }, ...inboxes].map((inbox) => (
              <Pill key={inbox.id} selected={draft.mailboxIds.includes(inbox.id)} onClick={() => update({ mailboxIds: toggle(draft.mailboxIds, inbox.id) })}>{inbox.name}</Pill>
            ))}
          </div>
        </div>
        <div className="space-y-1.5">
          <div className="text-xs font-medium text-muted-foreground">Channel</div>
          <div className="flex flex-wrap gap-1.5">
            {CHANNELS.map((option) => (
              <Pill key={option.value} selected={draft.channels.includes(option.value)} onClick={() => update({ channels: toggle(draft.channels, option.value) })}>{option.label}</Pill>
            ))}
          </div>
        </div>
      </fieldset>

      <fieldset className="space-y-2">
        <legend className="mb-1 text-sm font-medium">Targets</legend>
        <TargetRow id="sla-first" label="First response" help="From the customer's first message to the first teammate reply." value={draft.firstResponse} onChange={(firstResponse) => update({ firstResponse })} />
        <TargetRow id="sla-next" label="Next response" help="From each later customer message to the next teammate reply." value={draft.nextResponse} onChange={(nextResponse) => update({ nextResponse })} />
        <TargetRow id="sla-resolution" label="Resolution" help="From creation until the conversation is resolved." value={draft.resolution} onChange={(resolution) => update({ resolution })} />
      </fieldset>

      <div className="space-y-3">
        <div className="flex items-start justify-between gap-4">
          <div>
            <Label htmlFor="sla-business-hours" className="text-sm">Count business hours only</Label>
            <p className="text-xs text-muted-foreground">
              {businessHoursConfigured
                ? 'Clocks stop outside the business hours set in Chat widget settings.'
                : 'Business hours are not set up yet, so targets run on calendar time. Set them in Chat widget settings.'}
            </p>
          </div>
          <Switch id="sla-business-hours" checked={draft.businessHours} onCheckedChange={(businessHours) => update({ businessHours })} />
        </div>
        <div className="flex items-start justify-between gap-4">
          <div>
            <Label htmlFor="sla-pause" className="text-sm">Pause resolution while waiting on the customer</Label>
            <p className="text-xs text-muted-foreground">The resolution clock stops while the team has replied last, the status is waiting, or the conversation is snoozed.</p>
          </div>
          <Switch id="sla-pause" checked={draft.pauseWhileWaiting} onCheckedChange={(pauseWhileWaiting) => update({ pauseWhileWaiting })} />
        </div>
        <div className="flex items-start justify-between gap-4">
          <div>
            <Label htmlFor="sla-enabled" className="text-sm">Enabled</Label>
            <p className="text-xs text-muted-foreground">Turning a policy on again applies it only to conversations created afterwards.</p>
          </div>
          <Switch id="sla-enabled" checked={draft.enabled} onCheckedChange={(enabled) => update({ enabled })} />
        </div>
      </div>

      {errors.length > 0 ? (
        <ul role="alert" className="space-y-0.5 rounded-md border border-destructive/30 bg-destructive/5 px-3 py-2 text-xs text-destructive">
          {errors.map((error) => <li key={error}>{error}</li>)}
        </ul>
      ) : null}

      <DialogFooter>
        <Button type="button" variant="ghost" onClick={onCancel}>Cancel</Button>
        <Button type="submit" disabled={saving}>{saving ? 'Saving…' : policy ? 'Save policy' : 'Create policy'}</Button>
      </DialogFooter>
    </form>
  )
}
