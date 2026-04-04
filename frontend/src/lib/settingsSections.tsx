import type { FC, CSSProperties } from 'react';
import { HugeiconsIcon } from '@hugeicons/react';
import {
  UserIcon,
  Settings02Icon,
  Notification02Icon,
  Settings01Icon,
  UserGroupIcon,
  UserMultipleIcon,
  BookOpen01Icon,
  ArrowReloadHorizontalIcon,
  Tag01Icon,
  File01Icon,
  RepeatIcon,
  Robot01Icon,
  GitBranchIcon,
  FileImportIcon,
  HelpCircleIcon,
  LinkForwardIcon,
  FolderKanbanIcon,
  Mail01Icon,
  SlidersHorizontalIcon,
  SparklesIcon,
  BubbleChatIcon,
  Route01Icon,
} from '@hugeicons/core-free-icons';

export type IconComponent = FC<{ className?: string; style?: CSSProperties }>;

type HugeIconData = Parameters<typeof HugeiconsIcon>[0]['icon'];

function hi(icon: HugeIconData): IconComponent {
  const C = ({ className, style }: { className?: string; style?: CSSProperties }) => (
    <HugeiconsIcon icon={icon} className={className} style={style} strokeWidth={2} />
  );
  return C;
}

const Profile = hi(UserIcon);
const Account = hi(Settings02Icon);
const Notifications = hi(Notification02Icon);
const General = hi(Settings01Icon);
const Members = hi(UserGroupIcon);
const Teams = hi(UserMultipleIcon);
const Knowledge = hi(BookOpen01Icon);
const Workflows = hi(ArrowReloadHorizontalIcon);
const Labels = hi(Tag01Icon);
const TaskTemplates = hi(File01Icon);
const RecurringTasks = hi(RepeatIcon);
const Automations = hi(Robot01Icon);
const Delivery = hi(GitBranchIcon);
const ImportExport = hi(FileImportIcon);
const HelpCenter = hi(HelpCircleIcon);
const Redirects = hi(LinkForwardIcon);
const Pipelines = hi(FolderKanbanIcon);
const EmailAccounts = hi(Mail01Icon);
const Autonomy = hi(SlidersHorizontalIcon);
const AIAutomations = hi(SparklesIcon);
const ChatWidget = hi(BubbleChatIcon);
const InboxesRouting = hi(Route01Icon);

export type SettingsSection =
  | 'general'
  | 'members'
  | 'teams'
  | 'knowledge'
  | 'workflows'
  | 'labels'
  | 'task-templates'
  | 'recurring-tasks'
  | 'automations'
  | 'delivery'
  | 'import'
  | 'helpcenter'
  | 'redirects'
  | 'crm-pipelines'
  | 'crm-email'
  | 'crm-autonomy'
  | 'ai-automations'
  | 'chat-general'
  | 'inboxes-routing';

export type SettingsRouteSection = SettingsSection | 'profile' | 'notifications' | 'account';

export type SettingsSectionMeta<T extends SettingsRouteSection = SettingsRouteSection> = {
  id: T;
  label: string;
  description: string;
  icon: IconComponent;
  group: string;
  requiresManageSettings?: boolean;
  sidebar?: boolean;
};

export const SETTINGS_ROUTE_SECTIONS: SettingsSectionMeta[] = [
  {
    id: 'profile',
    label: 'Profile',
    description: '',
    icon: Profile,
    group: 'My Account',
  },
  {
    id: 'account',
    label: 'Account',
    description: '',
    icon: Account,
    group: 'My Account',
  },
  {
    id: 'notifications',
    label: 'Notifications',
    description: '',
    icon: Notifications,
    group: 'My Account',
  },
  {
    id: 'general',
    label: 'General',
    description: '',
    icon: General,
    group: 'Workspace',
  },
  {
    id: 'members',
    label: 'Members',
    description: '',
    icon: Members,
    group: 'Workspace',
  },
  {
    id: 'teams',
    label: 'Teams',
    description: '',
    icon: Teams,
    group: 'Workspace',
  },
  {
    id: 'knowledge',
    label: 'Knowledge',
    description: 'Manage help center docs and website content sources used across AI experiences.',
    icon: Knowledge,
    group: 'Workspace',
  },
  {
    id: 'workflows',
    label: 'Workflows',
    description: 'Legacy workflow settings route kept for compatibility. Workflow management now lives under teams.',
    icon: Workflows,
    group: 'Project Settings',
    sidebar: false,
  },
  {
    id: 'labels',
    label: 'Labels',
    description: 'Categorize and filter tasks with color-coded labels.',
    icon: Labels,
    group: 'Project Settings',
  },
  {
    id: 'task-templates',
    label: 'Task Templates',
    description: 'Define reusable templates for quick task creation.',
    icon: TaskTemplates,
    group: 'Project Settings',
  },
  {
    id: 'recurring-tasks',
    label: 'Recurring Tasks',
    description: 'Manage recurring work templates, schedules, failures, and generated tasks.',
    icon: RecurringTasks,
    group: 'Project Settings',
  },
  {
    id: 'automations',
    label: 'Automations',
    description: '',
    icon: Automations,
    group: 'Project Settings',
  },
  {
    id: 'delivery',
    label: 'Git Integration',
    description: 'Connect GitHub and manage synced repositories.',
    icon: Delivery,
    group: 'Project Settings',
  },
  {
    id: 'import',
    label: 'Import / Export',
    description: 'Import data from Shortcut and other project management tools.',
    icon: ImportExport,
    group: 'Data',
  },
  {
    id: 'helpcenter',
    label: 'Help Center',
    description: 'Configure your public help center branding, domain, and SEO.',
    icon: HelpCenter,
    group: 'Support & Docs',
  },
  {
    id: 'redirects',
    label: 'Redirects',
    description: 'Manage URL redirects for the public help center.',
    icon: Redirects,
    group: 'Support & Docs',
  },
  {
    id: 'crm-pipelines',
    label: 'Pipelines',
    description: 'Configure deal pipelines and stages.',
    icon: Pipelines,
    group: 'CRM Settings',
  },
  {
    id: 'crm-email',
    label: 'Email Accounts',
    description: 'Connect Gmail to sync conversations and detect buyer signals.',
    icon: EmailAccounts,
    group: 'CRM Settings',
  },
  {
    id: 'crm-autonomy',
    label: 'Autonomy',
    description: 'Configure self-driving deal automation thresholds.',
    icon: Autonomy,
    group: 'CRM Settings',
  },
  {
    id: 'ai-automations',
    label: 'AI & Automations',
    description: 'Read-only inventory and health for shared built-ins and contextual agents.',
    icon: AIAutomations,
    group: 'AI & Automations',
    requiresManageSettings: true,
  },
  {
    id: 'chat-general',
    label: 'Chat Widget',
    description: 'Widget installation, availability, identity capture, appearance, and AI auto-reply behavior.',
    icon: ChatWidget,
    group: 'Support & Docs',
  },
  {
    id: 'inboxes-routing',
    label: 'Inboxes & Routing',
    description: 'Manage team inboxes, email forwarding, and AI conversation routing.',
    icon: InboxesRouting,
    group: 'Support & Docs',
  },
];

export const SETTINGS_SECTIONS = SETTINGS_ROUTE_SECTIONS.filter(
  (section): section is SettingsSectionMeta<SettingsSection> =>
    section.id !== 'profile' && section.id !== 'notifications' && section.id !== 'account',
);

export const isSettingsSection = (value: string): value is SettingsSection =>
  SETTINGS_SECTIONS.some((section) => section.id === value);

export const SETTINGS_SECTION_LABELS: Record<SettingsRouteSection, string> = SETTINGS_ROUTE_SECTIONS.reduce(
  (labels, section) => {
    labels[section.id] = section.label;
    return labels;
  },
  {} as Record<SettingsRouteSection, string>,
);

export function buildSettingsRoutePath(workspaceSlug: string, section: SettingsRouteSection): string {
  return `/w/${workspaceSlug}/settings/${section}`;
}

export type SettingsSidebarGroup = {
  label: string;
  sections: SettingsSectionMeta[];
};

export function getSettingsSidebarGroups(canManageSettings: boolean): SettingsSidebarGroup[] {
  const groups: SettingsSidebarGroup[] = [];

  for (const section of SETTINGS_ROUTE_SECTIONS) {
    if (section.requiresManageSettings && !canManageSettings) {
      continue;
    }
    if (section.sidebar === false) {
      continue;
    }

    const existingGroup = groups.find((group) => group.label === section.group);
    if (existingGroup) {
      existingGroup.sections.push(section);
      continue;
    }

    groups.push({ label: section.group, sections: [section] });
  }

  return groups;
}
