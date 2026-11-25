// Fictional records shared by all six captures.
import { WORKSPACE_ID as ws, ORGANIZATION_ID, CONVERSATION_ID, buildConversation, SUPPORT_INSTALLATION } from '../../e2e/support/fixtures/supportE2E.ts'

export const now = '2026-11-20T10:25:00Z'
const time = (hourMinute) => '2026-11-20T' + hourMinute + ':00Z'
const timestamps = { created_at: now, updated_at: now }
export const workspace = { ...timestamps, id: ws, name: 'Northstar', slug: 'northstar', organization_id: ORGANIZATION_ID, owner_id: 'sam', timezone: 'UTC' }
const org = { ...timestamps, id: ORGANIZATION_ID, name: 'Northstar', slug: 'northstar', owner_id: 'sam', role: 'owner' }
const people = ['Sam Ellis', 'Ana Bell'].map((name, i) => ({
  ...timestamps, id: i ? 'ana' : 'sam', full_name: name, email: name.split(' ')[0].toLowerCase() + '@northstar.example',
}))
const members = people.map((person, i) => ({
  ...person, id: 'member-' + person.id, user_id: person.id, display_name: person.full_name, role: i ? 'member' : 'owner', status: 'active', workspace_id: ws,
}))
const team = { id: 'payments', workspace_id: ws, name: 'Payments', key: 'PAY', handle: 'payments', team_type: 'engineering', color: '#4674c5' }
const epic = { id: 'checkout', team_id: team.id, name: 'Checkout reliability', display_id: 18, workspace_id: ws, ...timestamps }
const mailbox = { id: 'support', name: 'Customer support', handle: 'support', icon: 'inbox', is_shared: true, is_default: false, active: true, unread_count: 3, total_count: 8 }
const billingMailbox = { ...mailbox, id: 'billing', name: 'Billing', handle: 'billing' }
const routes = [
  { channel_id: 'C3', channel_name: 'support-desk', purpose: 'inbox', mailbox_ids: [mailbox.id, billingMailbox.id] },
  { channel_id: 'C1', crm_company_id: 'acme', channel_name: 'acme-support', purpose: 'support', mailbox_id: mailbox.id, team_id: null, epic_id: null },
  { channel_id: 'C5', channel_name: 'partner-requests', purpose: 'mixed', mailbox_id: null, team_id: null, epic_id: null },
  { channel_id: 'C2', channel_name: 'payments', purpose: 'pm', mailbox_id: null, team_id: team.id, epic_id: epic.id },
]
export const taskId = '6ec87beb-8360-4101-a805-19077bbcf142'
const task = {
  ...timestamps, id: taskId, task_key: 'PAY-142', display_id: 142, name: 'Fix checkout error for discount codes', workspace_id: ws,
  description: 'Checkout fails when a percentage discount is applied. Acme reproduced the error with the FALL20 code on two separate orders.',
  team_id: team.id, team_name: team.name, epic_id: epic.id, epic_name: epic.name, workflow_id: 'payments-workflow', workflow_state_id: 'review', state_name: 'In review', state_type: 'started',
  task_type: 'bug', severity: 'major', priority: 'high', owner_member_ids: [members[1].id], requester_member_id: members[0].id,
  started: true, completed: false, blocked: false, archived: false, position: 1,
}
const state = { id: task.workflow_state_id, workflow_id: task.workflow_id, name: 'In review', state_type: 'started', color: '#8b5cf6', position: 3 }
const workflow = { workflow: { id: task.workflow_id, name: 'Payments workflow', team_id: team.id, workspace_id: ws }, states: [state] }
const detail = { task, state, owners: [people[1]], followers: [], requester_member: members[0], labels: [], epic_name: epic.name }
const comments = [{
  author: people[1], reply_count: 0, reactions: [], replies: [],
  comment: { id: 'comment-1', entity_type: 'task', entity_id: task.id, author_id: people[1].id, body: 'The discount calculation is fixed. The regression check passes with FALL20. Ready for review.', created_at: time('10:18'), updated_at: time('10:18'), slack_discussion_channel: 'payments', slack_permalink: 'https://northstar-example.slack.com/archives/C2/p1795169880000000' },
}]
const thread = [
  ['10:02', 'Lena Park', 'We’re seeing an error at checkout when a customer applies FALL20. The same order goes through without the code.'],
  ['10:05', 'Sam Ellis', 'Thanks, Lena. Is this happening on every discounted order, or only with this code?'],
  ['10:08', 'Lena Park', 'Two orders so far, both with FALL20. I can reproduce it on a new cart too.'],
  ['10:12', 'Sam Ellis', 'Thanks for checking. I’ve shared the example with our payments team and I’ll update you here.'],
]
const messages = thread.map(([at, name, content], i) => ({
  id: 'message-' + i, workspace_id: ws, conversation_id: CONVERSATION_ID, sender_type: i % 2 ? 'user' : 'customer',
  sender_display_name: name, sender_user_id: i % 2 ? 'sam' : undefined, content, is_internal: false,
  message_type: 'reply', via_channel: 'slack', channel_delivery: i % 2 ? { provider: 'slack', status: 'sent', retryable: false } : null,
  created_at: time(at), updated_at: time(at),
}))
const slackOrigin = JSON.stringify({ via: 'slack', slack_permalink: 'https://northstar-example.slack.com/archives/C3/p1795169880000000' })
messages[3].metadata = slackOrigin
messages.push({ ...messages[3], id: 'internal-note', content: 'Payments reproduced this on a fresh cart. Ana is checking the discount rule.', is_internal: true, channel_delivery: null, message_type: 'note', created_at: time('10:13') })
messages.push({ ...messages[1], id: 'slack-assignment', content: 'Sam assigned this conversation to Ana.', is_internal: true, message_type: 'system', system_event_type: 'assigned', metadata: JSON.stringify({ via: 'slack', slack_permalink: 'https://northstar-example.slack.com/archives/C2/p1795169880000000' }), created_at: time('10:14') })
const rows = [
  ['Lena Park', 'Checkout returns an error', 'slack', thread[3][2], '10:12'],
  ['Owen Mills', 'October invoice copy', 'email', 'Could you send a copy to our billing address?', '10:07'],
  ['Mia Chen', 'Adding a teammate', 'widget', 'That worked. Thank you!', '09:54'],
  ['Priya Shah', 'Webhook delivery retries', 'slack', 'The delivery log still shows two attempts.', '09:46'],
  ['Jules Rivera', 'Update our billing contact', 'email', 'Our finance team has a new email address.', '09:30'],
  ['Noah Kim', 'Workspace invitation expired', 'widget', 'Can you send me a fresh invitation?', '09:08'],
  ['Eva Stone', 'Export last month’s payments', 'email', 'Is it possible to include the currency column?', '08:51'],
  ['Ben Morris', 'Payout timing', 'slack', 'We received the transfer this morning. Thanks!', '08:42'],
]
const conversations = rows.map(([name, subject, source, preview, at], i) => ({
  ...buildConversation([0, 1, 0, 1, 0, 1, 0, 0][i]), id: i ? 'conversation-' + i : CONVERSATION_ID,
  display_id: 1042 - i, customer_name: name, customer_email: name.split(' ')[0].toLowerCase() + (i ? '@customer.example' : '@acme.example'),
  anonymous_id: source === 'widget' ? 'visitor-' + i : null,
  provider_destinations: source === 'slack' ? [{ provider: source, id: i ? 'conversation-' + i : CONVERSATION_ID, display_id: 1042 - i, name: i ? '#partner-support' : '#acme-support' }] : undefined,
  subject, source, channel: source, flow_state: i % 3 === 0 ? 'assigned_to_human' : 'queued_for_human', human_takeover: true, assigned_user_id: i === 0 ? 'ana' : i % 3 === 0 ? 'sam' : undefined, priority: i ? 'medium' : 'high',
  mailbox_id: mailbox.id, mailbox_name: mailbox.name, last_message: preview, list_last_message_preview: preview,
  list_last_message_at: time(at), list_last_activity_at: time(at), created_at: time(i ? '08:30' : '10:02'), updated_at: time(at),
  last_message_sender_type: i ? 'customer' : 'user', last_message_sender_display_name: i ? name : 'Sam Ellis',
  linked_task_id: i ? undefined : task.id, provider_channel_name: source === 'slack' ? (i ? 'partner-support' : 'acme-support') : undefined,
}))
const paged = data => ({ data, total: data.length, page: 1, per_page: 50, total_pages: 1 })
const counts = { inbox: 3, mine: 1, waiting: 0, ai_active: 0, total: 3, my_inbox: 1, unassigned: 2 }
const sharedInbox = { ...mailbox, id: 'shared', name: 'All conversations', is_default: true }
const inbox = '/api/support/inbox'
const conversation = inbox + '/conversations/' + CONVERSATION_ID
const workspacePath = '/api/workspaces/' + ws
const taskPath = '/api/pm/tasks/' + task.id
const permissions = ['workspace.read', 'workspace.members.read', 'settings.read', 'settings.manage', 'team.manage', 'pm.read', 'pm.edit', 'crm.read', 'support.read', 'support.edit', 'support.admin', 'support.merge', 'docs.read', 'ws.connect']
const fixtures = new Map(Object.entries({
  '/api/notifications': { data: [], unread_count: 0 }, '/api/notifications/unread-count': { count: 0 }, '/api/automation/runs/attention-count': { count: 0 },
  '/api/auth/config': { signup_enabled: true, app_email_configured: true },
  '/api/auth/me': { ...people[0], default_workspace_id: ws },
  '/api/workspaces': [workspace], '/api/organizations': [org], '/api/workspaces/by-slug/northstar': workspace,
  [workspacePath + '/my-membership']: members[0], [workspacePath + '/members']: members, [workspacePath + '/assignable-members']: members,
  [workspacePath + '/me']: { workspace_id: ws, modules: ['support', 'pm', 'docs', 'crm'], membership: members[0], permissions, team_memberships: [{ team_id: team.id }] },
  '/api/settings': { settings: { id: 'settings', workspace_id: ws, notifications_enabled: true }, teams: [team], people: [], memberships: [], user_memberships: [], managers: [], job_role_criteria: [], invitation_team_preassignments: [], team_estimate_settings: [], team_field_visibility: [], team_repo_defaults: [] },
  '/api/slack/': { configured: true, connected: true, ai_enabled: true, workspace_name: workspace.name, routes },
  '/api/crm/companies/acme': { id: 'acme', name: 'Acme', domain: 'acme.example' },
  '/api/slack/channels': routes.map(r => ({ id: r.channel_id, name: r.channel_name, is_ext_shared: ['support', 'mixed'].includes(r.purpose), is_private: true, is_member: true })).concat([{ id: 'C4', name: 'customer-support', is_private: false, is_member: false, is_ext_shared: false }]),
  '/api/support/channels': [{ provider: 'telegram', id: 'telegram-1', workspace_id: ws, handle: 'northstar_support_bot', display_name: 'Northstar Support', status: 'active', mailbox_id: mailbox.id, ai_replies_disabled: true, last_event_at: time('09:54') }],
  '/api/settings/module-access': { modules: [], grants: [] }, [inbox + '/mailboxes']: [mailbox, billingMailbox], [inbox + '/routing-usage']: { triage_enabled: false, exhausted: false },
  [inbox + '/attachments/policy']: { max_bytes: 10485760, content_types: ['image/png', 'application/pdf'] },
  [inbox + '/installations']: { ...SUPPORT_INSTALLATION, settings: { ...SUPPORT_INSTALLATION.settings, email_fallback_enabled: true } }, [inbox + '/mailboxes/scopes']: { shared_inbox: sharedInbox, mailboxes: [mailbox, billingMailbox] },
  [inbox + '/unread-stats']: counts, [inbox + '/conversations']: { ...paged(conversations), meta: { unread: counts } },
  [inbox + '/messages/message-0/slack-source']: { permalink: 'https://northstar-example.slack.com/archives/CACME/p1795168920000001' },
  [inbox + '/messages/message-2/slack-source']: { permalink: 'https://northstar-example.slack.com/archives/CACME/p1795169280000002?thread_ts=1795168920.000001&cid=CACME' },
  [conversation + '/slack-thread']: { permalink: 'https://northstar-example.slack.com/archives/C3/p1795169880000000' },
  [conversation + '/visitor-context']: {
    device: null, location: null, contact: { id: 'lena', name: 'Lena Park', email: 'lena@acme.example', job_title: 'Operations lead', lifecycle_stage: 'customer', source: 'slack' },
    company: { id: 'acme', display_id: 'ACME', name: 'Acme', domain: 'acme.example', custom_properties: {}, updated_at: now }, company_context_status: 'ok',
    channels: ['slack'], other_conversations: [], total_conversations: 1, session_created_at: null,
  },
  [conversation + '/assignees']: members,
  [conversation + '/associations']: { tasks: [{ object_type: 'task', object_id: task.id, title: task.name, display_id: task.task_key, status: state.name, workflow_state_id: state.id }], crm_records: [], support_conversations: [], docs: [] },
  [conversation]: conversations[0], [conversation + '/messages']: messages, [conversation + '/message-pages']: { data: messages, has_more: false },
  [conversation + '/translation']: { available: false, preference: {}, conversation: {}, languages: {} },
  [conversation + '/ai-run/interactions']: { interactions: [] },
  '/api/pm/tasks/board': [{ state, tasks: [task], task_count: 1, point_total: 0, has_more: false }],
  [taskPath + '/associations']: { task_relationships: { blocked_by: [], blocking: [], relates_to: [], related_by: [], duplicates: [], duplicated_by: [] }, tasks: [], crm_records: [], docs: [], support_conversations: [{ object_type: 'support_conversation', object_id: CONVERSATION_ID, title: conversations[0].subject, display_id: '#1042' }] },
  [taskPath + '/triage']: { status: 'disabled' },
  [taskPath + '/checklist']: ['Reproduce checkout with FALL20', 'Verify the fix on a fresh cart'].map((text, position) => ({
    ...timestamps, id: 'check-' + position, task_id: task.id, text, position, completed: true,
  })),
  [taskPath]: detail, '/api/pm/comments': comments, '/api/pm/epics': [{ epic, owners: [], labels: [] }],
  '/api/pm/workflows': [workflow], '/api/pm/workflows/resolve': workflow, '/api/pm/tasks': paged([task]),
  [taskPath + '/updates']: { data: [{ id: 'priority-change', kind: 'change', occurred_at: time('10:19'), actor: people[0], activity: { id: 'activity', action: 'updated', field_name: 'priority', old_value: 'medium', new_value: 'high', metadata: { via: 'slack', slack_permalink: 'https://northstar-example.slack.com/archives/C2/p1795169880000000' } } }], high_water: '', read_initialized: true, unread_count: 0 },
}))

export function responseFor(path) {
  return fixtures.get(path) ?? []
}
