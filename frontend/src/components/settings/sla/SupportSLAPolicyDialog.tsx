import { useState } from 'react'
import { QuietDropdown } from '@/components/design-system/quiet-dropdown'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { SUPPORT_SLA_NO_TIER, type SupportSLAPolicy, type SupportSLAPolicyInput, type SupportSLATier } from '@/lib/pmTypes'
import {
  MAX_SLA_POLICY_COMPANIES,
  draftToPolicyInput,
  emptySLAPolicyDraft,
  policyToDraft,
  type SLAPolicyDraft,
  type SLATargetDraft,
  type SLATargetUnit,
} from '@/lib/supportSLAPolicyForm'
import { toggleListValue } from '@/lib/toggleListValue'
import { SettingsPill } from '../SettingsPill'
import { SLACompanyPicker } from './SLACompanyPicker'

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
  workspaceId: string
  inboxes: { id: string; name: string }[]
  tiers: SupportSLATier[]
  /** Teammates who can be notified or assigned, by user ID. */
  teammates: { id: string; name: string }[]
  businessHoursConfigured: boolean
  saving: boolean
  onSave: (input: SupportSLAPolicyInput) => Promise<boolean>
}

/** Create or edit one SLA policy. */
export function SupportSLAPolicyDialog({ open, onOpenChange, policy, workspaceId, inboxes, tiers, teammates, businessHoursConfigured, saving, onSave }: SupportSLAPolicyDialogProps) {
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
            workspaceId={workspaceId}
            inboxes={inboxes}
            tiers={tiers}
            teammates={teammates}
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
  workspaceId,
  inboxes,
  tiers,
  teammates,
  businessHoursConfigured,
  saving,
  onCancel,
  onSave,
}: {
  policy: SupportSLAPolicy | null
  workspaceId: string
  inboxes: { id: string; name: string }[]
  tiers: SupportSLATier[]
  teammates: { id: string; name: string }[]
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
              <SettingsPill key={option.value} selected={draft.priorities.includes(option.value)} onClick={() => update({ priorities: toggleListValue(draft.priorities, option.value) })}>{option.label}</SettingsPill>
            ))}
          </div>
        </div>
        <div className="space-y-1.5">
          <div className="text-xs font-medium text-muted-foreground">Inbox</div>
          <div className="flex flex-wrap gap-1.5">
            {[{ id: 'shared', name: 'Main inbox' }, ...inboxes].map((inbox) => (
              <SettingsPill key={inbox.id} selected={draft.mailboxIds.includes(inbox.id)} onClick={() => update({ mailboxIds: toggleListValue(draft.mailboxIds, inbox.id) })}>{inbox.name}</SettingsPill>
            ))}
          </div>
        </div>
        <div className="space-y-1.5">
          <div className="text-xs font-medium text-muted-foreground">Channel</div>
          <div className="flex flex-wrap gap-1.5">
            {CHANNELS.map((option) => (
              <SettingsPill key={option.value} selected={draft.channels.includes(option.value)} onClick={() => update({ channels: toggleListValue(draft.channels, option.value) })}>{option.label}</SettingsPill>
            ))}
          </div>
        </div>
        {tiers.length > 0 || draft.companyTierIds.length > 0 ? (
          <div className="space-y-1.5">
            <div className="text-xs font-medium text-muted-foreground">Company tier</div>
            <div className="flex flex-wrap gap-1.5">
              {[...tiers.map((tier) => ({ id: tier.id, name: tier.name })), { id: SUPPORT_SLA_NO_TIER, name: 'No tier' }].map((tier) => (
                <SettingsPill key={tier.id} selected={draft.companyTierIds.includes(tier.id)} onClick={() => update({ companyTierIds: toggleListValue(draft.companyTierIds, tier.id) })}>{tier.name}</SettingsPill>
              ))}
            </div>
            <p className="text-xs text-muted-foreground">“No tier” matches companies without a tier and conversations without a company.</p>
          </div>
        ) : null}
        <div className="space-y-1.5">
          <div className="text-xs font-medium text-muted-foreground">Company</div>
          <SLACompanyPicker workspaceId={workspaceId} value={draft.companies} limit={MAX_SLA_POLICY_COMPANIES} onChange={(companies) => update({ companies })} />
        </div>
      </fieldset>

      <fieldset className="space-y-2">
        <legend className="mb-1 text-sm font-medium">Targets</legend>
        <TargetRow id="sla-first" label="First response" help="From the customer's first message to the first teammate reply." value={draft.firstResponse} onChange={(firstResponse) => update({ firstResponse })} />
        <TargetRow id="sla-next" label="Next response" help="From each later customer message to the next teammate reply." value={draft.nextResponse} onChange={(nextResponse) => update({ nextResponse })} />
        <TargetRow id="sla-resolution" label="Resolution" help="From creation until the conversation is resolved." value={draft.resolution} onChange={(resolution) => update({ resolution })} />
      </fieldset>

      <SLAAlertFields draft={draft} teammates={teammates} update={update} />

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

const NOBODY = '__nobody__'

/** Warnings before a target is missed, and who else hears about them. */
function SLAAlertFields({
  draft,
  teammates,
  update,
}: {
  draft: SLAPolicyDraft
  teammates: { id: string; name: string }[]
  update: (patch: Partial<SLAPolicyDraft>) => void
}) {
  const names = new Map(teammates.map((teammate) => [teammate.id, teammate.name]))
  const name = (id: string) => names.get(id) ?? 'Former teammate'
  const notified = draft.notifyUserIds.map(name)
  return (
    <fieldset className="space-y-2">
      <legend className="mb-1 text-sm font-medium">Before a breach</legend>
      <TargetRow
        id="sla-warn"
        label="Warn before a target is missed"
        help="Notifies the assignee, or the inbox's teammates when nobody is assigned. Each due time warns once."
        value={draft.warnBefore}
        onChange={(warnBefore) => update({ warnBefore })}
      />
      <div className="space-y-3 rounded-md border border-border/70 px-3 py-2.5">
        <div className="space-y-1.5">
          <Label className="text-sm">Also notify, on warnings and breaches</Label>
          <QuietDropdown
            label="Teammates to notify"
            multiple
            selected={draft.notifyUserIds}
            options={teammates.map((teammate) => ({ value: teammate.id, label: teammate.name }))}
            onSelect={(id) => update({ notifyUserIds: toggleListValue(draft.notifyUserIds, id) })}
            trigger={(
              <button type="button" aria-label="Teammates to notify" className="flex w-full items-center justify-between gap-2 rounded-md border border-border px-2.5 py-1.5 text-left text-xs hover:bg-muted">
                <span className={notified.length ? 'truncate' : 'truncate text-muted-foreground'}>{notified.length ? notified.join(', ') : 'Nobody else'}</span>
              </button>
            )}
          />
        </div>
        <div className="flex items-start justify-between gap-4">
          <div>
            <Label htmlFor="sla-notify-cs-owner" className="text-sm">Notify the company's customer success owner</Label>
            <p className="text-xs text-muted-foreground">When the conversation's company has one.</p>
          </div>
          <Switch id="sla-notify-cs-owner" checked={draft.notifyCSOwner} onCheckedChange={(notifyCSOwner) => update({ notifyCSOwner })} />
        </div>
        {draft.warnBefore.enabled ? (
          <div className="space-y-1.5">
            <Label className="text-sm">If nobody is assigned at the warning, assign to</Label>
            <Select value={draft.assignUserId ?? NOBODY} onValueChange={(value) => update({ assignUserId: value === NOBODY ? null : value })}>
              <SelectTrigger aria-label="Assign at the warning" className="h-8 w-full"><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value={NOBODY}>Nobody, only notify</SelectItem>
                {draft.assignUserId && !names.has(draft.assignUserId) ? <SelectItem value={draft.assignUserId}>Former teammate</SelectItem> : null}
                {teammates.map((teammate) => <SelectItem key={teammate.id} value={teammate.id}>{teammate.name}</SelectItem>)}
              </SelectContent>
            </Select>
          </div>
        ) : null}
      </div>
    </fieldset>
  )
}
