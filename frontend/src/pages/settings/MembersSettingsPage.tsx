import { MembersTab } from '@/components/settings';
import { invalidateWorkspaceTeamsCache } from '@/hooks/useWorkspaceTeams';
import { useInvalidateSettings } from '@/hooks/queries/useSettings';
import { SettingsPageFrame, type SettingsPageContext } from './SettingsPageFrame';

export function MembersSettingsPage() {
  return (
    <SettingsPageFrame section="members" hideHeader>
      {(context) => <MembersSettingsContent {...context} />}
    </SettingsPageFrame>
  );
}

function MembersSettingsContent({ workspaceId, organizationId, settings, permissions }: SettingsPageContext) {
  const invalidateSettings = useInvalidateSettings(workspaceId);
  return (
    <MembersTab
      workspaceId={workspaceId}
      organizationId={organizationId}
      editable={permissions.canManageMembers}
      canManageTeams={permissions.canManageTeams}
      canManageModuleAccess={permissions.canManageModuleAccess}
      teams={settings.teams}
      userMemberships={settings.user_memberships}
      onRefresh={() => {
        invalidateWorkspaceTeamsCache();
        invalidateSettings();
      }}
    />
  );
}
