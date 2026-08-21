import { buildTriageBanner } from '../triage-banner'
import type { SupportConversation, SupportInboxScopeListResponse } from '@helpin-ai/support-core'

const conversation: SupportConversation = {
  id: 'conv-1', workspace_id: 'ws-1', display_id: 1, subject: 'Refund', status: 'open', priority: 'medium', source: 'widget',
  triage: {
    id: 'triage-1', workspace_id: 'ws-1', conversation_id: 'conv-1', status: 'suggested',
    classifier_source: 'ai', suggested_mailbox_id: 'billing', confidence: 0.876, reason: ' Billing request ',
    auto_moved: false, created_at: '2026-08-20T00:00:00Z', updated_at: '2026-08-20T00:00:00Z',
  },
  created_at: '2026-08-20T00:00:00Z', updated_at: '2026-08-20T00:00:00Z',
}
const scopes = {
  shared_inbox: { id: 'shared', name: 'Main inbox' },
  mailboxes: [{ id: 'billing', name: 'Billing' }],
} as SupportInboxScopeListResponse

test('shows an actionable suggestion with source, confidence, and reason', () => {
  expect(buildTriageBanner(conversation, scopes)).toEqual({
    mailboxId: 'billing', mailboxName: 'Billing', source: 'AI triage', confidence: '88%', reason: 'Billing request',
  })
})

test('hides locked, dismissed, and already-current suggestions', () => {
  expect(buildTriageBanner({ ...conversation, mailbox_id: 'billing' }, scopes)).toBeNull()
  expect(buildTriageBanner({ ...conversation, triage: { ...conversation.triage!, locked_at: '2026-08-20T01:00:00Z' } }, scopes)).toBeNull()
  expect(buildTriageBanner({ ...conversation, triage: { ...conversation.triage!, status: 'dismissed' } }, scopes)).toBeNull()
})
