import { useTitle } from '@/hooks/useTitle';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import {
  AccountNotificationPreferences,
  SupportNotificationCategoriesCard,
  WorkspaceMuteNotificationsCard,
  WorkspaceNotificationCategoriesCard,
} from '@/components/settings/NotificationPreferencesPanels';
import { QuietPageHeader, QuietSectionHeader } from '@/components/design-system/quiet';

export default function NotificationSettings() {
  useTitle('Notifications');
  const workspace = useWorkspaceStore((s) => s.currentWorkspace);

  return (
    <div className="space-y-7">
      <QuietPageHeader title="Notifications" description="Choose how you receive updates. Changes save automatically." />
      <section aria-label="All workspaces" className="space-y-3">
        <QuietSectionHeader title="All workspaces" className="[&>div]:text-quiet-text-primary" />
        <AccountNotificationPreferences />
      </section>
      {workspace?.id ? (
        <section aria-label="This workspace" className="space-y-5 border-t border-quiet-divider-strong pt-5">
          <QuietSectionHeader title={<>This workspace <span className="font-normal normal-case tracking-normal text-quiet-text-secondary">· {workspace.name}</span></>} className="[&>div]:text-quiet-text-primary" />
          <WorkspaceMuteNotificationsCard workspaceId={workspace.id} />
          <SupportNotificationCategoriesCard workspaceId={workspace.id} />
          <WorkspaceNotificationCategoriesCard workspaceId={workspace.id} />
        </section>
      ) : null}
    </div>
  );
}
