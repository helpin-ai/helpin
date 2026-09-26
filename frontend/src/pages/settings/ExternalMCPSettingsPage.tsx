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
      {({ workspaceId, currentWorkspaceSlug, permissions }) => (
        <ExternalMCPConnections
          workspaceId={workspaceId}
          workspaceSlug={currentWorkspaceSlug}
          canManageSettings={permissions.canManageSettings}
          canEditCustomAgents={permissions.hasAny('pm.edit', 'docs.edit', 'crm.edit', 'support.edit')}
          canEditPresetAgents={permissions.has('pm.edit')}
          createOpen={createOpen}
          onCreateOpenChange={setCreateOpen}
        />
      )}
    </SettingsPageFrame>
  );
}
