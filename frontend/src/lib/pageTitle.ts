const PUBLIC_ROUTE_TITLES: Record<string, string> = {
  '/forgot-password': 'Forgot Password',
  '/login': 'Sign In',
  '/onboarding': 'Onboarding',
  '/register': 'Sign Up',
  '/reset-password': 'Reset Password',
  '/verify-email': 'Verify Email',
  '/workspaces': 'Workspaces',
}

const WORKSPACE_ROUTE_TITLES: Record<string, string> = {
  dashboard: 'Dashboard',
  docs: 'Documentation',
  'docs/drafts': 'Drafts',
  'docs/my': 'My Documents',
  'docs/recent': 'Recent Documents',
  notifications: 'Notifications',
  'pm/agent-runs': 'Agent Runs',
  'pm/agents': 'Agents',
  'pm/epics': 'Epics',
  'pm/labels': 'Labels',
  'pm/my-work': 'My Work',
  'pm/objectives': 'Objectives',
  'pm/reports': 'Reports',
  'pm/roadmap': 'Roadmap',
  'pm/sprints': 'Sprints',
  'pm/support': 'Support',
  'pm/tasks': 'Tasks',
  'pm/tool-catalog': 'Tool Catalog',
  setup: 'Setup',
  sprints: 'Sprints',
  support: 'Support',
  'support/coverage': 'Support Coverage',
  'support/search': 'Support Search',
  tasks: 'Tasks',
  'team-goals': 'Team Goals',
  'automation/activity': 'Automation Activity',
  'automation/agents': 'Agents',
  'automation/flows': 'Automation Flows',
  'automation/runs': 'Agent Runs',
  'automation/skills': 'Skill Catalog',
  'automation/tools': 'Tool Catalog',
  'automation/triggers': 'Automation Trigger Catalog',
  'crm/companies': 'Companies',
  'crm/contacts': 'Contacts',
  'crm/deals': 'Deals',
  'crm/meetings': 'Meetings',
  'crm/insights': 'CRM Insights',
  'crm/review': 'CRM Review',
}

const SETTINGS_TITLES: Record<string, string> = {
  account: 'Organization Settings',
  access: 'Access Settings',
  automations: 'Automations Settings',
  billing: 'Billing Settings',
  'chat-general': 'Chat Widget Settings',
  'crm-autonomy': 'CRM Autonomy Settings',
  'crm-email': 'Email Accounts Settings',
  'crm-pipelines': 'Pipelines Settings',
  delivery: 'Delivery Settings',
  'external-mcp': 'External MCP Settings',
  general: 'General Settings',
  'git-connections': 'Git Connections Settings',
  helpcenter: 'Help Center Settings',
  import: 'Import & Export Settings',
  'inboxes-routing': 'Inboxes & Routing Settings',
  knowledge: 'Knowledge Settings',
  labels: 'Labels Settings',
  mcp: 'MCP Settings',
  members: 'Members Settings',
  notifications: 'Notifications',
  profile: 'Profile',
  'recurring-tasks': 'Recurring Tasks Settings',
  redirects: 'Redirects Settings',
  repositories: 'Repositories Settings',
  security: 'Security',
  'support-ai-assistant': 'Support AI Assistant Settings',
  'support-translation': 'Support Translation Settings',
  'task-templates': 'Task Templates Settings',
  teams: 'Teams Settings',
  workflows: 'Workflows Settings',
}

/** Returns a route-specific fallback while a page is loading. */
export function getPageTitle(pathname: string): string {
  const normalizedPath = pathname.replace(/\/+$/, '') || '/'
  const publicTitle = PUBLIC_ROUTE_TITLES[normalizedPath]
  if (publicTitle) return publicTitle

  if (normalizedPath === '/oauth/authorize') return 'Authorize AI Client'
  if (normalizedPath.startsWith('/join/')) return 'Join Workspace'
  if (normalizedPath.startsWith('/share/')) return 'Shared Document'
  if (normalizedPath.startsWith('/shared/')) return 'Shared conversation'

  const workspaceMatch = normalizedPath.match(/^\/w\/[^/]+\/?(.*)$/)
  if (!workspaceMatch) return 'Helpin'

  const workspacePath = workspaceMatch[1]
  if (workspacePath.startsWith('settings/')) {
    return SETTINGS_TITLES[workspacePath.slice('settings/'.length)] ?? 'Settings'
  }

  const routeTitle = WORKSPACE_ROUTE_TITLES[workspacePath]
  if (routeTitle) return routeTitle

  if (/^crm\/companies\/[^/]+$/.test(workspacePath)) return 'Company'
  if (/^crm\/contacts\/[^/]+$/.test(workspacePath)) return 'Contact'
  if (/^crm\/deals\/[^/]+$/.test(workspacePath)) return 'Deal'
  if (/^docs\/documents\/[^/]+$/.test(workspacePath)) return 'Document'
  if (/^docs\/spaces\/[^/]+$/.test(workspacePath)) return 'Space'
  if (/^pm\/coding-sessions\/[^/]+$/.test(workspacePath)) return 'Agent Session'
  if (/^pm\/epics\/[^/]+$/.test(workspacePath)) return 'Epic'
  if (/^pm\/objectives\/[^/]+$/.test(workspacePath)) return 'Objective'
  if (/^pm\/sprints\/[^/]+$/.test(workspacePath)) return 'Sprint'
  if (/^pm\/tasks\/[^/]+$/.test(workspacePath)) return 'Task'
  if (/^sprints\/[^/]+$/.test(workspacePath)) return 'Sprint'

  return 'Workspace'
}
