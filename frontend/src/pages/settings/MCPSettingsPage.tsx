import { MCPSettingsPanel } from '@/components/settings/mcp/MCPSettingsPanel';
import { SettingsPageFrame } from './SettingsPageFrame';

export function MCPSettingsPage() {
  return (
    <SettingsPageFrame section="mcp" hideHeader>
      {({ workspaceId, currentWorkspaceName }) => (
        <MCPSettingsPanel workspaceId={workspaceId} workspaceName={currentWorkspaceName} />
      )}
    </SettingsPageFrame>
  );
}
