import type { SupportConversationActions } from './pmTypes'

export const MACRO_SNOOZE_OPTIONS: { minutes: number; label: string }[] = [
  { minutes: 60, label: '1 hour' },
  { minutes: 4 * 60, label: '4 hours' },
  { minutes: 24 * 60, label: '1 day' },
  { minutes: 3 * 24 * 60, label: '3 days' },
  { minutes: 7 * 24 * 60, label: '1 week' },
]

const STATUS_LABELS: Record<NonNullable<SupportConversationActions['status']>, string> = {
  open: 'reopen',
  waiting_on_customer: 'mark waiting on customer',
  resolved: 'resolve',
}

export function hasMacroActions(actions: SupportConversationActions | null | undefined): actions is SupportConversationActions {
  if (!actions) return false
  return Boolean(
    actions.status || actions.priority || actions.assign_user_id !== undefined || actions.mailbox_id ||
    actions.add_tag_ids?.length || actions.remove_tag_ids?.length || actions.snooze_minutes,
  )
}

function snoozeLabel(minutes: number): string {
  return MACRO_SNOOZE_OPTIONS.find((option) => option.minutes === minutes)?.label ?? `${minutes} minutes`
}

/**
 * Plain-language list of what a macro does, in the order the server applies
 * the actions. Unknown IDs fall back to generic wording.
 */
export function describeMacroActions(
  actions: SupportConversationActions | null | undefined,
  names: { tags?: Map<string, string>; members?: Map<string, string>; inboxes?: Map<string, string> } = {},
): string[] {
  if (!hasMacroActions(actions)) return []
  const parts: string[] = []
  if (actions.mailbox_id) {
    parts.push(`move to ${actions.mailbox_id === 'shared' ? 'Main inbox' : names.inboxes?.get(actions.mailbox_id) ?? 'another inbox'}`)
  }
  if (actions.assign_user_id !== undefined) {
    parts.push(actions.assign_user_id === '' ? 'unassign' : `assign to ${names.members?.get(actions.assign_user_id) ?? 'a teammate'}`)
  }
  if (actions.priority) parts.push(`set priority ${actions.priority}`)
  for (const id of actions.add_tag_ids ?? []) parts.push(`tag ${names.tags?.get(id) ?? 'a tag'}`)
  for (const id of actions.remove_tag_ids ?? []) parts.push(`remove tag ${names.tags?.get(id) ?? 'a tag'}`)
  if (actions.status) parts.push(STATUS_LABELS[actions.status])
  if (actions.snooze_minutes) parts.push(`snooze for ${snoozeLabel(actions.snooze_minutes)}`)
  return parts
}

/** Drops unset fields so an empty macro serializes as {}. */
export function compactMacroActions(actions: SupportConversationActions): SupportConversationActions {
  const result: SupportConversationActions = {}
  if (actions.status) result.status = actions.status
  if (actions.priority) result.priority = actions.priority
  if (actions.assign_user_id !== undefined) result.assign_user_id = actions.assign_user_id
  if (actions.mailbox_id) result.mailbox_id = actions.mailbox_id
  if (actions.add_tag_ids?.length) result.add_tag_ids = actions.add_tag_ids
  if (actions.remove_tag_ids?.length) result.remove_tag_ids = actions.remove_tag_ids
  if (actions.snooze_minutes) result.snooze_minutes = actions.snooze_minutes
  return result
}

/** Reduces Markdown or editor text to comparable words. */
function plainMacroText(text: string): string {
  return text
    .replace(/!\[([^\]]*)\]\([^)]*\)/g, '$1') // images keep their alt text
    .replace(/\[([^\]]*)\]\([^)]*\)/g, '$1') // links keep their text
    .replace(/^\s*(?:[-*+]|\d+[.)])\s+/gm, '') // list markers
    .replace(/^\s*(?:#{1,6}|>)\s*/gm, '') // headings and quotes
    .replace(/[*_`~|\\]/g, ' ')
    .replace(/\s+/g, ' ')
    .trim()
    .toLowerCase()
}

/** A short piece of a macro's inserted text that identifies it in the editor. */
export function macroTextMarker(insertedText: string): string {
  return plainMacroText(insertedText).slice(0, 40)
}

/** Whether the editor still holds the text a macro inserted. */
export function macroTextPresent(editorText: string, marker: string): boolean {
  if (!marker) return false
  return plainMacroText(editorText).includes(marker)
}
