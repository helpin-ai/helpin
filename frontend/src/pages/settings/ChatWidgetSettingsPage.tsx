import { ChatGeneralTab } from '@/components/settings';
import { SettingsPageFrame } from './SettingsPageFrame';

export function ChatWidgetSettingsPage() {
  return (
    <SettingsPageFrame section="chat-general">
      {({ workspaceId, permissions }) => (
        <ChatGeneralTab workspaceId={workspaceId} canManageSigningSecret={permissions.has('support.admin')} />
      )}
    </SettingsPageFrame>
  );
}
