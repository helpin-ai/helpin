import { TaskTemplatesSettings } from '@/components/pm/TaskTemplatesSettings';
import { SettingsPageFrame } from './SettingsPageFrame';

export function TaskTemplatesSettingsPage({ initialTeamId }: { initialTeamId?: string }) {
  return (
    <SettingsPageFrame section="task-templates">
      {({ workspaceId }) => (
        <TaskTemplatesSettings workspaceId={workspaceId} initialTeamId={initialTeamId} />
      )}
    </SettingsPageFrame>
  );
}
