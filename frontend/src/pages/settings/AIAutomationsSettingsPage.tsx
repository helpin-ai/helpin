import { AIAutomationsTab } from '@/components/settings';
import { SettingsPageFrame } from './SettingsPageFrame';

export function AIAutomationsSettingsPage() {
  return (
    <SettingsPageFrame section="ai-automations">
      {({ workspaceId, permissions }) => {
        if (!permissions.canManageSettings) {
          return (
            <div className="rounded-none border border-border bg-card px-6 py-8 text-sm text-muted-foreground">
              You do not have permission to view the shared AI & Automations governance surface.
            </div>
          );
        }

        return <AIAutomationsTab workspaceId={workspaceId} />;
      }}
    </SettingsPageFrame>
  );
}
