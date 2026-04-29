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
