import { useState, type KeyboardEvent } from 'react'
import { QuietDropdown } from '@/components/design-system/quiet-dropdown'
import { DatePicker } from '@/components/ui/date-picker'
import {
  customFieldValueFits,
  formatCustomFieldValue,
  rawCustomFieldValue,
  type CustomFieldDefinition,
} from '@/lib/customFields'
import { cn } from '@/lib/utils'

const inputClassName =
  'w-full rounded bg-transparent px-1.5 py-0.5 text-xs text-foreground outline-none transition-colors placeholder:text-muted-foreground hover:bg-accent focus:bg-accent'

const triggerClassName =
  'flex w-full min-w-0 items-center rounded px-1.5 py-0.5 text-left text-xs transition-colors hover:bg-accent focus-visible:bg-accent focus-visible:outline-none'

const NONE = '__none__'

/** A text or number input that saves on blur or Enter and resets on Escape. */
function CommitInput({
  definition,
  initial,
  onSave,
}: {
  definition: CustomFieldDefinition
  initial: string
  onSave: (value: unknown) => void
}) {
  const [draft, setDraft] = useState(initial)
  const [synced, setSynced] = useState(initial)
  if (initial !== synced) {
    setSynced(initial)
    setDraft(initial)
  }
  const number = definition.field_type === 'number'
  const commit = () => {
    const next = draft.trim()
    if (next === initial.trim()) return
    if (!next) onSave(null)
    else onSave(number ? Number(next) : next)
  }
  const onKeyDown = (event: KeyboardEvent<HTMLInputElement | HTMLTextAreaElement>) => {
    if (event.key === 'Escape') {
      setDraft(initial)
      event.currentTarget.blur()
    } else if (event.key === 'Enter' && (definition.field_type !== 'long_text' || event.metaKey || event.ctrlKey)) {
      event.preventDefault()
      event.currentTarget.blur()
    }
  }
  if (definition.field_type === 'long_text') {
    return (
      <textarea
        aria-label={definition.display_label}
        className={cn(inputClassName, 'min-h-[1.75rem] resize-y')}
        rows={Math.min(6, Math.max(1, draft.split('\n').length))}
        maxLength={10000}
        value={draft}
        placeholder="None"
        onChange={(event) => setDraft(event.target.value)}
        onBlur={commit}
        onKeyDown={onKeyDown}
      />
    )
  }
  return (
    <input
      aria-label={definition.display_label}
      className={cn(inputClassName, number && '[appearance:textfield] [&::-webkit-inner-spin-button]:appearance-none [&::-webkit-outer-spin-button]:appearance-none')}
      type={number ? 'number' : 'text'}
      inputMode={number ? 'decimal' : undefined}
      step={number ? 'any' : undefined}
      maxLength={number ? undefined : 500}
      value={draft}
      placeholder="None"
      onChange={(event) => setDraft(event.target.value)}
      onBlur={commit}
      onKeyDown={onKeyDown}
    />
  )
}

/**
 * Edits one defined field's value. A value that does not fit the field shows
 * as empty with its raw text in a hint; saving replaces it.
 */
export function CustomFieldEditor({
  definition,
  value,
  editable,
  onSave,
}: {
  definition: CustomFieldDefinition
  value: unknown
  editable: boolean
  onSave: (value: unknown) => void
}) {
  const present = value !== null && value !== undefined && value !== ''
  const fits = present && customFieldValueFits(definition, value)
  const display = fits ? formatCustomFieldValue(definition, value) : null
  const mismatch = present && !fits ? `Stored value “${rawCustomFieldValue(value)}” doesn’t fit this field` : undefined

  if (!editable) {
    return (
      <span className={cn('block truncate px-1.5 py-0.5 text-xs', display ? 'text-foreground' : 'text-muted-foreground')} title={mismatch ?? display ?? undefined}>
        {display ?? (mismatch ? rawCustomFieldValue(value) : 'None')}
      </span>
    )
  }

  switch (definition.field_type) {
    case 'text':
    case 'long_text':
    case 'number': {
      const initial = fits ? (definition.field_type === 'number' ? String(Number(value)) : String(value)) : ''
      return (
        <div title={mismatch}>
          <CommitInput definition={definition} initial={initial} onSave={onSave} />
        </div>
      )
    }
    case 'date':
      return (
        <div title={mismatch}>
          <DatePicker
            value={fits ? String(value).slice(0, 10) : ''}
            label={definition.display_label}
            onChange={(next) => onSave(next || null)}
            trigger={
              <button type="button" aria-label={definition.display_label} className={triggerClassName}>
                <span className={cn('truncate', !display && 'text-muted-foreground')}>{display ?? 'None'}</span>
              </button>
            }
          />
        </div>
      )
    case 'boolean':
    case 'select': {
      const options = definition.field_type === 'boolean'
        ? [{ value: 'true', label: 'Yes' }, { value: 'false', label: 'No' }]
        : definition.options
      const current = fits ? String(value) : NONE
      return (
        <QuietDropdown
          label={definition.display_label}
          selected={[current]}
          options={[{ value: NONE, label: 'None' }, ...options]}
          onSelect={(next) => {
            if (next === current) return
            if (next === NONE) onSave(null)
            else onSave(definition.field_type === 'boolean' ? next === 'true' : next)
          }}
          trigger={
            <button type="button" aria-label={definition.display_label} className={triggerClassName} title={mismatch}>
              <span className={cn('truncate', !display && 'text-muted-foreground')}>{display ?? 'None'}</span>
            </button>
          }
        />
      )
    }
    case 'multi_select': {
      const selected = fits ? (value as string[]) : []
      return (
        <QuietDropdown
          label={definition.display_label}
          multiple
          selected={selected}
          options={definition.options}
          onSelect={(option) => {
            const next = selected.includes(option) ? selected.filter((item) => item !== option) : [...selected, option]
            onSave(next.length > 0 ? next : null)
          }}
          trigger={
            <button type="button" aria-label={definition.display_label} className={triggerClassName} title={mismatch ?? display ?? undefined}>
              <span className={cn('truncate', !display && 'text-muted-foreground')}>{display ?? 'None'}</span>
            </button>
          }
        />
      )
    }
  }
}
