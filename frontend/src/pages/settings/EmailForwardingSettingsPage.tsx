import { SupportEmailForwardingTab } from '@/components/settings';
import { SettingsPageFrame } from './SettingsPageFrame';

export function EmailForwardingSettingsPage() {
  return (
    <SettingsPageFrame section="email-forwarding">
      {({ workspaceId }) => <SupportEmailForwardingTab workspaceId={workspaceId} />}
    </SettingsPageFrame>
  );
}
