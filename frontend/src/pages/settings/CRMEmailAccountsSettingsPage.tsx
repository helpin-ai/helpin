import { CRMEmailSettingsTab } from '@/components/settings';
import { SettingsPageFrame } from './SettingsPageFrame';

export function CRMEmailAccountsSettingsPage() {
  return (
    <SettingsPageFrame section="crm-email">
      {({ workspaceId }) => <CRMEmailSettingsTab workspaceId={workspaceId} />}
    </SettingsPageFrame>
  );
}
