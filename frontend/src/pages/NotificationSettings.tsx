import { useTitle } from '@/hooks/useTitle';
import { AccountNotificationPreferences } from '@/components/settings/NotificationPreferencesPanels';

export default function NotificationSettings() {
  useTitle('Notifications');

  return (
    <div className="space-y-4">
      <div>
        <h2 className="text-xl font-semibold">Notifications</h2>
        <p className="text-sm text-muted-foreground mt-1">
          Manage notification delivery for your account across all workspaces. Workspace-specific mute is available in workspace settings.
        </p>
      </div>
      <AccountNotificationPreferences />
    </div>
  );
}
