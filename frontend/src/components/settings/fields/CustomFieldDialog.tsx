import { useState } from 'react'
import { QuietIconAction, QuietPrimaryAction, QuietTextAction, QuietUnderlineInput } from '@/components/design-system/quiet'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/design-system/quiet-dropdown-select'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Label } from '@/components/ui/label'
import {
  CUSTOM_FIELD_KEY_PATTERN,
  CUSTOM_FIELD_TYPES,
  customFieldKeyFromLabel,
  isChoiceField,
  type CustomFieldDefinition,
  type CustomFieldDefinitionInput,
  type CustomFieldOption,
  type CustomFieldType,
} from '@/lib/customFields'
import { Delete01Icon, PlusSignIcon } from '@/lib/icons'

/** What the dialog edits: an existing field, or a new one with optional presets. */
export type CustomFieldDraft =
  | { mode: 'edit'; field: CustomFieldDefinition }
  | { mode: 'create'; preset?: Partial<CustomFieldDefinitionInput>; keyLocked?: boolean }

const MAX_OPTIONS = 100

export function CustomFieldDialog({
  draft,
  saving,
  onOpenChange,
  onSave,
}: {
  draft: CustomFieldDraft | null
  saving: boolean
  onOpenChange: (open: boolean) => void
  onSave: (input: CustomFieldDefinitionInput) => Promise<boolean>
}) {
  const editing = draft?.mode === 'edit'
  return (
    <Dialog open={Boolean(draft)} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{editing ? 'Edit field' : 'New field'}</DialogTitle>
          <DialogDescription>
            {editing ? 'The key stays the same. Stored values are never changed.' : 'The key is how integrations and the API refer to this field. It cannot change later.'}
          </DialogDescription>
        </DialogHeader>
        {draft ? (
          <CustomFieldForm
            key={draft.mode === 'edit' ? draft.field.id : `new:${draft.preset?.key ?? ''}`}
            draft={draft}
            saving={saving}
            onCancel={() => onOpenChange(false)}
            onSave={onSave}
          />
        ) : null}
      </DialogContent>
    </Dialog>
  )
}

function CustomFieldForm({
  draft,
  saving,
  onCancel,
  onSave,
}: {
  draft: CustomFieldDraft
  saving: boolean
  onCancel: () => void
  onSave: (input: CustomFieldDefinitionInput) => Promise<boolean>
}) {
  const initial: CustomFieldDefinitionInput = draft.mode === 'edit'
    ? { key: draft.field.key, label: draft.field.label, description: draft.field.description, field_type: draft.field.field_type, options: draft.field.options }
    : { key: '', label: '', description: '', field_type: 'text', options: [], ...draft.preset }
  const keyLocked = draft.mode === 'edit' || Boolean(draft.keyLocked)
  const originalType = draft.mode === 'edit' ? draft.field.field_type : null
  const savedOptions = new Set(draft.mode === 'edit' ? draft.field.options.map((option) => option.value) : [])

  const [label, setLabel] = useState(initial.label)
  const [key, setKey] = useState(initial.key)
  const [keyEdited, setKeyEdited] = useState(keyLocked || Boolean(initial.key))
  const [description, setDescription] = useState(initial.description)
  const [fieldType, setFieldType] = useState<CustomFieldType>(initial.field_type)
  const [options, setOptions] = useState<CustomFieldOption[]>(initial.options.length ? initial.options : [])

  const choice = isChoiceField(fieldType)
  const cleanOptions = options.map((option) => ({ value: option.value.trim(), label: option.label.trim() })).filter((option) => option.value)
  const duplicate = new Set(cleanOptions.map((option) => option.value.toLowerCase())).size !== cleanOptions.length
  const keyValid = CUSTOM_FIELD_KEY_PATTERN.test(key)
  const error = !keyValid && key
    ? 'Use lowercase letters, digits and underscores, starting with a letter.'
    : choice && cleanOptions.length === 0
      ? 'Add at least one option.'
      : duplicate
        ? 'Each option needs a different value.'
        : null
  const canSave = keyValid && !error && !saving

  const changeLabel = (next: string) => {
    setLabel(next)
    if (!keyEdited) setKey(customFieldKeyFromLabel(next))
  }

  const submit = () => {
    if (!canSave) return
    void onSave({ key, label: label.trim(), description: description.trim(), field_type: fieldType, options: choice ? cleanOptions : [] }).then((saved) => {
      if (saved) onCancel()
    })
  }

  return (
    <form className="space-y-4" onSubmit={(event) => { event.preventDefault(); submit() }}>
      <div className="space-y-1">
        <Label htmlFor="custom-field-label" className="text-sm font-medium text-quiet-text-secondary">Label</Label>
        <QuietUnderlineInput id="custom-field-label" value={label} maxLength={80} placeholder="For example: Billing plan" onChange={(event) => changeLabel(event.target.value)} autoFocus />
      </div>
      <div className="space-y-1">
        <Label htmlFor="custom-field-key" className="text-sm font-medium text-quiet-text-secondary">Key</Label>
        <QuietUnderlineInput
          id="custom-field-key"
          value={key}
          maxLength={64}
          disabled={keyLocked}
          className="font-mono text-xs"
          placeholder="billing_plan"
          aria-invalid={Boolean(key) && !keyValid}
          onChange={(event) => { setKeyEdited(true); setKey(event.target.value.trim()) }}
        />
      </div>
      <div className="space-y-1">
        <Label htmlFor="custom-field-type" className="text-sm font-medium text-quiet-text-secondary">Type</Label>
        <Select value={fieldType} onValueChange={(value) => setFieldType(value as CustomFieldType)}>
          <SelectTrigger id="custom-field-type" aria-label="Type" variant="underline" className="w-full px-0.5"><SelectValue /></SelectTrigger>
          <SelectContent>
            {CUSTOM_FIELD_TYPES.map((type) => <SelectItem key={type.value} value={type.value}>{type.label}</SelectItem>)}
          </SelectContent>
        </Select>
        {originalType && originalType !== fieldType ? (
          <p className="text-xs text-muted-foreground">Stored values stay as they are. Values that don&apos;t fit the new type show as empty until someone edits them.</p>
        ) : null}
      </div>
      {choice ? (
        <fieldset className="space-y-2">
          <legend className="text-sm font-medium text-quiet-text-secondary">Options</legend>
          {options.length === 0 ? <p className="text-xs text-muted-foreground">No options yet.</p> : null}
          <ol className="space-y-1.5">
            {options.map((option, index) => {
              const locked = savedOptions.has(option.value)
              return (
                <li key={index} className="grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto] items-center gap-2">
                  <QuietUnderlineInput
                    aria-label={`Option ${index + 1} value`}
                    value={option.value}
                    maxLength={100}
                    disabled={locked}
                    title={locked ? 'Saved values keep their option value; change the label instead.' : undefined}
                    placeholder="Value"
                    onChange={(event) => setOptions(options.map((entry, i) => i === index ? { ...entry, value: event.target.value } : entry))}
                  />
                  <QuietUnderlineInput
                    aria-label={`Option ${index + 1} label`}
                    value={option.label}
                    maxLength={80}
                    placeholder="Label (optional)"
                    onChange={(event) => setOptions(options.map((entry, i) => i === index ? { ...entry, label: event.target.value } : entry))}
                  />
                  <QuietIconAction type="button" aria-label={`Remove option ${index + 1}`} onClick={() => setOptions(options.filter((_, i) => i !== index))}>
                    <Delete01Icon className="h-4 w-4" />
                  </QuietIconAction>
                </li>
              )
            })}
          </ol>
          <QuietTextAction type="button" disabled={options.length >= MAX_OPTIONS} onClick={() => setOptions([...options, { value: '', label: '' }])}>
            <PlusSignIcon className="h-4 w-4" />
            Add option
          </QuietTextAction>
        </fieldset>
      ) : null}
      <div className="space-y-1">
        <Label htmlFor="custom-field-description" className="text-sm font-medium text-quiet-text-secondary">Description</Label>
        <QuietUnderlineInput id="custom-field-description" value={description} maxLength={300} placeholder="Optional, shown to teammates" onChange={(event) => setDescription(event.target.value)} />
      </div>
      {error ? <p role="alert" className="text-xs text-destructive">{error}</p> : null}
      <DialogFooter className="border-t border-quiet-divider-strong pt-3">
        <QuietTextAction type="button" disabled={saving} onClick={onCancel}>Cancel</QuietTextAction>
        <QuietPrimaryAction type="submit" disabled={!canSave}>{saving ? 'Saving…' : draft.mode === 'edit' ? 'Save field' : 'Create field'}</QuietPrimaryAction>
      </DialogFooter>
    </form>
  )
}
