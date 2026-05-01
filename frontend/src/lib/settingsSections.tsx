import type { FC, CSSProperties } from 'react';
import type { Permission } from '@/lib/types';
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
  Search01Icon,
  GitBranchIcon,
  FileImportIcon,
  HelpCircleIcon,
  LinkForwardIcon,
  FolderKanbanIcon,
  Mail01Icon,
  SlidersHorizontalIcon,
  BubbleChatIcon,
  Route01Icon,
  Shield01Icon,
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
const CommandIntents = hi(Search01Icon);
const Delivery = hi(GitBranchIcon);
const ImportExport = hi(FileImportIcon);
const HelpCenter = hi(HelpCircleIcon);
const Redirects = hi(LinkForwardIcon);
const Pipelines = hi(FolderKanbanIcon);
const EmailAccounts = hi(Mail01Icon);
const Autonomy = hi(SlidersHorizontalIcon);
const ChatWidget = hi(BubbleChatIcon);
const InboxesRouting = hi(Route01Icon);
const Access = hi(Shield01Icon);

export type SettingsSection =
  | 'general'
  | 'members'
  | 'teams'
  | 'access'
  | 'knowledge'
  | 'workflows'
  | 'labels'
  | 'task-templates'
  | 'recurring-tasks'
  | 'automations'
  | 'command-intents'
  | 'delivery'
  | 'import'
  | 'helpcenter'
  | 'redirects'
  | 'crm-pipelines'
  | 'crm-email'
  | 'crm-autonomy'
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
  requiredPermission?: Permission;
  sidebar?: boolean;
};

export const SETTINGS_ROUTE_SECTIONS: SettingsSectionMeta[] = [
  {
    id: 'profile',
    label: 'Profile',
    description: '',
    icon: Profile,
    group: 'Personal',
  },
  {
    id: 'notifications',
    label: 'Notifications',
    description: '',
    icon: Notifications,
    group: 'Personal',
  },
  {
    id: 'account',
    label: 'Organization',
    description: '',
    icon: Account,
    group: 'Organization',
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
    id: 'access',
    label: 'Access',
    description: 'Grant CRM and Support access by team or by direct workspace member exception.',
    icon: Access,
    group: 'Workspace',
    requiredPermission: 'module_access.manage',
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
    description: 'Manage workflows, pipeline rules, and workspace-level GitHub event rules.',
    icon: Workflows,
    group: 'Projects',
    sidebar: false,
  },
  {
    id: 'delivery',
    label: 'GitHub',
    description: 'Connect GitHub and manage synced repositories.',
    icon: Delivery,
    group: 'Projects',
  },
  {
    id: 'labels',
    label: 'Labels',
    description: 'Categorize and filter tasks with color-coded labels.',
    icon: Labels,
    group: 'Projects',
  },
  {
    id: 'task-templates',
    label: 'Templates',
    description: 'Define reusable templates for quick task creation.',
    icon: TaskTemplates,
    group: 'Projects',
  },
  {
    id: 'recurring-tasks',
    label: 'Recurring Tasks',
    description: 'Manage recurring work templates, schedules, failures, and generated tasks.',
    icon: RecurringTasks,
    group: 'Projects',
  },
  {
    id: 'automations',
    label: 'Automations',
    description: '',
    icon: Automations,
    group: 'Projects',
  },
  {
    id: 'command-intents',
    label: 'Command Intents',
    description: 'Review prompts the command bar could not match. Use them to add tools, preset coverage, or new presets.',
    icon: CommandIntents,
    group: 'Projects',
    requiresManageSettings: true,
  },
  {
    id: 'inboxes-routing',
    label: 'Inboxes & Routing',
    description: 'Manage team inboxes, email forwarding, and AI conversation routing.',
    icon: InboxesRouting,
    group: 'Support',
  },
  {
    id: 'chat-general',
    label: 'Chat Widget',
    description: 'Widget installation, availability, identity capture, appearance, and AI auto-reply behavior.',
    icon: ChatWidget,
    group: 'Support',
  },
  {
    id: 'helpcenter',
    label: 'Help Center',
    description: 'Configure your public help center branding, domain, and SEO.',
    icon: HelpCenter,
    group: 'Support',
  },
  {
    id: 'redirects',
    label: 'Redirects',
    description: 'Manage URL redirects for the public help center.',
    icon: Redirects,
    group: 'Support',
  },
  {
    id: 'crm-pipelines',
    label: 'Deal Pipelines',
    description: 'Configure deal pipelines and stages.',
    icon: Pipelines,
    group: 'CRM',
  },
  {
    id: 'crm-email',
    label: 'Email Sync',
    description: 'Connect Gmail to sync conversations and detect buyer signals.',
    icon: EmailAccounts,
    group: 'CRM',
  },
  {
    id: 'crm-autonomy',
    label: 'Deal Automation',
    description: 'Configure self-driving deal automation thresholds.',
    icon: Autonomy,
    group: 'CRM',
  },
  {
    id: 'import',
    label: 'Import & Export',
    description: 'Import data from Shortcut and other project management tools.',
    icon: ImportExport,
    group: 'Data',
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

export function getSettingsSidebarGroups(canManageSettings: boolean, permissionSet?: Set<string>): SettingsSidebarGroup[] {
  const groups: SettingsSidebarGroup[] = [];

  for (const section of SETTINGS_ROUTE_SECTIONS) {
    if (section.requiresManageSettings && !canManageSettings) {
      continue;
    }
    if (section.requiredPermission && !permissionSet?.has(section.requiredPermission)) {
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
