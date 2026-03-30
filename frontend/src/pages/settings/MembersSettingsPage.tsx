import { MembersTab } from '@/components/settings';
import { SettingsPageFrame } from './SettingsPageFrame';

export function MembersSettingsPage() {
  return (
    <SettingsPageFrame section="members">
      {({ workspaceId, organizationId, settings, permissions }) => (
        <MembersTab
          workspaceId={workspaceId}
          organizationId={organizationId}
          editable={permissions.canManageMembers}
          teams={settings.teams}
          userMemberships={settings.user_memberships}
        />
      )}
    </SettingsPageFrame>
  );
}
