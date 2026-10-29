import { useId, useState, type KeyboardEvent } from 'react'
import { QuietPrimaryAction, QuietTextAction, QuietUnderlineInput } from '@/components/design-system/quiet'
import { Cancel01Icon } from '@/lib/icons'
import { CustomerPortalApiError, customerPortalService, type CustomerPortalCCOptions, type CustomerPortalRequestDetail } from '@/lib/services/customerPortalService'
import { PORTAL_CC_LIMIT, usePortalCCOptions } from './usePortalCCOptions'

const EMAIL_PATTERN = /^[^\s@]+@[^\s@]+\.[^\s@]+$/

/**
 * Edits a list of email addresses as removable chips. Enter, comma or
 * leaving the field adds what was typed; colleagues are suggested.
 */
export function PortalCCEditor({
  label,
  value,
  onChange,
  options,
  disabled = false,
}: {
  label: string
  value: string[]
  onChange: (value: string[]) => void
  options: CustomerPortalCCOptions | null
  disabled?: boolean
}) {
  const id = useId()
  const [draft, setDraft] = useState('')
  const [error, setError] = useState<string | null>(null)
  const suggestions = (options?.colleagues ?? []).filter((colleague) => !value.includes(colleague.email))

  const add = () => {
    const address = draft.trim().replace(/[,;]+$/, '').toLowerCase()
    if (!address) return
    if (!EMAIL_PATTERN.test(address)) {
      setError(`${address} is not an email address.`)
      return
    }
    if (value.length >= PORTAL_CC_LIMIT) {
      setError(`Copy at most ${PORTAL_CC_LIMIT} people.`)
      return
    }
    if (options?.mode === 'colleagues') {
      const domain = address.slice(address.lastIndexOf('@') + 1)
      if (!options.colleagues.some((colleague) => colleague.email === address) && !options.domains.includes(domain)) {
        setError(`${address} is not a colleague at your company.`)
        return
      }
    }
    setError(null)
    setDraft('')
    if (!value.includes(address)) onChange([...value, address])
  }

  const onKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key === 'Enter' || event.key === ',' || event.key === ';') {
      event.preventDefault()
      add()
    } else if (event.key === 'Backspace' && !draft && value.length > 0) {
      onChange(value.slice(0, -1))
    }
  }

  return (
    <div className="min-w-0 space-y-1">
      <label htmlFor={id} className="block text-sm font-medium text-quiet-text-secondary">{label}</label>
      {value.length > 0 ? (
        <ul className="flex flex-wrap gap-1.5 pt-1" aria-label={`${label}: ${value.length}`}>
          {value.map((address) => (
            <li key={address} className="inline-flex items-center gap-1 rounded-full border border-quiet-divider-strong py-0.5 pl-2.5 pr-1 text-[12.5px] text-quiet-text-secondary">
              {address}
              {disabled ? null : (
                <button type="button" aria-label={`Stop copying ${address}`} onClick={() => onChange(value.filter((item) => item !== address))} className="rounded-full p-0.5 text-quiet-text-tertiary hover:bg-quiet-row-hover hover:text-quiet-text-primary focus-visible:outline-2 focus-visible:outline-ring">
                  <Cancel01Icon className="size-3" />
                </button>
              )}
            </li>
          ))}
        </ul>
      ) : null}
      {disabled ? null : (
        <>
          <QuietUnderlineInput
            id={id}
            type="email"
            list={suggestions.length > 0 ? `${id}-colleagues` : undefined}
            value={draft}
            placeholder={options?.mode === 'colleagues' ? 'A colleague’s email address' : 'name@company.com'}
            onChange={(event) => { setDraft(event.target.value); setError(null) }}
            onKeyDown={onKeyDown}
            onBlur={add}
            aria-describedby={error ? `${id}-error` : undefined}
            aria-invalid={Boolean(error)}
          />
          {suggestions.length > 0 ? (
            <datalist id={`${id}-colleagues`}>
              {suggestions.map((colleague) => <option key={colleague.email} value={colleague.email}>{colleague.name || colleague.email}</option>)}
            </datalist>
          ) : null}
          {error ? <p id={`${id}-error`} role="alert" className="text-[12.5px] text-destructive">{error}</p> : (
            <p className="text-[12.5px] text-quiet-text-tertiary">They get our replies by email{options?.mode === 'colleagues' ? '; copy people at your company' : ''}.</p>
          )}
        </>
      )}
    </div>
  )
}

/**
 * Who a request copies. The requester can change the list when the
 * workspace lets requesters copy people.
 */
export function PortalRequestCC({
  slug,
  request,
  ccMode,
  onUpdated,
}: {
  slug: string
  request: CustomerPortalRequestDetail
  ccMode: CustomerPortalCCOptions['mode']
  onUpdated: (request: CustomerPortalRequestDetail) => void
}) {
  const cc = request.cc ?? []
  const [editing, setEditing] = useState<string[] | null>(null)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const options = usePortalCCOptions(slug, editing !== null)
  if (cc.length === 0 && !request.can_edit_cc) return null

  const save = async () => {
    if (!editing) return
    setSaving(true)
    setError(null)
    try {
      onUpdated(await customerPortalService.setCC(slug, request.reference, editing))
      setEditing(null)
    } catch (saveError) {
      setError(saveError instanceof CustomerPortalApiError && saveError.status < 500 ? saveError.message : 'Could not update who is copied. Please try again.')
    } finally {
      setSaving(false)
    }
  }

  if (editing) {
    return (
      <div className="mt-5 max-w-xl space-y-3">
        <PortalCCEditor label="Copied" value={editing} options={options ?? { mode: ccMode, colleagues: [], domains: [] }} onChange={setEditing} />
        {error ? <p role="alert" className="text-[12.5px] text-destructive">{error}</p> : null}
        <div className="flex items-center gap-4">
          <QuietPrimaryAction type="button" disabled={saving} onClick={() => void save()}>{saving ? 'Saving…' : 'Save'}</QuietPrimaryAction>
          <QuietTextAction type="button" disabled={saving} onClick={() => { setEditing(null); setError(null) }}>Cancel</QuietTextAction>
        </div>
      </div>
    )
  }
  return (
    <div className="mt-4 flex min-w-0 flex-wrap items-baseline gap-x-2 gap-y-1 text-[13px]">
      <span className="text-[12px] text-quiet-text-tertiary">Copied</span>
      <span className="min-w-0 break-words text-quiet-text-secondary">{cc.length ? cc.join(', ') : 'Nobody'}</span>
      {request.can_edit_cc ? <QuietTextAction type="button" onClick={() => setEditing(cc)}>{cc.length ? 'Change' : 'Copy colleagues'}</QuietTextAction> : null}
    </div>
  )
}
