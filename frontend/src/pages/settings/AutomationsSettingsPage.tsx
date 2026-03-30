import { AutomationsTab } from '@/components/settings';
import { SettingsPageFrame } from './SettingsPageFrame';

export function AutomationsSettingsPage() {
  return (
    <SettingsPageFrame section="automations">
      {({ workspaceId, settings, permissions }) => (
        <AutomationsTab workspaceId={workspaceId} teams={settings.teams} editable={permissions.canAdminAutomations} />
      )}
    </SettingsPageFrame>
  );
}
