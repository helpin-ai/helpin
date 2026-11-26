import { SUPPORT_NOTIFICATION_CATEGORIES, WORKSPACE_NOTIFICATION_CATEGORIES, type NotificationCategory } from './notificationTypes'
import type { WorkspaceModule } from './types'

export type ModuleNotificationRow = NotificationCategory & { slackRow: string; slackOnly?: boolean }
type ModuleSection = { module: WorkspaceModule | 'integrations'; title: string; rows: ModuleNotificationRow[] }
const slackOnly = (slackRow: string, label: string, description: string): ModuleNotificationRow =>
  ({ key: slackRow, slackRow, label, description, slackOnly: true })

const agentAttention = WORKSPACE_NOTIFICATION_CATEGORIES.find(row => row.key === 'agent_attention')!
export const MODULE_NOTIFICATION_SECTIONS: ModuleSection[] = [
  { module: 'pm', title: 'Projects', rows: WORKSPACE_NOTIFICATION_CATEGORIES.filter(row => row !== agentAttention)
    .map(row => ({ ...row, slackRow: 'projects.' + row.key })) },
  { module: 'docs', title: 'Docs', rows: [
    { key: 'mentions', slackRow: 'docs.mentions', label: 'Mentions', description: 'Direct mentions in documents and comments. In-app, desktop and email mention settings are shared with Projects' },
    slackOnly('docs.updates', 'Document updates', 'Edits to documents you follow'),
  ] },
  { module: 'support', title: 'Support inbox', rows: [
    slackOnly('support.assignments', 'Assignments', 'Conversations assigned to you'),
    ...SUPPORT_NOTIFICATION_CATEGORIES.map(row => ({ ...row, slackRow: row.key === 'support_mentions' ? 'support.mentions' : 'support.replies' })),
  ] },
  { module: 'crm', title: 'CRM', rows: [
    slackOnly('crm.assignments', 'Record assignments', 'Contacts, companies, deals and CRM follow-up tasks assigned to you'),
    slackOnly('crm.mentions', 'Note mentions', 'When someone mentions you in a CRM note'),
    { key: 'crm_signals', slackRow: 'crm.signals', label: 'Signals ready for review', description: 'Signals routed to you or your team' },
    slackOnly('crm.updates', 'Deal stage changes', 'Stage changes on deals you own'),
  ] },
  { module: 'agents', title: 'Agents', rows: [
    { ...agentAttention, slackRow: 'agents.attention' },
    slackOnly('agents.activity', 'Run activity', 'Progress and outcomes of your agent runs'),
  ] },
  { module: 'automation', title: 'Automation', rows: [
    slackOnly('automation.attention', 'Workflow requests', 'Workflow runs that need your input or approval. Assigned project tasks use your Projects assignment setting'),
    slackOnly('automation.activity', 'Workflow activity', 'Run updates and failures for your workflows'),
  ] },
  { module: 'integrations', title: 'Integrations', rows: [
    { key: 'integrations', slackRow: 'integrations.connection', label: 'Connection issues', description: 'External tools that need you to reconnect', supportsEmail: false },
  ] },
]
