import { CRMMeetingSettingsTab } from '@/components/settings/CRMMeetingSettingsTab';
import { SettingsPageFrame } from './SettingsPageFrame';

export function CRMMeetingSettingsPage() {
  return (
    <SettingsPageFrame section="crm-meetings">
      {({ workspaceId, permissions }) => <CRMMeetingSettingsTab workspaceId={workspaceId} canManage={permissions.has('crm.admin')} />}
    </SettingsPageFrame>
  );
}
