import type { ReactNode } from 'react'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { useInboxScopes, useSupportTags } from '@/hooks/queries/useSupport'
import { useWorkspaceMembers } from '@/hooks/queries/useWorkspaces'
import type { SupportConversationActions } from '@/lib/pmTypes'
import { MACRO_SNOOZE_OPTIONS } from '@/lib/supportMacros'
import { cn } from '@/lib/utils'

const NO_CHANGE = '__none__'
const UNASSIGN = '__unassign__'

function Field({ id, label, children }: { id: string; label: string; children: ReactNode }) {
  return (
    <div className="space-y-1">
      <Label htmlFor={id} className="text-[11px] font-medium text-muted-foreground">{label}</Label>
      {children}
    </div>
  )
}

function ActionSelect({
  id,
  value,
  onChange,
  options,
}: {
  id: string
  value: string
  onChange: (value: string) => void
  options: { value: string; label: string }[]
}) {
  return (
    <Select value={value} onValueChange={onChange}>
      <SelectTrigger id={id} className="h-8 w-full text-xs"><SelectValue /></SelectTrigger>
      <SelectContent>
        <SelectItem value={NO_CHANGE}>No change</SelectItem>
        {options.map((option) => <SelectItem key={option.value} value={option.value}>{option.label}</SelectItem>)}
      </SelectContent>
    </Select>
  )
}

function TagPills({
  label,
  tags,
  selected,
  disabled,
  onToggle,
}: {
  label: string
  tags: { id: string; name: string }[]
  selected: string[]
  disabled: string[]
  onToggle: (id: string) => void
}) {
  return (
    <div className="space-y-1">
      <div className="text-[11px] font-medium text-muted-foreground">{label}</div>
      {tags.length === 0 ? (
        <p className="text-[11px] text-muted-foreground">No tags yet.</p>
      ) : (
        <div className="flex flex-wrap gap-1">
          {tags.map((tag) => (
            <button
              key={tag.id}
              type="button"
              aria-pressed={selected.includes(tag.id)}
              disabled={disabled.includes(tag.id)}
              onClick={() => onToggle(tag.id)}
              className={cn(
                'rounded-full border px-2 py-0.5 text-[11px] transition-colors disabled:opacity-40',
                selected.includes(tag.id) ? 'border-foreground bg-foreground text-background' : 'border-border text-muted-foreground hover:bg-muted',
              )}
            >
              {tag.name}
            </button>
          ))}
        </div>
      )}
    </div>
  )
}

function toggle(list: string[] | undefined, id: string): string[] {
  const current = list ?? []
  return current.includes(id) ? current.filter((value) => value !== id) : [...current, id]
}

/** Edits the actions a shortcut runs when used. */
export function MacroActionsEditor({
  workspaceId,
  value,
  onChange,
}: {
  workspaceId: string
  value: SupportConversationActions
  onChange: (value: SupportConversationActions) => void
}) {
  const { data: tags = [] } = useSupportTags(workspaceId)
  const { data: members = [] } = useWorkspaceMembers(workspaceId)
  const { data: scopes } = useInboxScopes(workspaceId)
  const activeMembers = members.filter((member) => member.user_id && member.role !== 'viewer')
  const patch = (next: Partial<SupportConversationActions>) => onChange({ ...value, ...next })
  const pick = <T extends string>(selected: string): T | undefined => (selected === NO_CHANGE ? undefined : (selected as T))

  return (
    <div className="space-y-3" data-testid="macro-actions-editor">
      <div className="grid gap-2 sm:grid-cols-2">
        <Field id="macro-status" label="Status">
          <ActionSelect
            id="macro-status"
            value={value.status ?? NO_CHANGE}
            onChange={(selected) => patch({ status: pick(selected), ...(selected === 'resolved' ? { snooze_minutes: undefined } : {}) })}
            options={[{ value: 'open', label: 'Reopen' }, { value: 'waiting_on_customer', label: 'Waiting on customer' }, { value: 'resolved', label: 'Resolve' }]}
          />
        </Field>
        <Field id="macro-priority" label="Priority">
          <ActionSelect
            id="macro-priority"
            value={value.priority ?? NO_CHANGE}
            onChange={(selected) => patch({ priority: pick(selected) })}
            options={[{ value: 'urgent', label: 'Urgent' }, { value: 'high', label: 'High' }, { value: 'medium', label: 'Medium' }, { value: 'low', label: 'Low' }]}
          />
        </Field>
        <Field id="macro-assignee" label="Assignee">
          <ActionSelect
            id="macro-assignee"
            value={value.assign_user_id === undefined ? NO_CHANGE : value.assign_user_id === '' ? UNASSIGN : value.assign_user_id}
            onChange={(selected) => patch({ assign_user_id: selected === NO_CHANGE ? undefined : selected === UNASSIGN ? '' : selected })}
            options={[
              { value: UNASSIGN, label: 'Unassign' },
              ...activeMembers.map((member) => ({ value: member.user_id!, label: member.full_name || member.email || 'Teammate' })),
            ]}
          />
        </Field>
        <Field id="macro-inbox" label="Move to inbox">
          <ActionSelect
            id="macro-inbox"
            value={value.mailbox_id ?? NO_CHANGE}
            onChange={(selected) => patch({ mailbox_id: pick(selected) })}
            options={[
              { value: 'shared', label: scopes?.shared_inbox?.name || 'Main inbox' },
              ...(scopes?.mailboxes ?? []).map((mailbox) => ({ value: mailbox.id, label: mailbox.name })),
            ]}
          />
        </Field>
        <Field id="macro-snooze" label="Snooze">
          <Select
            value={value.snooze_minutes ? String(value.snooze_minutes) : NO_CHANGE}
            onValueChange={(selected) => patch({ snooze_minutes: selected === NO_CHANGE ? undefined : Number(selected) })}
            disabled={value.status === 'resolved'}
          >
            <SelectTrigger id="macro-snooze" className="h-8 w-full text-xs"><SelectValue /></SelectTrigger>
            <SelectContent>
              <SelectItem value={NO_CHANGE}>No change</SelectItem>
              {MACRO_SNOOZE_OPTIONS.map((option) => <SelectItem key={option.minutes} value={String(option.minutes)}>{option.label}</SelectItem>)}
            </SelectContent>
          </Select>
        </Field>
      </div>
      <TagPills
        label="Add tags"
        tags={tags}
        selected={value.add_tag_ids ?? []}
        disabled={value.remove_tag_ids ?? []}
        onToggle={(id) => patch({ add_tag_ids: toggle(value.add_tag_ids, id) })}
      />
      <TagPills
        label="Remove tags"
        tags={tags}
        selected={value.remove_tag_ids ?? []}
        disabled={value.add_tag_ids ?? []}
        onToggle={(id) => patch({ remove_tag_ids: toggle(value.remove_tag_ids, id) })}
      />
    </div>
  )
}
