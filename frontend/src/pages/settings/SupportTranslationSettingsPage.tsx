import { SupportTranslationSettings } from '@/components/settings/SupportTranslationSettings';
import { SettingsPageFrame } from './SettingsPageFrame';

export function SupportTranslationSettingsPage() {
  return (
    <SettingsPageFrame section="support-translation">
      {({ workspaceId, permissions }) => (
        <SupportTranslationSettings
          workspaceId={workspaceId}
          editable={permissions.permissionSet.has('support.admin')}
        />
      )}
    </SettingsPageFrame>
  );
}
