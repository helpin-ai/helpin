import { CRMMeetingSettingsTab } from '@/components/settings/CRMMeetingSettingsTab';
import { SettingsPageFrame } from './SettingsPageFrame';

export function CRMMeetingSettingsPage() {
  return (
    <SettingsPageFrame section="crm-meetings">
      {({ workspaceId }) => <CRMMeetingSettingsTab workspaceId={workspaceId} />}
    </SettingsPageFrame>
  );
}
