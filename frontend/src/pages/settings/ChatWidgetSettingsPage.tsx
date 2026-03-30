import { ChatGeneralTab } from '@/components/settings';
import { SettingsPageFrame } from './SettingsPageFrame';

export function ChatWidgetSettingsPage() {
  return (
    <SettingsPageFrame section="chat-general">
      {({ workspaceId }) => <ChatGeneralTab workspaceId={workspaceId} />}
    </SettingsPageFrame>
  );
}
