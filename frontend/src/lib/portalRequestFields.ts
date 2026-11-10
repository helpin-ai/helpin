import type { CustomerPortalRequestType } from '@/lib/services/customerPortalService'

/** Field values while a customer writes; inputs keep text until sent. */
export type PortalFieldValues = Record<string, string | string[]>


/**
 * Converts form values for the API: numbers become numbers, yes/no becomes a
 * boolean, and empty values are left out. Returns the first problem when a
 * required field is empty or a value is not valid.
 */
export function portalFieldsForSubmit(type: CustomerPortalRequestType | undefined, values: PortalFieldValues): { fields: Record<string, unknown>; error: string | null } {
  const fields: Record<string, unknown> = {}
  if (!type) return { fields, error: null }
  for (const field of type.fields) {
    const raw = values[field.key]
    const empty = raw === undefined || (Array.isArray(raw) ? raw.length === 0 : raw.trim() === '')
    if (empty) {
      if (field.required) return { fields, error: `${field.label} is required.` }
      continue
    }
    switch (field.field_type) {
      case 'number': {
        const number = Number(String(raw).trim())
        if (!Number.isFinite(number)) return { fields, error: `${field.label} must be a number.` }
        fields[field.key] = number
        break
      }
      case 'boolean':
        fields[field.key] = raw === 'true'
        break
      case 'multi_select':
        fields[field.key] = raw
        break
      default:
        fields[field.key] = String(raw).trim()
    }
  }
  return { fields, error: null }
}
