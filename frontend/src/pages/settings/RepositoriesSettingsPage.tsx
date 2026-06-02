import { WorkspaceRepositoriesTab } from '@/components/settings';
import { SettingsPageFrame } from './SettingsPageFrame';

export function RepositoriesSettingsPage() {
  return (
    <SettingsPageFrame section="repositories">
      {({ workspaceId, permissions }) => (
        <WorkspaceRepositoriesTab
          workspaceId={workspaceId}
          editable={permissions.canManageSettings}
        />
      )}
    </SettingsPageFrame>
  );
}
