import { TaskTemplatesSettings } from '@/components/pm/TaskTemplatesSettings';
import { SettingsPageFrame } from './SettingsPageFrame';

export function TaskTemplatesSettingsPage({ initialTeamId }: { initialTeamId?: string }) {
  return (
    <SettingsPageFrame section="task-templates">
      {({ workspaceId, permissions }) => (
        <TaskTemplatesSettings
          workspaceId={workspaceId}
          initialTeamId={initialTeamId}
          canManageSharedTemplates={permissions.isAdmin}
          managedTeamIds={permissions.teamMemberships
            .filter((membership) => membership.role === 'owner')
            .map((membership) => membership.team_id)}
        />
      )}
    </SettingsPageFrame>
  );
}
