import { useEffect, useState } from 'react';
import { useTitle } from '@/hooks/useTitle';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { invalidateWorkspaceTeamsCache } from '@/hooks/useWorkspaceTeams';
import { useWorkspaceAccess, usePermissions } from '@/hooks/queries';
import { settingsService } from '@/lib/services/settingsService';
import type { WorkspaceSettings } from '@/lib/types';
import { LabelsSettings } from '@/components/pm/LabelsSettings';
import { StoryTemplatesSettings } from '@/components/pm/StoryTemplatesSettings';
import { PipelineSettings } from '@/components/crm/PipelineSettings';
import { Skeleton } from '@/components/ui/skeleton';
import { Bot, FileText, FolderKanban, GitBranch, Globe, Import, Mail, MessageSquare, RefreshCw, Settings2, Sliders, Tag, Users, type LucideIcon } from 'lucide-react';
import {
  GeneralTab,
  MembersTab,
  TeamsTab,
  PeopleTab,
  JobRolesTab,
  BonusTiersTab,
  SystemTab,
  AITab,
  ProjectDeliveryTab,
  WorkflowManager,
  AutomationsTab,
  ImportTab,
  HelpcenterTab,
  CRMEmailSettingsTab,
  CRMAutonomySettingsTab,
  ChatGeneralTab,
  ChatAITab,
} from '@/components/settings';

export type SettingsSection = 'general' | 'members' | 'teams' | 'notifications' | 'people' | 'jobroles' | 'tiers' | 'workflows' | 'labels' | 'story-templates' | 'automations' | 'delivery' | 'ai' | 'import' | 'helpcenter' | 'crm-pipelines' | 'crm-email' | 'crm-autonomy' | 'chat-general' | 'chat-ai' | 'system' | 'account';

export const SETTINGS_SECTIONS: { id: SettingsSection; label: string; description: string; icon: LucideIcon; group: string }[] = [
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
    id: 'workflows',
    label: 'Workflows',
    description: '',
    icon: GitBranch,
    group: 'Project Settings',
  },
  {
    id: 'labels',
    label: 'Labels',
    description: '',
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
    id: 'ai',
    label: 'AI',
    description: 'Choose the workspace planning methodology used for epic PRD and story planning.',
    icon: Bot,
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
    group: 'Docs',
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
    id: 'chat-general',
    label: 'Chat Widget',
    description: 'Widget installation, identity capture, and CRM integration.',
    icon: MessageSquare,
    group: 'Support Settings',
  },
  {
    id: 'chat-ai',
    label: 'AI & Routing',
    description: 'AI auto-reply, handoff routing, business hours, and CSAT.',
    icon: Bot,
    group: 'Support Settings',
  },
  /* {
    id: 'people',
    label: 'People',
    description: '',
    icon: UserPlus,
    group: 'Reward Settings',
  },
  {
    id: 'jobroles',
    label: 'Job Roles',
    description: '',
    icon: Briefcase,
    group: 'Reward Settings',
  },
  {
    id: 'tiers',
    label: 'Bonus Tiers',
    description: '',
    icon: Award,
    group: 'Reward Settings',
  },
  {
    id: 'system',
    label: 'Reward Defaults',
    description: '',
    icon: Settings2,
    group: 'Reward Settings',
  }, */
];

export const isSettingsSection = (value: string): value is SettingsSection =>
  SETTINGS_SECTIONS.some((section) => section.id === value) || value === 'account';

export default function Settings({ section, initialWorkflowId, initialTeamId }: { section: SettingsSection; initialWorkflowId?: string; initialTeamId?: string }) {
  useTitle('Settings');
  const { currentWorkspace } = useWorkspaceStore();
  const wsId = currentWorkspace?.id ?? '';
  const { data: access } = useWorkspaceAccess(wsId);
  const { canManageSettings, canManageMembers, canManageTeams, canAdminWorkflows, canAdminLabels, canAdminAutomations, canImport } = usePermissions(access);
  const [settings, setSettings] = useState<WorkspaceSettings | null>(null);
  const [loading, setLoading] = useState(true);

  const load = async (silent = false) => {
    const ws = useWorkspaceStore.getState().currentWorkspace;
    if (!ws?.id) {
      setSettings(null);
      if (!silent) setLoading(false);
      return;
    }
    if (!silent) setLoading(true);
    try {
      const { data } = await settingsService.getAll(ws.id);
      if (data) {
        setSettings(data);
        invalidateWorkspaceTeamsCache();
      }
    } finally {
      if (!silent) setLoading(false);
    }
  };

  useEffect(() => { load(); }, [currentWorkspace?.id]);

  if (loading) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-10 w-48" />
        <Skeleton className="h-96" />
      </div>
    );
  }

  if (!settings) {
    return (
      <div className="text-center py-12">
        <p className="text-muted-foreground">Could not load workspace settings.</p>
      </div>
    );
  }

  const workspaceId = currentWorkspace?.id ?? settings.settings.workspace_id;
  if (!workspaceId) {
    return (
      <div className="text-center py-12">
        <p className="text-muted-foreground">Could not determine workspace for settings.</p>
      </div>
    );
  }

  const sectionMeta = SETTINGS_SECTIONS.find((candidate) => candidate.id === section)!;

  const renderSection = () => {
    switch (section) {
      case 'general':
        return (
          <GeneralTab
            workspaceId={workspaceId}
            editable={canManageSettings}
          />
        );
      case 'members':
        return (
          <MembersTab
            workspaceId={workspaceId}
            organizationId={currentWorkspace?.organization_id}
            editable={canManageMembers}
          />
        );
      case 'teams':
        return (
          <TeamsTab
            workspaceId={workspaceId}
            teams={settings.teams}
            userMemberships={settings.user_memberships}
            invitationPreassignments={settings.invitation_team_preassignments}
            teamEstimateSettings={settings.team_estimate_settings}
            teamFieldVisibility={settings.team_field_visibility}
            teamRepoDefaults={settings.team_repo_defaults}
            editable={canManageTeams}
            onRefresh={load}
            initialTeamId={initialTeamId}
          />
        );
      case 'people':
        return (
          <PeopleTab
            workspaceId={workspaceId}
            people={settings.people}
            editable={canManageMembers}
            onRefresh={load}
          />
        );
      case 'jobroles':
        return (
          <JobRolesTab
            workspaceId={workspaceId}
            criteria={settings.job_role_criteria}
            editable={canManageSettings}
            onRefresh={load}
          />
        );
      case 'tiers':
        return (
          <BonusTiersTab
            workspaceId={workspaceId}
            tiers={settings.bonus_tiers}
            editable={canManageSettings}
            onRefresh={load}
          />
        );
      case 'system':
        return (
          <SystemTab
            workspaceId={workspaceId}
            config={settings.settings}
            editable={canManageSettings}
            onRefresh={load}
          />
        );
      case 'delivery':
        return (
          <ProjectDeliveryTab
            workspaceId={workspaceId}
            editable={canManageSettings}
          />
        );
      case 'ai':
        return (
          <AITab
            workspaceId={workspaceId}
            config={settings.settings}
            editable={canManageSettings}
            onRefresh={load}
          />
        );
      case 'workflows':
        return (
          <WorkflowManager
            workspaceId={workspaceId}
            teams={settings.teams}
            editable={canAdminWorkflows}
            initialWorkflowId={initialWorkflowId}
          />
        );
      case 'labels':
        return <LabelsSettings workspaceId={workspaceId} initialTeamId={initialTeamId} editable={canAdminLabels} />;
      case 'story-templates':
        return <StoryTemplatesSettings workspaceId={workspaceId} initialTeamId={initialTeamId} />;
      case 'automations':
        return <AutomationsTab workspaceId={workspaceId} teams={settings.teams} editable={canAdminAutomations} />;
      case 'import':
        return <ImportTab workspaceId={workspaceId} editable={canImport} />;
      case 'helpcenter':
        return <HelpcenterTab workspaceId={workspaceId} workspaceName={currentWorkspace?.name ?? ''} />;
      case 'crm-pipelines':
        return <PipelineSettings />;
      case 'crm-email':
        return <CRMEmailSettingsTab workspaceId={workspaceId} />;
      case 'crm-autonomy':
        return <CRMAutonomySettingsTab workspaceId={workspaceId} />;
      case 'chat-general':
        return <ChatGeneralTab workspaceId={workspaceId} />;
      case 'chat-ai':
        return <ChatAITab workspaceId={workspaceId} />;
      default:
        return null;
    }
  };

  return (
    <div className="space-y-4">
      {section !== 'teams' && (
        <div>
          <h2 className="text-xl font-semibold">{sectionMeta.label}</h2>
          {sectionMeta.description && (
            <p className="text-sm text-muted-foreground">{sectionMeta.description}</p>
          )}
        </div>
      )}
      {renderSection()}
    </div>
  );
}
