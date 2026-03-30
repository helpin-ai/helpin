import { HelpcenterTab } from '@/components/settings';
import { SettingsPageFrame } from './SettingsPageFrame';

export function HelpCenterSettingsPage() {
  return (
    <SettingsPageFrame section="helpcenter">
      {({ workspaceId, currentWorkspaceName }) => (
        <HelpcenterTab workspaceId={workspaceId} workspaceName={currentWorkspaceName} />
      )}
    </SettingsPageFrame>
  );
}
