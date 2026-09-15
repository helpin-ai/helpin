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
  GitBranchIcon,
  FileImportIcon,
  HelpCircleIcon,
  LinkForwardIcon,
  FolderKanbanIcon,
  Mail01Icon,
  Calendar03Icon,
  SlidersHorizontalIcon,
  BubbleChatIcon,
  Route01Icon,
  Shield01Icon,
  Shield02Icon,
  Globe02Icon,
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
const Security = hi(Shield02Icon);
const Account = hi(Settings02Icon);
const Billing = Account;
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
const Meetings = hi(Calendar03Icon);
const Autonomy = hi(SlidersHorizontalIcon);
const ChatWidget = hi(BubbleChatIcon);
const InboxesRouting = hi(Route01Icon);
const Access = hi(Shield01Icon);
const MCP = hi(Robot01Icon);
const ExternalMCP = hi(Globe02Icon);

export type SettingsSection =
  | 'general'
  | 'members'
  | 'teams'
  | 'access'
  | 'mcp'
  | 'external-mcp'
  | 'billing'
  | 'repositories'
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
  | 'crm-meetings'
  | 'crm-autonomy'
  | 'support-ai-assistant'
  | 'chat-general'
  | 'inboxes-routing';

export type SettingsRouteSection = SettingsSection | 'profile' | 'security' | 'notifications' | 'account' | 'git-connections';

export type SettingsSectionMeta<T extends SettingsRouteSection = SettingsRouteSection> = {
  id: T;
  label: string;
  description: string;
  icon: IconComponent;
  group: string;
  requiresManageSettings?: boolean;
  requiredPermission?: Permission;
  sidebar?: boolean;
  keywords?: string[];
  scope?: 'organization';
  options?: { id: string; label: string; keywords?: string[] }[];
};

const SETTINGS_GROUP_LABELS = ['Personal', 'Workspace & access', 'Integrations', 'Projects', 'CRM', 'Support'];
export const SETTINGS_HOME_LABEL = 'Settings home';

export const SETTINGS_ROUTE_SECTIONS: SettingsSectionMeta[] = [
  {
    id: 'profile',
    keywords: ["name", "avatar", "photo", "timezone"],
    label: 'Profile',
    description: "Your name, avatar, and personal details.",
    icon: Profile,
    group: 'Personal',
  },
  {
    id: 'security',
    keywords: ["password", "login", "authentication", "2fa"],
    label: 'Security',
    description: "Password and account security.",
    icon: Security,
    group: 'Personal',
  },
  {
    id: 'notifications',
    keywords: ["alerts", "email notifications", "desktop", "push"],
    label: 'Notifications',
    description: "Choose when and how Helpin notifies you.",
    icon: Notifications,
    group: 'Personal',
  },
  {
    id: 'account',
    scope: 'organization',
    keywords: ["organization name"],
    label: 'Organization',
    description: "Organization details and preferences.",
    icon: Account,
    group: 'Workspace & access',
  },
  {
    id: 'git-connections',
    scope: 'organization',
    label: 'Git Connections',
    description: 'Manage organization-level GitHub and GitLab provider connections.',
    icon: Delivery,
    group: 'Integrations',
  },
  {
    id: 'general',
    keywords: ["workspace name", "logo", "timezone"],
    label: 'General',
    description: "Workspace name and preferences.",
    icon: General,
    group: 'Workspace & access',
  },
  {
    id: 'billing',
    keywords: ["invoices", "payment", "subscription", "plan", "usage", "credits"],
    label: 'Billing',
    description: 'Manage this workspace plan, AI usage, payment methods, and invoices.',
    icon: Billing,
    group: 'Workspace & access',
  },
  {
    id: 'members',
    keywords: ["invite teammate", "invite user", "remove member", "roles"],
    label: 'Members',
    description: "Invite people and manage workspace membership.",
    icon: Members,
    group: 'Workspace & access',
  },
  {
    id: 'teams',
    keywords: ["team membership", "team settings"],
    label: 'Teams',
    description: "Organize people into teams.",
    icon: Teams,
    group: 'Workspace & access',
  },
  {
    id: 'access',
    label: 'Module Access',
    description: 'Grant CRM and Support access by team or by direct workspace member exception.',
    icon: Access,
    group: 'Workspace & access',
    requiredPermission: 'module_access.manage',
  },
  {
    id: 'mcp',
    label: 'MCP access',
    description: 'Connect outside MCP clients to Helpin and control their workspace access.',
    icon: MCP,
    group: 'Integrations',
    requiredPermission: 'workspace.read',
  },
  {
    id: 'external-mcp',
    label: 'External MCP',
    description: 'Connect remote MCP servers and choose which tools Helpin agents may use.',
    icon: ExternalMCP,
    group: 'Integrations',
    requiredPermission: 'settings.read',
  },
  {
    id: 'repositories',
    label: 'Repositories',
    description: 'Choose which synced Git repositories are available to this workspace.',
    icon: Delivery,
    group: 'Integrations',
  },
  {
    id: 'knowledge',
    label: 'Knowledge',
    description: 'Manage help center docs and website content sources used across AI experiences.',
    icon: Knowledge,
    group: 'Integrations',
  },
  {
    id: 'workflows',
    label: 'Workflows',
    description: 'Manage workflows, pipeline rules, and workspace-level Git provider event rules.',
    icon: Workflows,
    group: 'Projects',
    sidebar: false,
  },
  {
    id: 'delivery',
    label: 'Delivery',
    description: 'Configure project delivery repositories, branches, and team defaults.',
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
    keywords: ["rules", "triggers"],
    label: 'Automations',
    description: "Configure rules for recurring actions.",
    icon: Automations,
    group: 'Projects',
  },
  {
    id: 'support-ai-assistant',
    label: 'AI Assistant',
    description: 'Configure how support AI replies, drafts internal notes, and hands conversations to humans.',
    icon: Automations,
    group: 'Support',
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
    description: 'Widget installation, availability, identity capture, and appearance.',
    icon: ChatWidget,
    group: 'Support',
  },
  {
    id: 'helpcenter',
    options: [{"id": "helpcenter-domain", "label": "Domain & SEO", "keywords": ["custom domain", "website address", "url", "seo"]}],
    keywords: ["custom domain", "branding", "seo", "languages"],
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
    keywords: ["deal stages", "sales pipeline"],
    label: 'Deal Pipelines',
    description: 'Configure deal pipelines and stages.',
    icon: Pipelines,
    group: 'CRM',
  },
  {
    id: 'crm-email',
    options: [{"id": "email-signature", "label": "Email signature", "keywords": ["signature", "footer"]}, {"id": "email-sending-limits", "label": "Sending limits", "keywords": ["daily limit", "sending limit", "quota", "capacity", "pacing"]}, {"id": "email-sync", "label": "Sync preferences", "keywords": ["email history", "address filters", "blocklist", "allowlist"]}, {"id": "email-calendar", "label": "Calendar events", "keywords": ["private meetings", "solo meetings", "calendar sync"]}, {"id": "email-contacts", "label": "Contact creation", "keywords": ["automatic contacts", "blocked prefixes", "noreply"]}],
    keywords: ["gmail", "email sync", "mailbox", "outlook"],
    label: 'Email',
    description: 'Manage mailboxes, sending limits, and CRM email preferences.',
    icon: EmailAccounts,
    group: 'CRM',
  },
  {
    id: 'crm-meetings',
    label: 'Meeting Notes',
    description: 'Choose how Helpin joins calls, takes notes, and saves recordings.',
    icon: Meetings,
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
    keywords: ["csv", "export", "migrate"],
    label: 'Import & Export',
    description: 'Import data from Shortcut and other project management tools.',
    icon: ImportExport,
    group: 'Integrations',
  },
];

export const SETTINGS_SECTIONS = SETTINGS_ROUTE_SECTIONS.filter(
  (section): section is SettingsSectionMeta<SettingsSection> =>
    section.id !== 'profile'
    && section.id !== 'security'
    && section.id !== 'notifications'
    && section.id !== 'account'
    && section.id !== 'git-connections',
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

  return groups.sort((a, b) => SETTINGS_GROUP_LABELS.indexOf(a.label) - SETTINGS_GROUP_LABELS.indexOf(b.label));
}
