import { ProjectDeliveryTab } from '@/components/settings';
import { SettingsPageFrame } from './SettingsPageFrame';

export function DeliverySettingsPage() {
  return (
    <SettingsPageFrame section="delivery">
      {({ workspaceId, settings, permissions }) => (
        <ProjectDeliveryTab
          workspaceId={workspaceId}
          editable={permissions.canManageSettings}
          teams={settings.teams}
          teamRepoDefaults={settings.team_repo_defaults}
        />
      )}
    </SettingsPageFrame>
  );
}
