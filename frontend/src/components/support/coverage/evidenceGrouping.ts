import type { SupportGapEvidence } from '@/lib/supportCoverageTypes'

export type EvidenceGroup =
  | {
      kind: 'conversation'
      conversationId: string
      items: SupportGapEvidence[]
    }
  | { kind: 'standalone'; item: SupportGapEvidence }

// Group evidence rows so that all rows sharing a conversation_id collapse
// into a single conversation card. Rows without conversation_id are kept
// as standalone groups in their original order.
//
// Insertion order is preserved by the order each conversation is first seen,
// matching the backend's evidence ordering (created_at DESC).
export function groupEvidenceByConversation(
  evidence: SupportGapEvidence[],
): EvidenceGroup[] {
  const groups: EvidenceGroup[] = []
  const byConversation = new Map<string, SupportGapEvidence[]>()
  for (const ev of evidence) {
    if (ev.conversation_id) {
      const existing = byConversation.get(ev.conversation_id)
      if (existing) {
        existing.push(ev)
      } else {
        const items = [ev]
        byConversation.set(ev.conversation_id, items)
        groups.push({ kind: 'conversation', conversationId: ev.conversation_id, items })
      }
    } else {
      groups.push({ kind: 'standalone', item: ev })
    }
  }
  return groups
}

const OURS = new Set(['user', 'agent', 'ai'])

// Returns true when sender_role represents a workspace-side reply (teammate,
// agent, or AI). Customer messages and unknown/empty roles fall to the left
// (customer side) by default.
export function isOurSenderRole(role: string): boolean {
  return OURS.has(role)
}

// Conversation-scoped evidence is one of two kinds:
// - "message" rows have a message_id and represent an actual chat message
// - "event" rows have only a conversation_id (e.g. agent flagging a docs
//   issue, or an analysis snapshot scoped to the conversation)
// We render messages as bubbles and events as compact rows, so the count
// and header label distinguish between them.
export function isMessageEvidence(ev: SupportGapEvidence): boolean {
  return Boolean(ev.message_id)
}

const MIXED_LABEL = 'Conversation evidence'

// conversationCardHeaderLabel falls back to a generic label when the group
// contains both messages and non-message events, since the parent's preferred
// label only describes the first item.
export function conversationCardHeaderLabel(
  items: SupportGapEvidence[],
  defaultLabel: string,
): string {
  let hasMessage = false
  let hasEvent = false
  for (const ev of items) {
    if (isMessageEvidence(ev)) hasMessage = true
    else hasEvent = true
    if (hasMessage && hasEvent) return MIXED_LABEL
  }
  return defaultLabel
}

// conversationCardCountText returns a short summary like "3 messages" or
// "2 events", but only when there's more than one item of that kind.
// Returns an empty string when no qualifying count applies.
export function conversationCardCountText(items: SupportGapEvidence[]): string {
  let messages = 0
  let events = 0
  for (const ev of items) {
    if (isMessageEvidence(ev)) messages++
    else events++
  }
  if (messages > 1) return `${messages} messages`
  if (messages === 0 && events > 1) return `${events} events`
  return ''
}
