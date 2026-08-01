import { MCPAccessPanel } from '@/components/settings/mcp/MCPAccessPanel';
import { SettingsPageFrame } from './SettingsPageFrame';

export function MCPSettingsPage() {
  return (
    <SettingsPageFrame section="mcp" hideHeader>
      {({ workspaceId, currentWorkspaceName }) => (
        <MCPAccessPanel workspaceId={workspaceId} workspaceName={currentWorkspaceName} />
      )}
    </SettingsPageFrame>
  );
}
