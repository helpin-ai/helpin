import { useState } from 'react';
import { ExternalAgentsAddButton, ExternalAgentsSettings } from '@/components/automation/ExternalAgentsSettings';
import { SettingsPageFrame } from './SettingsPageFrame';

export function ExternalAgentsSettingsPage() {
  const [createOpen, setCreateOpen] = useState(false);
  return (
    <SettingsPageFrame
      section="external-agents"
      headerAction={({ workspaceId, permissions }) => permissions.canManageSettings ? (
        <ExternalAgentsAddButton workspaceId={workspaceId} onAdd={() => setCreateOpen(true)} />
      ) : null}
    >
      {({ workspaceId, permissions }) => (
        <ExternalAgentsSettings
          workspaceId={workspaceId}
          canManageSettings={permissions.canManageSettings}
          createOpen={createOpen}
          onCreateOpenChange={setCreateOpen}
        />
      )}
    </SettingsPageFrame>
  );
}
