import {
  ChartColumnIcon,
  BotIcon,
  Briefcase01Icon,
  Building03Icon,
  CheckmarkCircle02Icon,
  Clock01Icon,
  DollarCircleIcon,
  File01Icon,
  InboxIcon,
  LayoutTable01Icon,
  BulbIcon,
  Mail01Icon,
  Message01Icon,
  PauseIcon,
  ArrowReloadHorizontalIcon,
  Setting07Icon,
  KanbanIcon,
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
  UserRemove01Icon,
  Wrench01Icon,
  BookOpen01Icon,
} from '@/lib/icons';
import { buildSettingsRoutePath, getSettingsSidebarGroups } from '@/lib/settingsSections';
import type { NavGroup, RailId, RailItem } from './types';

export function deriveActiveRail(pathname: string): RailId {
  if (pathname.includes('/settings')) return 'settings';
  if (pathname.includes('/support')) return 'support';
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
  { key: 'tasks', label: 'Tasks', icon: LayoutTable01Icon, path: 'tasks' },
  { key: 'epics', label: 'Epics', icon: Layers01Icon, path: 'epics' },
  { key: 'sprints', label: 'Sprints', icon: ArrowReloadHorizontalIcon, path: 'sprints' },
];

export const projectCreateOptions = [
  { key: 'task' as const, label: 'Task', icon: KanbanIcon, pages: ['tasks'] },
  { key: 'epic' as const, label: 'Epic', icon: Layers01Icon, pages: ['epics'] },
  { key: 'sprint' as const, label: 'Sprint', icon: ArrowReloadHorizontalIcon, pages: ['sprints'] },
  { key: 'objective' as const, label: 'Objective', icon: Target01Icon, pages: ['objectives'] },
];

export function buildRailItems(wsSlug: string, totalSupportUnread: number, agentAttentionCount: number): RailItem[] {
  return [
    { id: 'projects', label: 'Projects', icon: FolderKanbanIcon, defaultLink: `/w/${wsSlug}/pm/my-work` },
    { id: 'crm', label: 'CRM', icon: Briefcase01Icon, defaultLink: `/w/${wsSlug}/crm/contacts` },
    { id: 'support', label: 'Support', icon: Message01Icon, defaultLink: `/w/${wsSlug}/support`, indicator: Boolean(totalSupportUnread) },
    { id: 'automation', label: 'Automation', icon: BotIcon, defaultLink: `/w/${wsSlug}/automation/flows`, indicator: Boolean(agentAttentionCount) },
    { id: 'docs', label: 'Docs', icon: File01Icon, defaultLink: `/w/${wsSlug}/docs` },
    { id: 'settings', label: 'Settings', icon: Setting07Icon, defaultLink: buildSettingsRoutePath(wsSlug, 'profile') },
  ];
}

export function buildPanelNavGroups(wsSlug: string, canManageSettings: boolean, permissionSet?: Set<string>): Record<RailId, NavGroup[]> {
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
          { link: `/w/${wsSlug}/crm/contacts`, label: 'Contacts', icon: UserGroupIcon },
          { link: `/w/${wsSlug}/crm/companies`, label: 'Companies', icon: Building03Icon },
          { link: `/w/${wsSlug}/crm/deals`, label: 'Deals', icon: DollarCircleIcon },
          { link: `/w/${wsSlug}/crm/review`, label: 'Review', icon: ClipboardIcon },
          { link: `/w/${wsSlug}/crm/insights`, label: 'Insights', icon: BulbIcon },
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
          { link: `/w/${wsSlug}/automation/activity`, label: 'Activity', icon: Clock01Icon },
          { link: `/w/${wsSlug}/automation/agents`, label: 'Agents', icon: BotIcon },
        ],
      },
      {
        label: 'Library',
        items: [
          { link: `/w/${wsSlug}/automation/library`, label: 'Trigger Catalog', icon: BotIcon },
          { link: `/w/${wsSlug}/automation/tools`, label: 'Tool Catalog', icon: Wrench01Icon },
          { link: `/w/${wsSlug}/automation/skills`, label: 'Skill Catalog', icon: BookOpen01Icon },
        ],
      },
    ],
    docs: [
      {
        label: '',
        items: [
          { link: `/w/${wsSlug}/docs/recent`, label: 'Recent Docs', icon: Clock01Icon },
          { link: `/w/${wsSlug}/docs/my`, label: 'My Documents', icon: UserIcon },
          { link: `/w/${wsSlug}/docs`, label: 'All Docs', icon: File01Icon },
        ],
      },
    ],
    settings: getSettingsSidebarGroups(canManageSettings, permissionSet).map((group) => ({
      label: group.label,
      items: group.sections.map((section) => ({
        link: buildSettingsRoutePath(wsSlug, section.id),
        label: section.label,
        icon: section.icon,
      })),
    })),
  };
}

export const supportFilterItems = [
  { key: 'my_inbox' as const, label: 'My Inbox', icon: UserIcon },
  { key: 'unassigned' as const, label: 'Unassigned', icon: UserRemove01Icon },
  { key: 'mentions' as const, label: 'Mentions', icon: Message01Icon },
  { key: 'all' as const, label: 'All', icon: Mail01Icon },
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
