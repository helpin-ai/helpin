import { QuietUnderlineInput, QuietUnderlineTextarea } from '@/components/design-system/quiet'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/design-system/quiet-dropdown-select'
import type { CustomerPortalRequestField, CustomerPortalRequestType } from '@/lib/services/customerPortalService'
import type { PortalFieldValues } from '@/lib/portalRequestFields'
import { cn } from '@/lib/utils'

const NONE = '__none__'

/** Request type cards; the chosen one is pressed. */
export function PortalRequestTypePicker({
  types,
  value,
  onChange,
}: {
  types: CustomerPortalRequestType[]
  value: string
  onChange: (id: string) => void
}) {
  return (
    <fieldset className="space-y-2">
      <legend className="mb-2 text-sm font-medium text-quiet-text-secondary">What is your request about?</legend>
      <div className="grid gap-2 sm:grid-cols-2">
        {types.map((type) => {
          const selected = type.id === value
          return (
            <button
              key={type.id}
              type="button"
              aria-pressed={selected}
              onClick={() => onChange(type.id)}
              className={cn(
                'rounded-lg border px-3.5 py-3 text-left transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/50',
                selected ? 'border-quiet-text-primary bg-quiet-row-hover' : 'border-quiet-divider-strong hover:bg-quiet-row-hover',
              )}
            >
              <span className="block text-sm font-medium text-quiet-text-primary">{type.name}</span>
              {type.description ? <span className="mt-0.5 block text-[12.5px] leading-snug text-quiet-text-tertiary">{type.description}</span> : null}
            </button>
          )
        })}
      </div>
    </fieldset>
  )
}

function FieldInput({ id, field, value, onChange }: { id: string; field: CustomerPortalRequestField; value: string | string[] | undefined; onChange: (value: string | string[]) => void }) {
  const text = typeof value === 'string' ? value : ''
  switch (field.field_type) {
    case 'long_text':
      return <QuietUnderlineTextarea id={id} rows={3} className="max-h-none min-h-20" maxLength={10000} required={field.required} value={text} onChange={(event) => onChange(event.target.value)} />
    case 'number':
      return <QuietUnderlineInput id={id} inputMode="decimal" required={field.required} value={text} onChange={(event) => onChange(event.target.value)} />
    case 'date':
      return <QuietUnderlineInput id={id} type="date" required={field.required} value={text} onChange={(event) => onChange(event.target.value)} />
    case 'boolean':
    case 'select': {
      const options = field.field_type === 'boolean' ? [{ value: 'true', label: 'Yes' }, { value: 'false', label: 'No' }] : field.options
      return (
        <Select value={text || NONE} onValueChange={(next) => onChange(next === NONE ? '' : next)}>
          <SelectTrigger id={id} aria-label={field.label} variant="underline" className="w-full px-0.5"><SelectValue /></SelectTrigger>
          <SelectContent>
            <SelectItem value={NONE}>{field.required ? 'Choose one' : 'None'}</SelectItem>
            {options.map((option) => <SelectItem key={option.value} value={option.value}>{option.label}</SelectItem>)}
          </SelectContent>
        </Select>
      )
    }
    case 'multi_select': {
      const selected = Array.isArray(value) ? value : []
      return (
        <div id={id} className="flex flex-wrap gap-1.5 pt-1" role="group" aria-label={field.label}>
          {field.options.map((option) => {
            const on = selected.includes(option.value)
            return (
              <button
                key={option.value}
                type="button"
                aria-pressed={on}
                onClick={() => onChange(on ? selected.filter((item) => item !== option.value) : [...selected, option.value])}
                className={cn(
                  'rounded-full border px-2.5 py-1 text-[12.5px] transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/50',
                  on ? 'border-quiet-text-primary bg-quiet-text-primary text-background' : 'border-quiet-divider-strong text-quiet-text-secondary hover:bg-quiet-row-hover',
                )}
              >
                {option.label}
              </button>
            )
          })}
        </div>
      )
    }
    default:
      return <QuietUnderlineInput id={id} maxLength={500} required={field.required} value={text} onChange={(event) => onChange(event.target.value)} />
  }
}

/** The chosen request type's fields, in the order the team set. */
export function PortalRequestTypeFields({
  idPrefix,
  type,
  values,
  onChange,
}: {
  idPrefix: string
  type: CustomerPortalRequestType
  values: PortalFieldValues
  onChange: (values: PortalFieldValues) => void
}) {
  return (
    <>
      {type.fields.map((field) => {
        const id = `${idPrefix}-field-${field.key}`
        return (
          <div key={field.key} className="min-w-0 space-y-1">
            <label htmlFor={id} className="block text-sm font-medium text-quiet-text-secondary">
              {field.label}
              {field.required ? <span className="text-quiet-text-tertiary"> (required)</span> : null}
            </label>
            <FieldInput id={id} field={field} value={values[field.key]} onChange={(value) => onChange({ ...values, [field.key]: value })} />
            {field.help_text ? <p className="text-[12.5px] text-quiet-text-tertiary">{field.help_text}</p> : null}
          </div>
        )
      })}
    </>
  )
}
