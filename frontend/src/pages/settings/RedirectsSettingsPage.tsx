import { RedirectsTab } from '@/components/settings';
import { SettingsPageFrame } from './SettingsPageFrame';

export function RedirectsSettingsPage() {
  return (
    <SettingsPageFrame section="redirects">
      {({ workspaceId, permissions }) => (
        <RedirectsTab workspaceId={workspaceId} editable={permissions.canManageSettings} />
      )}
    </SettingsPageFrame>
  );
}
