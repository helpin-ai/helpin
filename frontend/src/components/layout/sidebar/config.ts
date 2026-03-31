import {
  ArrowUpRight,
  BarChart3,
  Bot,
  Briefcase,
  Building2,
  CheckCircle2,
  CircleDot,
  ClipboardCheck,
  Clock,
  DollarSign,
  FileText,
  FolderKanban,
  GanttChart,
  Inbox,
  LayoutList,
  Layers,
  Lightbulb,
  Loader2,
  Mail,
  MessageSquare,
  OctagonX,
  Pause,
  Play,
  RefreshCw,
  Settings,
  SquareKanban,
  Target,
  User,
  UserX,
  Users,
  Wrench,
  XCircle,
  type LucideIcon,
} from 'lucide-react';
import { buildSettingsRoutePath, getSettingsSidebarGroups } from '@/lib/settingsSections';
import type { NavGroup, RailId, RailItem } from './types';

export function deriveActiveRail(pathname: string): RailId {
  if (pathname.includes('/settings')) return 'settings';
  if (pathname.includes('/support')) return 'support';
  if (pathname.includes('/crm')) return 'crm';
  if (pathname.includes('/pm/agent-runs') || pathname.includes('/pm/agents') || pathname.includes('/pm/tool-catalog')) return 'agents';
  if (pathname.includes('/pm/') || pathname.endsWith('/pm')) return 'projects';
  if (pathname.includes('/docs')) return 'docs';
  return 'projects';
}

export const teamSubItems: { key: string; label: string; icon: LucideIcon; path: string }[] = [
  { key: 'stories', label: 'Stories', icon: LayoutList, path: 'stories' },
  { key: 'epics', label: 'Epics', icon: Layers, path: 'epics' },
  { key: 'sprints', label: 'Sprints', icon: RefreshCw, path: 'sprints' },
];

export const projectCreateOptions = [
  { key: 'story' as const, label: 'Story', icon: SquareKanban, pages: ['stories'] },
  { key: 'epic' as const, label: 'Epic', icon: Layers, pages: ['epics'] },
  { key: 'sprint' as const, label: 'Sprint', icon: RefreshCw, pages: ['sprints'] },
  { key: 'objective' as const, label: 'Objective', icon: Target, pages: ['objectives'] },
];

export function buildRailItems(wsSlug: string, totalSupportUnread: number): RailItem[] {
  return [
    { id: 'projects', label: 'Projects', icon: FolderKanban, defaultLink: `/w/${wsSlug}/pm/my-work` },
    { id: 'crm', label: 'CRM', icon: Briefcase, defaultLink: `/w/${wsSlug}/crm/contacts` },
    { id: 'support', label: 'Support', icon: MessageSquare, defaultLink: `/w/${wsSlug}/support`, indicator: Boolean(totalSupportUnread) },
    { id: 'agents', label: 'Automation', icon: Bot, defaultLink: `/w/${wsSlug}/pm/agent-runs` },
    { id: 'docs', label: 'Docs', icon: FileText, defaultLink: `/w/${wsSlug}/docs` },
    { id: 'settings', label: 'Settings', icon: Settings, defaultLink: buildSettingsRoutePath(wsSlug, 'profile') },
  ];
}

export function buildPanelNavGroups(wsSlug: string, canManageSettings: boolean): Record<RailId, NavGroup[]> {
  return {
    projects: [
      {
        label: '',
        items: [
          { link: `/w/${wsSlug}/pm/my-work`, label: 'My Work', icon: ClipboardCheck },
          { link: `/w/${wsSlug}/pm/objectives`, label: 'Objectives', icon: Target },
          { link: `/w/${wsSlug}/pm/roadmap`, label: 'Roadmap', icon: GanttChart },
          { link: `/w/${wsSlug}/pm/reports`, label: 'Reports', icon: BarChart3 },
        ],
      },
    ],
    crm: [
      {
        label: '',
        items: [
          { link: `/w/${wsSlug}/crm/contacts`, label: 'Contacts', icon: Users },
          { link: `/w/${wsSlug}/crm/companies`, label: 'Companies', icon: Building2 },
          { link: `/w/${wsSlug}/crm/deals`, label: 'Deals', icon: DollarSign },
          { link: `/w/${wsSlug}/crm/lists`, label: 'Lists', icon: LayoutList },
          { link: `/w/${wsSlug}/crm/sequences`, label: 'Sequences', icon: Play },
          { link: `/w/${wsSlug}/crm/review`, label: 'Review', icon: ClipboardCheck },
          { link: `/w/${wsSlug}/crm/insights`, label: 'Insights', icon: Lightbulb },
        ],
      },
    ],
    support: [
      {
        label: '',
        items: [],
      },
    ],
    agents: [
      {
        label: '',
        items: [
          { link: `/w/${wsSlug}/pm/agent-runs`, label: 'Runs', icon: Clock },
          { link: `/w/${wsSlug}/pm/agents`, label: 'Agents', icon: Bot },
          { link: `/w/${wsSlug}/pm/tool-catalog`, label: 'Tool Catalog', icon: Wrench },
        ],
      },
    ],
    docs: [
      {
        label: '',
        items: [
          { link: `/w/${wsSlug}/docs/recent`, label: 'Recent Docs', icon: Clock },
          { link: `/w/${wsSlug}/docs/my`, label: 'My Documents', icon: User },
          { link: `/w/${wsSlug}/docs`, label: 'All Docs', icon: FileText },
        ],
      },
    ],
    settings: getSettingsSidebarGroups(canManageSettings).map((group) => ({
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
  { key: 'my_inbox' as const, label: 'My Inbox', icon: User },
  { key: 'unassigned' as const, label: 'Unassigned', icon: UserX },
  { key: 'mentions' as const, label: 'Mentions', icon: MessageSquare },
  { key: 'all' as const, label: 'All', icon: Mail },
];

export const supportStatusOptions: readonly { value: string; label: string; icon: LucideIcon; color: string }[] = [
  { value: 'all', label: 'All statuses', icon: Inbox, color: 'text-muted-foreground' },
  { value: 'open', label: 'Open', icon: CircleDot, color: 'text-blue-500' },
  { value: 'in_progress', label: 'In Progress', icon: Loader2, color: 'text-amber-500' },
  { value: 'waiting', label: 'Waiting', icon: Pause, color: 'text-orange-500' },
  { value: 'resolved', label: 'Resolved', icon: CheckCircle2, color: 'text-emerald-500' },
  { value: 'closed', label: 'Closed', icon: XCircle, color: 'text-slate-400' },
  { value: 'spam', label: 'Spam', icon: OctagonX, color: 'text-red-500' },
] as const;

export const supportAiItems = [
  { key: 'ai_all' as const, label: 'All AI', icon: Bot },
  { key: 'ai_pending' as const, label: 'Pending', icon: Clock },
  { key: 'ai_resolved' as const, label: 'Resolved', icon: CheckCircle2 },
  { key: 'ai_escalated' as const, label: 'Escalated', icon: ArrowUpRight },
];
