import type { SupportConversation, SupportInboxScopeListResponse } from '@helpin-ai/support-core'

export function buildTriageBanner(
  conversation: SupportConversation | undefined,
  scopes: SupportInboxScopeListResponse | undefined,
) {
  const triage = conversation?.triage
  if (!conversation || !triage?.suggested_mailbox_id) return null
  const mailbox = [scopes?.shared_inbox, ...(scopes?.mailboxes ?? [])]
    .filter(Boolean)
    .find((item) => item!.id === triage.suggested_mailbox_id)
  if (!mailbox) return null
  const currentMailbox = conversation.mailbox_id ?? 'shared'
  if (triage.status !== 'suggested' || triage.locked_at || triage.suggested_mailbox_id === currentMailbox) return null
  return {
    mailboxId: triage.suggested_mailbox_id,
    mailboxName: mailbox.name,
    source: triage.classifier_source === 'rule' ? 'Routing rule' : 'AI triage',
    confidence: triage.confidence == null ? null : `${Math.round(triage.confidence * 100)}%`,
    reason: triage.reason?.trim(),
  }
}
