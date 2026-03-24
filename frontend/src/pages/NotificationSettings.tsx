import { useTitle } from '@/hooks/useTitle';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import {
  AccountNotificationPreferences,
  SupportNotificationCategoriesCard,
  WorkspaceMuteNotificationsCard,
  WorkspaceNotificationCategoriesCard,
} from '@/components/settings/NotificationPreferencesPanels';

export default function NotificationSettings() {
  useTitle('Notifications');
  const workspace = useWorkspaceStore((s) => s.currentWorkspace);
  const workspaceId = workspace?.id ?? '';

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-xl font-semibold">Notifications</h2>
        <p className="text-sm text-muted-foreground mt-1">
          Manage account-wide delivery and workspace-specific notification preferences here.
        </p>
      </div>

      <section className="space-y-4">
        <div>
          <h3 className="text-sm font-semibold uppercase tracking-[0.12em] text-muted-foreground">Account</h3>
          <p className="mt-1 text-sm text-muted-foreground">
            Controls how notifications reach you across the product.
          </p>
        </div>
        <AccountNotificationPreferences />
      </section>

      {workspaceId ? (
        <section className="space-y-4">
          <div>
            <h3 className="text-sm font-semibold uppercase tracking-[0.12em] text-muted-foreground">This Workspace</h3>
            <p className="mt-1 text-sm text-muted-foreground">
              Personal overrides for activity from {workspace?.name ?? 'this workspace'}.
            </p>
          </div>
          <WorkspaceMuteNotificationsCard
            workspaceId={workspaceId}
            workspaceName={workspace?.name}
          />
          <WorkspaceNotificationCategoriesCard
            workspaceId={workspaceId}
          />
        </section>

      ) : null}

      {workspaceId ? (
        <section className="space-y-4">
          <div>
            <h3 className="text-sm font-semibold uppercase tracking-[0.12em] text-muted-foreground">Support Inbox</h3>
            <p className="mt-1 text-sm text-muted-foreground">
              Personal support-inbox alerts for {workspace?.name ?? 'this workspace'}.
            </p>
          </div>
          <SupportNotificationCategoriesCard
            workspaceId={workspaceId}
          />
        </section>
      ) : null}
    </div>
  );
}
