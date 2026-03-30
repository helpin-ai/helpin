import { CRMAutonomySettingsTab } from '@/components/settings';
import { SettingsPageFrame } from './SettingsPageFrame';

export function CRMAutonomySettingsPage() {
  return (
    <SettingsPageFrame section="crm-autonomy">
      {({ workspaceId }) => <CRMAutonomySettingsTab workspaceId={workspaceId} />}
    </SettingsPageFrame>
  );
}
