import { GeneralTab } from '@/components/settings';
import { SettingsPageFrame } from './SettingsPageFrame';

export function GeneralSettingsPage() {
  return (
    <SettingsPageFrame section="general">
      {({ workspaceId, permissions }) => (
        <GeneralTab workspaceId={workspaceId} editable={permissions.canManageSettings} />
      )}
    </SettingsPageFrame>
  );
}
