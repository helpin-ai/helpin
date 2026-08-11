import { useState } from 'react';
import { ExternalMCPAddButton, ExternalMCPConnections } from '@/components/automation/ExternalMCPConnections';
import { SettingsPageFrame } from './SettingsPageFrame';

export function ExternalMCPSettingsPage() {
  const [createOpen, setCreateOpen] = useState(false);
  return (
    <SettingsPageFrame
      section="external-mcp"
      headerAction={({ workspaceId, permissions }) => permissions.canManageSettings ? (
        <ExternalMCPAddButton workspaceId={workspaceId} onAdd={() => setCreateOpen(true)} />
      ) : null}
    >
      {({ workspaceId, currentWorkspaceName, permissions }) => (
        <ExternalMCPConnections
          workspaceId={workspaceId}
          workspaceName={currentWorkspaceName}
          canManageSettings={permissions.canManageSettings}
          createOpen={createOpen}
          onCreateOpenChange={setCreateOpen}
        />
      )}
    </SettingsPageFrame>
  );
}
