import { TeamsTab } from '@/components/settings';
import { invalidateWorkspaceTeamsCache } from '@/hooks/useWorkspaceTeams';
import { useInvalidateSettings } from '@/hooks/queries/useSettings';
import { SettingsPageFrame, type SettingsPageContext } from './SettingsPageFrame';

export function TeamsSettingsPage({ initialTeamId, initialSection }: { initialTeamId?: string; initialSection?: string }) {
  return (
    <SettingsPageFrame section="teams" hideHeader={!!initialTeamId}>
      {(context) => <TeamsSettingsContent {...context} initialTeamId={initialTeamId} initialSection={initialSection} />}
    </SettingsPageFrame>
  );
}

function TeamsSettingsContent({
  workspaceId,
  settings,
  access,
  permissions,
  initialTeamId,
  initialSection,
}: SettingsPageContext & {
  initialTeamId?: string;
  initialSection?: string;
}) {
  const invalidateSettings = useInvalidateSettings(workspaceId);

  return (
    <TeamsTab
      workspaceId={workspaceId}
      teams={settings.teams}
      userMemberships={settings.user_memberships}
      invitationPreassignments={settings.invitation_team_preassignments}
      teamEstimateSettings={settings.team_estimate_settings}
      teamFieldVisibility={settings.team_field_visibility}
      teamRepoDefaults={settings.team_repo_defaults}
      editable={permissions.canManageTeams}
      onRefresh={() => {
        invalidateWorkspaceTeamsCache();
        invalidateSettings();
      }}
      initialTeamId={initialTeamId}
      initialSection={initialSection}
      access={access}
    />
  );
}
