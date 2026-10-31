import { useState } from 'react'
import { QuietDropdown } from '@/components/design-system/quiet-dropdown'
import { QuietIconAction, QuietPrimaryAction, QuietTextAction, QuietUnderlineInput } from '@/components/design-system/quiet'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/design-system/quiet-dropdown-select'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { useCreateCustomField } from '@/hooks/queries/useCustomFields'
import {
  customFieldTypeLabel,
  type CustomFieldDefinition,
  type CustomFieldDefinitionInput,
  type RequestTypeField,
  type SupportRequestType,
  type SupportRequestTypeInput,
} from '@/lib/customFields'
import { ArrowDown01Icon, ArrowUp01Icon, Delete01Icon, PlusSignIcon } from '@/lib/icons'
import { CONVERSATION_PRIORITY_OPTIONS } from '@/lib/supportInboxFilters'
import { CustomFieldDialog } from './CustomFieldDialog'

const NONE = '__none__'
const MAX_FIELDS = 30

/** Editor for a request type: what customers see, defaults, and its fields. */
export function RequestTypeDialog({
  open,
  requestType,
  workspaceId,
  fields,
  inboxes,
  saving,
  onOpenChange,
  onSave,
}: {
  open: boolean
  requestType: SupportRequestType | null
  workspaceId: string
  fields: CustomFieldDefinition[]
  inboxes: { id: string; name: string }[]
  saving: boolean
  onOpenChange: (open: boolean) => void
  onSave: (input: SupportRequestTypeInput) => Promise<boolean>
}) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>{requestType ? 'Edit request type' : 'New request type'}</DialogTitle>
          <DialogDescription>Customers choose a request type on the portal, then fill in its fields.</DialogDescription>
        </DialogHeader>
        {open ? (
          <RequestTypeForm
            key={requestType?.id ?? 'new'}
            requestType={requestType}
            workspaceId={workspaceId}
            fields={fields}
            inboxes={inboxes}
            saving={saving}
            onCancel={() => onOpenChange(false)}
            onSave={onSave}
          />
        ) : null}
      </DialogContent>
    </Dialog>
  )
}

function RequestTypeForm({
  requestType,
  workspaceId,
  fields,
  inboxes,
  saving,
  onCancel,
  onSave,
}: {
  requestType: SupportRequestType | null
  workspaceId: string
  fields: CustomFieldDefinition[]
  inboxes: { id: string; name: string }[]
  saving: boolean
  onCancel: () => void
  onSave: (input: SupportRequestTypeInput) => Promise<boolean>
}) {
  const [name, setName] = useState(requestType?.name ?? '')
  const [description, setDescription] = useState(requestType?.description ?? '')
  const [enabled, setEnabled] = useState(requestType?.enabled ?? true)
  const [priority, setPriority] = useState(requestType?.default_priority ?? NONE)
  const [mailbox, setMailbox] = useState(requestType?.mailbox_id ?? NONE)
  const [selected, setSelected] = useState<RequestTypeField[]>(requestType?.fields ?? [])
  const [creatingField, setCreatingField] = useState(false)
  const createField = useCreateCustomField(workspaceId, 'conversation')
  const byKey = new Map(fields.map((field) => [field.key, field]))
  const available = fields.filter((field) => !selected.some((entry) => entry.key === field.key))
  const trimmed = name.trim()

  const patch = (index: number, change: Partial<RequestTypeField>) =>
    setSelected(selected.map((entry, i) => (i === index ? { ...entry, ...change } : entry)))
  const move = (index: number, direction: -1 | 1) => {
    const next = [...selected]
    ;[next[index], next[index + direction]] = [next[index + direction], next[index]]
    setSelected(next)
  }
  const add = (key: string) => setSelected([...selected, { key, required: false, customer_visible: true, help_text: '' }])

  const submit = () => {
    if (!trimmed || saving) return
    void onSave({
      name: trimmed,
      description: description.trim(),
      enabled,
      default_priority: priority === NONE ? null : priority,
      mailbox_id: mailbox === NONE ? null : mailbox,
      // Archived fields drop out when the type is saved.
      fields: selected.filter((entry) => byKey.has(entry.key)),
    }).then((saved) => { if (saved) onCancel() })
  }

  const createAndAdd = async (input: CustomFieldDefinitionInput) => {
    try {
      const field = await createField.mutateAsync(input)
      add(field.key)
      return true
    } catch {
      return false
    }
  }

  return (
    <>
    <form className="space-y-4" onSubmit={(event) => { event.preventDefault(); submit() }}>
      <div className="space-y-1">
        <Label htmlFor="request-type-name" className="text-sm font-medium text-quiet-text-secondary">Name</Label>
        <QuietUnderlineInput id="request-type-name" value={name} maxLength={60} placeholder="For example: Bug report" onChange={(event) => setName(event.target.value)} autoFocus />
      </div>
      <div className="space-y-1">
        <Label htmlFor="request-type-description" className="text-sm font-medium text-quiet-text-secondary">Description</Label>
        <QuietUnderlineInput id="request-type-description" value={description} maxLength={300} placeholder="Optional, shown to customers when they choose" onChange={(event) => setDescription(event.target.value)} />
      </div>
      <div className="grid gap-4 sm:grid-cols-2">
        <div className="space-y-1">
          <Label htmlFor="request-type-priority" className="text-sm font-medium text-quiet-text-secondary">Priority</Label>
          <Select value={priority} onValueChange={setPriority}>
            <SelectTrigger id="request-type-priority" aria-label="Priority" variant="underline" className="w-full px-0.5"><SelectValue /></SelectTrigger>
            <SelectContent>
              <SelectItem value={NONE}>Default (medium)</SelectItem>
              {CONVERSATION_PRIORITY_OPTIONS.map((option) => <SelectItem key={option.value} value={option.value}>{option.label}</SelectItem>)}
            </SelectContent>
          </Select>
        </div>
        <div className="space-y-1">
          <Label htmlFor="request-type-inbox" className="text-sm font-medium text-quiet-text-secondary">Inbox</Label>
          <Select value={mailbox} onValueChange={setMailbox}>
            <SelectTrigger id="request-type-inbox" aria-label="Inbox" variant="underline" className="w-full px-0.5"><SelectValue /></SelectTrigger>
            <SelectContent>
              <SelectItem value={NONE}>Usual routing</SelectItem>
              {inboxes.map((inbox) => <SelectItem key={inbox.id} value={inbox.id}>{inbox.name}</SelectItem>)}
            </SelectContent>
          </Select>
        </div>
      </div>

      <fieldset className="space-y-2">
        <legend className="text-sm font-medium text-quiet-text-secondary">Fields</legend>
        {selected.length === 0 ? <p className="text-xs text-muted-foreground">No fields: customers write a subject and message only.</p> : null}
        <ol className="divide-y divide-border rounded-md border border-border" aria-label="Fields on this request type">
          {selected.map((entry, index) => {
            const field = byKey.get(entry.key)
            const label = field?.display_label ?? entry.key
            return (
              <li key={entry.key} className="space-y-2 px-3 py-2.5" data-testid="request-type-field">
                <div className="flex flex-wrap items-center gap-2">
                  <div className="min-w-0 flex-1">
                    <span className="text-sm font-medium">{label}</span>{' '}
                    <span className="text-xs text-muted-foreground">{field ? customFieldTypeLabel(field.field_type) : 'Archived field'}</span>
                  </div>
                  <QuietIconAction type="button" aria-label={`Move ${label} up`} disabled={index === 0} onClick={() => move(index, -1)}>
                    <ArrowUp01Icon className="h-4 w-4" />
                  </QuietIconAction>
                  <QuietIconAction type="button" aria-label={`Move ${label} down`} disabled={index === selected.length - 1} onClick={() => move(index, 1)}>
                    <ArrowDown01Icon className="h-4 w-4" />
                  </QuietIconAction>
                  <QuietIconAction type="button" aria-label={`Remove ${label}`} onClick={() => setSelected(selected.filter((_, i) => i !== index))}>
                    <Delete01Icon className="h-4 w-4" />
                  </QuietIconAction>
                </div>
                <div className="flex flex-wrap items-center gap-x-5 gap-y-2 text-xs">
                  <label className="inline-flex items-center gap-2">
                    <Switch
                      checked={entry.customer_visible}
                      aria-label={`Customers see ${label}`}
                      onCheckedChange={(visible) => patch(index, { customer_visible: visible, required: visible && entry.required })}
                    />
                    Customers fill in
                  </label>
                  <label className="inline-flex items-center gap-2">
                    <Switch
                      checked={entry.required}
                      disabled={!entry.customer_visible}
                      aria-label={`${label} is required`}
                      onCheckedChange={(required) => patch(index, { required })}
                    />
                    Required
                  </label>
                  {!entry.customer_visible ? <span className="text-muted-foreground">Internal: teammates fill it in</span> : null}
                </div>
                {entry.customer_visible ? (
                  <QuietUnderlineInput
                    aria-label={`Help text for ${label}`}
                    value={entry.help_text}
                    maxLength={300}
                    placeholder={field?.description || 'Help text for customers (optional)'}
                    onChange={(event) => patch(index, { help_text: event.target.value })}
                  />
                ) : null}
              </li>
            )
          })}
        </ol>
        <div className="flex flex-wrap items-center gap-3">
          <QuietDropdown
            label="Add field"
            options={available.map((field) => ({ value: field.key, label: field.display_label, trailing: <span className="text-xs text-muted-foreground">{customFieldTypeLabel(field.field_type)}</span> }))}
            empty="Every field is on this type."
            onSelect={add}
            disabled={selected.length >= MAX_FIELDS}
            trigger={
              <QuietTextAction type="button" disabled={selected.length >= MAX_FIELDS}>
                <PlusSignIcon className="h-4 w-4" />
                Add field
              </QuietTextAction>
            }
          />
          <QuietTextAction type="button" disabled={selected.length >= MAX_FIELDS} onClick={() => setCreatingField(true)}>
            Create a field
          </QuietTextAction>
        </div>
      </fieldset>

      <label className="flex items-center justify-between gap-3 text-sm">
        <span>
          <span className="font-medium">Customers can choose it</span>
          <span className="block text-xs text-muted-foreground">Turned off, it leaves the portal; conversations keep it.</span>
        </span>
        <Switch checked={enabled} onCheckedChange={setEnabled} aria-label="Customers can choose this request type" />
      </label>

      <DialogFooter className="border-t border-quiet-divider-strong pt-3">
        <QuietTextAction type="button" disabled={saving} onClick={onCancel}>Cancel</QuietTextAction>
        <QuietPrimaryAction type="submit" disabled={saving || !trimmed}>{saving ? 'Saving…' : requestType ? 'Save request type' : 'Create request type'}</QuietPrimaryAction>
      </DialogFooter>
    </form>
    {/* Outside the form: portaled dialogs still bubble React submit events. */}
    <CustomFieldDialog
      draft={creatingField ? { mode: 'create' } : null}
      saving={createField.isPending}
      onOpenChange={(open) => { if (!open) setCreatingField(false) }}
      onSave={createAndAdd}
    />
    </>
  )
}
