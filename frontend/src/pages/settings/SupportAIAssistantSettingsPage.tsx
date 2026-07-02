import { ChatGeneralTab } from '@/components/settings';
import { SettingsPageFrame } from './SettingsPageFrame';

export function SupportAIAssistantSettingsPage() {
  return (
    <SettingsPageFrame section="support-ai-assistant">
      {({ workspaceId }) => <ChatGeneralTab workspaceId={workspaceId} mode="ai-assistant" />}
    </SettingsPageFrame>
  );
}
