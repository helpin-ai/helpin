import {
  BookOpen,
  Bot,
  FileText,
  FolderKanban,
  Globe,
  Import,
  Mail,
  MessageSquare,
  RefreshCw,
  Settings2,
  Sliders,
  Sparkles,
  Tag,
  User,
  Users,
  Bell,
  type LucideIcon,
} from 'lucide-react';

export type SettingsSection =
  | 'general'
  | 'members'
  | 'teams'
  | 'knowledge'
  | 'workflows'
  | 'labels'
  | 'story-templates'
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
  | 'team-inboxes'
  | 'email-forwarding'
  | 'chat-ai';

export type SettingsRouteSection = SettingsSection | 'profile' | 'notifications' | 'account';

export type SettingsSectionMeta<T extends SettingsRouteSection = SettingsRouteSection> = {
  id: T;
  label: string;
  description: string;
  icon: LucideIcon;
  group: string;
  requiresManageSettings?: boolean;
  sidebar?: boolean;
};

export const SETTINGS_ROUTE_SECTIONS: SettingsSectionMeta[] = [
  {
    id: 'profile',
    label: 'Profile',
    description: '',
    icon: User,
    group: 'My Account',
  },
  {
    id: 'account',
    label: 'Account',
    description: '',
    icon: Settings2,
    group: 'My Account',
  },
  {
    id: 'notifications',
    label: 'Notifications',
    description: '',
    icon: Bell,
    group: 'My Account',
  },
  {
    id: 'general',
    label: 'General',
    description: '',
    icon: Settings2,
    group: 'Workspace',
  },
  {
    id: 'members',
    label: 'Members',
    description: '',
    icon: Users,
    group: 'Workspace',
  },
  {
    id: 'teams',
    label: 'Teams',
    description: '',
    icon: Users,
    group: 'Workspace',
  },
  {
    id: 'knowledge',
    label: 'Knowledge',
    description: 'Manage help center docs and website content sources used across AI experiences.',
    icon: BookOpen,
    group: 'Workspace',
  },
  {
    id: 'workflows',
    label: 'Workflows',
    description: 'Legacy workflow settings route kept for compatibility. Workflow management now lives under teams.',
    icon: RefreshCw,
    group: 'Project Settings',
    sidebar: false,
  },
  {
    id: 'labels',
    label: 'Labels',
    description: 'Categorize and filter stories with color-coded labels.',
    icon: Tag,
    group: 'Project Settings',
  },
  {
    id: 'story-templates',
    label: 'Story Templates',
    description: 'Define reusable templates for quick story creation.',
    icon: FileText,
    group: 'Project Settings',
  },
  {
    id: 'recurring-tasks',
    label: 'Recurring Tasks',
    description: 'Manage recurring work templates, schedules, failures, and generated stories.',
    icon: RefreshCw,
    group: 'Project Settings',
  },
  {
    id: 'automations',
    label: 'Automations',
    description: '',
    icon: RefreshCw,
    group: 'Project Settings',
  },
  {
    id: 'delivery',
    label: 'Delivery',
    description: 'Connect GitHub, curate repositories, and monitor shared runner pools.',
    icon: Globe,
    group: 'Project Settings',
  },
  {
    id: 'import',
    label: 'Import / Export',
    description: 'Import data from Shortcut and other project management tools.',
    icon: Import,
    group: 'Data',
  },
  {
    id: 'helpcenter',
    label: 'Help Center',
    description: 'Configure your public help center branding, domain, and SEO.',
    icon: Globe,
    group: 'Support & Docs',
  },
  {
    id: 'redirects',
    label: 'Redirects',
    description: 'Manage URL redirects for the public help center.',
    icon: RefreshCw,
    group: 'Support & Docs',
  },
  {
    id: 'crm-pipelines',
    label: 'Pipelines',
    description: 'Configure deal pipelines and stages.',
    icon: FolderKanban,
    group: 'CRM Settings',
  },
  {
    id: 'crm-email',
    label: 'Email Accounts',
    description: 'Connect Gmail to sync conversations and detect buyer signals.',
    icon: Mail,
    group: 'CRM Settings',
  },
  {
    id: 'crm-autonomy',
    label: 'Autonomy',
    description: 'Configure self-driving deal automation thresholds.',
    icon: Sliders,
    group: 'CRM Settings',
  },
  {
    id: 'ai-automations',
    label: 'AI & Automations',
    description: 'Read-only inventory and health for shared built-ins and contextual agents.',
    icon: Sparkles,
    group: 'AI & Automations',
    requiresManageSettings: true,
  },
  {
    id: 'chat-general',
    label: 'Chat Widget',
    description: 'Widget installation, availability, identity capture, appearance, AI auto-reply, and routing.',
    icon: MessageSquare,
    group: 'Support & Docs',
  },
  {
    id: 'team-inboxes',
    label: 'Team Inboxes',
    description: 'Create private support inboxes, add members, pick icons, and link support teams.',
    icon: MessageSquare,
    group: 'Support & Docs',
  },
  {
    id: 'email-forwarding',
    label: 'Email Forwarding',
    description: 'Generate forwarding addresses for Shared Inbox and Team Inboxes.',
    icon: Mail,
    group: 'Support & Docs',
  },
  {
    id: 'chat-ai',
    label: 'AI & Routing',
    description: 'Configure support AI auto-reply, routing, handoff, and related chat behavior.',
    icon: Bot,
    group: 'Support & Docs',
    sidebar: false,
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
