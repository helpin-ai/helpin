import { ModuleAccessTab } from '@/components/settings';
import { SettingsPageFrame } from './SettingsPageFrame';

export function AccessSettingsPage() {
  return (
    <SettingsPageFrame section="access">
      {({ workspaceId, settings, permissions }) => (
        <ModuleAccessTab
          workspaceId={workspaceId}
          teams={settings.teams}
          people={settings.people}
          editable={permissions.canManageModuleAccess}
        />
      )}
    </SettingsPageFrame>
  );
}
