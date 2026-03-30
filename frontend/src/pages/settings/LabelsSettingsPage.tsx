import { LabelsSettings } from '@/components/pm/LabelsSettings';
import { SettingsPageFrame } from './SettingsPageFrame';

export function LabelsSettingsPage({ initialTeamId }: { initialTeamId?: string }) {
  return (
    <SettingsPageFrame section="labels">
      {({ workspaceId, permissions }) => (
        <LabelsSettings workspaceId={workspaceId} initialTeamId={initialTeamId} editable={permissions.canAdminLabels} />
      )}
    </SettingsPageFrame>
  );
}
