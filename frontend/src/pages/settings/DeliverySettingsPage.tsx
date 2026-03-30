import { ProjectDeliveryTab } from '@/components/settings';
import { SettingsPageFrame } from './SettingsPageFrame';

export function DeliverySettingsPage() {
  return (
    <SettingsPageFrame section="delivery">
      {({ workspaceId, permissions }) => (
        <ProjectDeliveryTab workspaceId={workspaceId} editable={permissions.canManageSettings} />
      )}
    </SettingsPageFrame>
  );
}
