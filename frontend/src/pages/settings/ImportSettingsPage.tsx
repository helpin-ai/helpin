import { ImportTab } from '@/components/settings';
import { SettingsPageFrame } from './SettingsPageFrame';

export function ImportSettingsPage() {
  return (
    <SettingsPageFrame section="import">
      {({ workspaceId, permissions }) => (
        <ImportTab workspaceId={workspaceId} editable={permissions.canImport} />
      )}
    </SettingsPageFrame>
  );
}
