import { useEffect, useState } from 'react';
import { useTitle } from '@/hooks/useTitle';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { invalidateWorkspaceTeamsCache } from '@/hooks/useWorkspaceTeams';
import { useWorkspaceAccess, usePermissions } from '@/hooks/queries';
import { settingsService } from '@/lib/services/settingsService';
import type { WorkspaceSettings } from '@/lib/types';
import { LabelsSettings } from '@/components/pm/LabelsSettings';
import { RecurringTemplatesSettings } from '@/components/pm/RecurringTemplatesSettings';
import { StoryTemplatesSettings } from '@/components/pm/StoryTemplatesSettings';
import { PipelineSettings } from '@/components/crm/PipelineSettings';
import { Skeleton } from '@/components/ui/skeleton';
import {
  GeneralTab,
  MembersTab,
  TeamsTab,
  KnowledgeTab,
  ProjectDeliveryTab,
  AutomationsTab,
  ImportTab,
  HelpcenterTab,
  CRMEmailSettingsTab,
  CRMAutonomySettingsTab,
  AIAutomationsTab,
  ChatGeneralTab,
  RedirectsTab,
  TeamInboxesTab,
  SupportEmailForwardingTab,
} from '@/components/settings';
import { SETTINGS_SECTIONS, isSettingsSection, type SettingsSection } from '@/lib/settingsSections';

export type { SettingsSection };
export { isSettingsSection };

export default function Settings({ section, initialTeamId }: { section: SettingsSection; initialWorkflowId?: string; initialTeamId?: string }) {
  useTitle('Settings');
  const { currentWorkspace } = useWorkspaceStore();
  const wsId = currentWorkspace?.id ?? '';
  const { data: access } = useWorkspaceAccess(wsId);
  const { canEdit, canManageSettings, canManageMembers, canManageTeams, canAdminLabels, canAdminAutomations, canImport } = usePermissions(access);
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
            teams={settings.teams}
            userMemberships={settings.user_memberships}
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
            access={access}
          />
        );
      case 'knowledge':
        return <KnowledgeTab workspaceId={workspaceId} />;
      case 'delivery':
        return (
          <ProjectDeliveryTab
            workspaceId={workspaceId}
            editable={canManageSettings}
          />
        );
      case 'workflows':
        // Workflows are now managed per-team; fall through to teams
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
            access={access}
            initialTeamId={initialTeamId}
          />
        );
      case 'labels':
        return <LabelsSettings workspaceId={workspaceId} initialTeamId={initialTeamId} editable={canAdminLabels} />;
      case 'story-templates':
        return <StoryTemplatesSettings workspaceId={workspaceId} initialTeamId={initialTeamId} />;
      case 'recurring-tasks':
        return <RecurringTemplatesSettings workspaceId={workspaceId} initialTeamId={initialTeamId} editable={canEdit} />;
      case 'automations':
        return <AutomationsTab workspaceId={workspaceId} teams={settings.teams} editable={canAdminAutomations} />;
      case 'import':
        return <ImportTab workspaceId={workspaceId} editable={canImport} />;
      case 'helpcenter':
        return <HelpcenterTab workspaceId={workspaceId} workspaceName={currentWorkspace?.name ?? ''} />;
      case 'redirects':
        return <RedirectsTab workspaceId={workspaceId} editable={canManageSettings} />;
      case 'crm-pipelines':
        return <PipelineSettings />;
      case 'crm-email':
        return <CRMEmailSettingsTab workspaceId={workspaceId} />;
      case 'crm-autonomy':
        return <CRMAutonomySettingsTab workspaceId={workspaceId} />;
      case 'ai-automations':
        if (!canManageSettings) {
          return (
            <div className="rounded-none border border-border bg-card px-6 py-8 text-sm text-muted-foreground">
              You do not have permission to view the shared AI & Automations governance surface.
            </div>
          );
        }
        return <AIAutomationsTab workspaceId={workspaceId} />;
      case 'chat-general':
        return <ChatGeneralTab workspaceId={workspaceId} />;
      case 'team-inboxes':
        return <TeamInboxesTab workspaceId={workspaceId} />;
      case 'email-forwarding':
        return <SupportEmailForwardingTab workspaceId={workspaceId} />;
      default:
        return null;
    }
  };

  return (
    <div className="space-y-4">
      <div>
        <h2 className="text-xl font-semibold">{sectionMeta.label}</h2>
        {sectionMeta.description && (
          <p className="text-sm text-muted-foreground">{sectionMeta.description}</p>
        )}
      </div>
      {renderSection()}
    </div>
  );
}
