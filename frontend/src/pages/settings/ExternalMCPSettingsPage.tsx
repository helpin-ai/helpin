import { ExternalMCPConnections } from '@/components/automation/ExternalMCPConnections';
import { SettingsPageFrame } from './SettingsPageFrame';

export function ExternalMCPSettingsPage() {
  return (
    <SettingsPageFrame section="external-mcp">
      {({ workspaceId, currentWorkspaceName, permissions }) => (
        <ExternalMCPConnections
          workspaceId={workspaceId}
          workspaceName={currentWorkspaceName}
          canManageSettings={permissions.canManageSettings}
        />
      )}
    </SettingsPageFrame>
  );
}
