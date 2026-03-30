import { TeamsTab } from '@/components/settings';
import { invalidateWorkspaceTeamsCache } from '@/hooks/useWorkspaceTeams';
import { useInvalidateSettings } from '@/hooks/queries/useSettings';
import { SettingsPageFrame, type SettingsPageContext } from './SettingsPageFrame';

export function TeamsSettingsPage({ initialTeamId }: { initialTeamId?: string }) {
  return (
    <SettingsPageFrame section="teams">
      {(context) => <TeamsSettingsContent {...context} initialTeamId={initialTeamId} />}
    </SettingsPageFrame>
  );
}

function TeamsSettingsContent({
  workspaceId,
  settings,
  access,
  permissions,
  initialTeamId,
}: SettingsPageContext & {
  initialTeamId?: string;
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
      access={access}
    />
  );
}
