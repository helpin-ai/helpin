import { RecurringTemplatesSettings } from '@/components/pm/RecurringTemplatesSettings';
import { SettingsPageFrame } from './SettingsPageFrame';

export function RecurringTasksSettingsPage({ initialTeamId }: { initialTeamId?: string }) {
  return (
    <SettingsPageFrame section="recurring-tasks">
      {({ workspaceId, permissions }) => (
        <RecurringTemplatesSettings workspaceId={workspaceId} initialTeamId={initialTeamId} editable={permissions.canEdit} />
      )}
    </SettingsPageFrame>
  );
}
