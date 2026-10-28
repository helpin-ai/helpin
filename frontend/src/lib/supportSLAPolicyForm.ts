import type { SupportSLAPolicy, SupportSLAPolicyInput } from './pmTypes'

export type SLATargetUnit = 'minutes' | 'hours' | 'days'

export interface SLATargetDraft {
  enabled: boolean
  amount: string
  unit: SLATargetUnit
}

export interface SLAPolicyDraft {
  name: string
  enabled: boolean
  priorities: string[]
  mailboxIds: string[]
  channels: string[]
  companyTierIds: string[]
  companies: { id: string; name: string }[]
  firstResponse: SLATargetDraft
  nextResponse: SLATargetDraft
  resolution: SLATargetDraft
  businessHours: boolean
  pauseWhileWaiting: boolean
  warnBefore: SLATargetDraft
  notifyUserIds: string[]
  notifyCSOwner: boolean
  assignUserId: string | null
}

const UNIT_MINUTES: Record<SLATargetUnit, number> = { minutes: 1, hours: 60, days: 24 * 60 }
export const MAX_SLA_TARGET_MINUTES = 90 * 24 * 60
export const MAX_SLA_POLICY_COMPANIES = 50
export const MAX_SLA_NOTIFY_USERS = 10

/** Splits minutes into the largest whole unit for editing. */
export function targetDraft(minutes: number | null | undefined, fallback: SLATargetDraft): SLATargetDraft {
  if (!minutes) return { ...fallback, enabled: false }
  if (minutes % UNIT_MINUTES.days === 0) return { enabled: true, amount: String(minutes / UNIT_MINUTES.days), unit: 'days' }
  if (minutes % UNIT_MINUTES.hours === 0) return { enabled: true, amount: String(minutes / UNIT_MINUTES.hours), unit: 'hours' }
  return { enabled: true, amount: String(minutes), unit: 'minutes' }
}

export function emptySLAPolicyDraft(): SLAPolicyDraft {
  return {
    name: '',
    enabled: true,
    priorities: [],
    mailboxIds: [],
    channels: [],
    companyTierIds: [],
    companies: [],
    firstResponse: { enabled: true, amount: '1', unit: 'hours' },
    nextResponse: { enabled: false, amount: '4', unit: 'hours' },
    resolution: { enabled: false, amount: '1', unit: 'days' },
    businessHours: false,
    pauseWhileWaiting: true,
    warnBefore: { enabled: false, amount: '15', unit: 'minutes' },
    notifyUserIds: [],
    notifyCSOwner: false,
    assignUserId: null,
  }
}

export function policyToDraft(policy: SupportSLAPolicy): SLAPolicyDraft {
  const empty = emptySLAPolicyDraft()
  return {
    name: policy.name,
    enabled: policy.enabled,
    priorities: [...policy.priorities],
    mailboxIds: [...policy.mailbox_ids],
    channels: [...policy.channels],
    companyTierIds: [...(policy.company_tier_ids ?? [])],
    companies: [...(policy.companies ?? [])],
    firstResponse: targetDraft(policy.first_response_minutes, empty.firstResponse),
    nextResponse: targetDraft(policy.next_response_minutes, empty.nextResponse),
    resolution: targetDraft(policy.resolution_minutes, empty.resolution),
    businessHours: policy.business_hours,
    pauseWhileWaiting: policy.pause_while_waiting,
    warnBefore: targetDraft(policy.warn_before_minutes, empty.warnBefore),
    notifyUserIds: [...(policy.notify_user_ids ?? [])],
    notifyCSOwner: Boolean(policy.notify_cs_owner),
    assignUserId: policy.assign_unassigned_user_id ?? null,
  }
}

function targetMinutes(draft: SLATargetDraft, label: string, errors: string[]): number | null {
  if (!draft.enabled) return null
  const amount = Number(draft.amount)
  if (!Number.isFinite(amount) || amount <= 0 || !Number.isInteger(amount)) {
    errors.push(`${label} needs a whole number greater than zero.`)
    return null
  }
  const minutes = amount * UNIT_MINUTES[draft.unit]
  if (minutes > MAX_SLA_TARGET_MINUTES) {
    errors.push(`${label} can be at most 90 days.`)
    return null
  }
  return minutes
}

/** Validates a draft and converts it to the API payload. */
export function draftToPolicyInput(draft: SLAPolicyDraft): { input: SupportSLAPolicyInput | null; errors: string[] } {
  const errors: string[] = []
  const name = draft.name.trim().replace(/\s+/g, ' ')
  if (!name) errors.push('Give the policy a name.')
  if (name.length > 120) errors.push('Keep the name under 120 characters.')
  if (draft.companies.length > MAX_SLA_POLICY_COMPANIES) errors.push(`Name at most ${MAX_SLA_POLICY_COMPANIES} companies.`)
  const first = targetMinutes(draft.firstResponse, 'First response', errors)
  const next = targetMinutes(draft.nextResponse, 'Next response', errors)
  const resolution = targetMinutes(draft.resolution, 'Resolution', errors)
  if (!draft.firstResponse.enabled && !draft.nextResponse.enabled && !draft.resolution.enabled) {
    errors.push('Turn on at least one target.')
  }
  const warn = targetMinutes(draft.warnBefore, 'The warning', errors)
  const shortest = Math.min(...[first, next, resolution].filter((minutes): minutes is number => minutes !== null))
  if (warn !== null && warn >= shortest) {
    errors.push(`Warn earlier than the shortest target (${formatSLATarget(shortest)}).`)
  }
  if (draft.notifyUserIds.length > MAX_SLA_NOTIFY_USERS) errors.push(`Notify at most ${MAX_SLA_NOTIFY_USERS} teammates.`)
  if (errors.length > 0) return { input: null, errors }
  return {
    input: {
      name,
      enabled: draft.enabled,
      priorities: draft.priorities,
      mailbox_ids: draft.mailboxIds,
      channels: draft.channels,
      company_tier_ids: draft.companyTierIds,
      company_ids: draft.companies.map((company) => company.id),
      first_response_minutes: first,
      next_response_minutes: next,
      resolution_minutes: resolution,
      business_hours: draft.businessHours,
      pause_while_waiting: draft.pauseWhileWaiting,
      warn_before_minutes: warn,
      notify_user_ids: draft.notifyUserIds,
      notify_cs_owner: draft.notifyCSOwner,
      // Assigning only happens on a warning.
      assign_unassigned_user_id: warn !== null ? draft.assignUserId : null,
    },
    errors,
  }
}

/** "1h", "30m", "2d" style summary for a target in minutes. */
export function formatSLATarget(minutes: number | null | undefined): string | null {
  if (!minutes) return null
  if (minutes % (24 * 60) === 0) return `${minutes / (24 * 60)}d`
  if (minutes % 60 === 0) return `${minutes / 60}h`
  if (minutes > 60) return `${Math.floor(minutes / 60)}h ${minutes % 60}m`
  return `${minutes}m`
}
