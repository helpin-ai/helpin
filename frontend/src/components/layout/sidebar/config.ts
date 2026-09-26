import {
  ChartColumnIcon,
  BotIcon,
  Briefcase01Icon,
  Building03Icon,
  Camera01Icon,
  CheckmarkCircle02Icon,
  Clock01Icon,
  DollarCircleIcon,
  File01Icon,
  InboxIcon,
  CheckListIcon,
  BulbIcon,
  Message01Icon,
  PauseIcon,
  ArrowReloadHorizontalIcon,
  Setting07Icon,
  Target01Icon,
  UserIcon,
  UserGroupIcon,
  type IconComponent,
} from '@/lib/icons';
import {
  RecordIcon,
  ClipboardIcon,
  FolderKanbanIcon,
  ChartGanttIcon,
  Layers01Icon,
  OctagonXIcon,
  Wrench01Icon,
  BookOpen01Icon,
} from '@/lib/icons';
import { buildSettingsHomePath } from '@/lib/settingsDiscovery';
import { buildSettingsRoutePath, getSettingsSidebarGroups } from '@/lib/settingsSections';
import { isWorkspaceSupportRoute } from '@/lib/workspaceRoutes';
import type { WorkspaceTeam } from '@/lib/types';
import type { NavGroup, RailId, RailItem } from './types';

export function deriveActiveRail(pathname: string): RailId {
  if (pathname.endsWith('/setup')) return 'setup';
  if (pathname.includes('/settings')) return 'settings';
  if (isWorkspaceSupportRoute(pathname)) return 'support';
  if (pathname.includes('/crm')) return 'crm';
  if (
    pathname.endsWith('/automation')
    || pathname.includes('/automation/')
  ) return 'automation';
  if (pathname.includes('/pm/') || pathname.endsWith('/pm')) return 'projects';
  if (pathname.includes('/docs')) return 'docs';
  return 'projects';
}

export const teamSubItems: { key: string; label: string; icon: IconComponent; path: string }[] = [
  { key: 'tasks', label: 'Tasks', icon: CheckListIcon, path: 'tasks' },
  { key: 'epics', label: 'Epics', icon: Layers01Icon, path: 'epics' },
  { key: 'sprints', label: 'Sprints', icon: ArrowReloadHorizontalIcon, path: 'sprints' },
];

export const projectCreateOptions = [
  { key: 'task' as const, label: 'Task', icon: CheckListIcon, pages: ['tasks'] },
  { key: 'epic' as const, label: 'Epic', icon: Layers01Icon, pages: ['epics'] },
  { key: 'sprint' as const, label: 'Sprint', icon: ArrowReloadHorizontalIcon, pages: ['sprints'] },
  { key: 'objective' as const, label: 'Objective', icon: Target01Icon, pages: ['objectives'] },
];

export function supportModuleUnreadCount(stats?: { total?: number | null }): number {
  return stats?.total ?? 0;
}

export function buildRailItems(
  wsSlug: string,
  totalSupportUnread: number,
  setupProgress?: number,
  crmDefaultLink = `/w/${wsSlug}/crm/overview`,
): RailItem[] {
  const items: RailItem[] = [
    { id: 'projects', label: 'Projects', icon: FolderKanbanIcon, defaultLink: `/w/${wsSlug}/pm/my-work` },
    { id: 'support', label: 'Support', icon: Message01Icon, defaultLink: `/w/${wsSlug}/support`, indicator: Boolean(totalSupportUnread) },
    { id: 'docs', label: 'Docs', icon: File01Icon, defaultLink: `/w/${wsSlug}/docs` },
    { id: 'crm', label: 'CRM', icon: Briefcase01Icon, defaultLink: crmDefaultLink },
    { id: 'automation', label: 'Automation', icon: BotIcon, defaultLink: `/w/${wsSlug}/automation/flows` },
    { id: 'settings', label: 'Settings', icon: Setting07Icon, defaultLink: buildSettingsHomePath(wsSlug) },
  ];
  if (setupProgress !== undefined) {
    items.push({ id: 'setup', label: 'Setup', icon: CheckmarkCircle02Icon, defaultLink: `/w/${wsSlug}/setup`, progressPercent: Math.max(0, Math.min(100, setupProgress)), separatorBefore: true });
  }
  return items;
}

export function buildSettingsTeamLink(wsSlug: string, teamId: string): string {
  return `${buildSettingsRoutePath(wsSlug, 'teams')}?team=${encodeURIComponent(teamId)}`;
}

export function buildPanelNavGroups(
  wsSlug: string,
  canManageSettings: boolean,
  permissionSet?: Set<string>,
  agentAttentionCount = 0,
  settingsTeams: readonly Pick<WorkspaceTeam, 'id' | 'name'>[] = [],
): Record<RailId, NavGroup[]> {
  return {
    projects: [
      {
        label: '',
        items: [
          { link: `/w/${wsSlug}/pm/my-work`, label: 'My Work', icon: ClipboardIcon },
          { link: `/w/${wsSlug}/pm/objectives`, label: 'Objectives', icon: Target01Icon },
          { link: `/w/${wsSlug}/pm/roadmap`, label: 'Roadmap', icon: ChartGanttIcon },
          { link: `/w/${wsSlug}/pm/reports`, label: 'Reports', icon: ChartColumnIcon },
        ],
      },
    ],
    crm: [
      {
        label: '',
        items: [
          { link: `/w/${wsSlug}/crm/overview`, label: 'Overview', icon: ChartColumnIcon },
          { link: `/w/${wsSlug}/crm/contacts`, label: 'Contacts', icon: UserGroupIcon },
          { link: `/w/${wsSlug}/crm/companies`, label: 'Companies', icon: Building03Icon },
          { link: `/w/${wsSlug}/crm/deals`, label: 'Deals', icon: DollarCircleIcon },
          { link: `/w/${wsSlug}/crm/emails`, label: 'Emails', icon: InboxIcon },
          { link: `/w/${wsSlug}/crm/meetings`, label: 'Meetings', icon: Camera01Icon },
          { link: `/w/${wsSlug}/crm/playbooks`, label: 'Playbooks', icon: BookOpen01Icon, separatorBefore: true },
          { link: `/w/${wsSlug}/crm/insights`, label: 'Signals', icon: BulbIcon },
        ],
      },
    ],
    support: [
      {
        label: '',
        items: [],
      },
    ],
    automation: [
      {
        label: '',
        items: [
          { link: `/w/${wsSlug}/automation/flows`, label: 'Flows', icon: ArrowReloadHorizontalIcon },
          { link: `/w/${wsSlug}/automation/agents`, label: 'Agents', icon: BotIcon },
          { link: `/w/${wsSlug}/automation/activity`, label: 'Activity', icon: Clock01Icon, badge: agentAttentionCount },
        ],
      },
      {
        label: 'Catalog',
        items: [
          { link: `/w/${wsSlug}/automation/triggers`, label: 'Triggers', icon: BotIcon },
          { link: `/w/${wsSlug}/automation/tools`, label: 'Tools', icon: Wrench01Icon },
          { link: `/w/${wsSlug}/automation/skills`, label: 'Skills', icon: BookOpen01Icon },
        ],
      },
    ],
    docs: [],
    settings: getSettingsSidebarGroups(canManageSettings, permissionSet).map((group) => ({
      label: group.label,
      icon: group.icon,
      items: group.sections.map((section) => ({
        link: buildSettingsRoutePath(wsSlug, section.id),
        label: section.label,
        icon: section.icon,
        ...(section.id === 'teams' && settingsTeams.length > 0
          ? {
              children: settingsTeams.map((team) => ({
                link: buildSettingsTeamLink(wsSlug, team.id),
                label: team.name,
              })),
            }
          : {}),
      })),
    })),
    setup: [],
  };
}

export const supportFilterItems = [
  { key: 'inbox' as const, label: 'Inbox', icon: InboxIcon },
  { key: 'mine' as const, label: 'Mine', icon: UserIcon },
  { key: 'waiting' as const, label: 'Waiting', icon: PauseIcon },
  { key: 'resolved' as const, label: 'Resolved', icon: CheckmarkCircle02Icon },
  { key: 'spam' as const, label: 'Spam', icon: OctagonXIcon },
];

export const supportStatusOptions: readonly { value: string; label: string; icon: IconComponent; color: string }[] = [
  { value: 'all', label: 'All statuses', icon: InboxIcon, color: 'text-muted-foreground' },
  { value: 'open', label: 'Open', icon: RecordIcon, color: 'text-blue-500' },
  { value: 'waiting_on_customer', label: 'Waiting on Customer', icon: PauseIcon, color: 'text-orange-500' },
  { value: 'resolved', label: 'Resolved', icon: CheckmarkCircle02Icon, color: 'text-emerald-500' },
  { value: 'spam', label: 'Spam', icon: OctagonXIcon, color: 'text-red-500' },
] as const;

export const supportAiItems = [
  { key: 'ai_active' as const, label: 'AI Handling', icon: BotIcon },
  { key: 'resolved_by_ai' as const, label: 'AI Resolved', icon: CheckmarkCircle02Icon },
];
