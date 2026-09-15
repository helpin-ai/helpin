import { useNotificationPreferences } from '@/hooks/queries/useNotifications';
import { usePermissions, useWorkspaceAccess } from '@/hooks/queries/useSession';
import { SettingsSection } from '@/components/settings/SettingsSection';
import { Skeleton } from '@/components/ui/skeleton';
import { Button } from '@/components/ui/button';
import { useTitle } from '@/hooks/useTitle';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import {
  AccountNotificationPreferences,
  CRMNotificationCategoriesCard,
  IntegrationNotificationCategoriesCard,
  SupportNotificationCategoriesCard,
  WorkspaceMuteNotificationsCard,
  WorkspaceNotificationCategoriesCard,
} from '@/components/settings/NotificationPreferencesPanels';
import { QuietPageHeader, QuietSectionHeader } from '@/components/design-system/quiet';

export default function NotificationSettings() {
  useTitle('Notifications');
  const workspace = useWorkspaceStore((s) => s.currentWorkspace);

  const access = useWorkspaceAccess(workspace?.id ?? '');
  const permissions = usePermissions(access.data);
  const preferences = useNotificationPreferences(workspace?.id ?? '');

  return (
    <div className="space-y-7">
      <QuietPageHeader title="Notifications" description="Choose how you receive updates. Changes save automatically." />
      <SettingsSection title="General" description="Applies across all workspaces" defaultOpen>
        <AccountNotificationPreferences />
      </SettingsSection>
      {workspace?.id ? (
        <section aria-label="This workspace" className="space-y-3">
          <QuietSectionHeader title={<>This workspace <span className="font-normal normal-case tracking-normal text-quiet-text-secondary">· {workspace.name}</span></>} className="[&>div]:text-quiet-text-primary" />
          {preferences.isError ? <div role="alert" className="text-sm text-quiet-text-secondary">Couldn’t load workspace preferences. <Button variant="ghost" onClick={() => preferences.refetch()}>Try again</Button></div> : <>
          <WorkspaceMuteNotificationsCard workspaceId={workspace.id} />
          {access.isLoading ? <Skeleton className="h-40" /> : access.isError ? (
            <div role="alert" className="text-sm text-quiet-text-secondary">Couldn’t load your module access. <Button variant="ghost" onClick={() => access.refetch()}>Try again</Button></div>
          ) : <>
            {permissions.canAccessModule('support') && <SupportNotificationCategoriesCard workspaceId={workspace.id} />}
            {permissions.canAccessModule('pm') && <WorkspaceNotificationCategoriesCard workspaceId={workspace.id} />}
            {permissions.canAccessModule('crm') && <CRMNotificationCategoriesCard workspaceId={workspace.id} />}
            {permissions.has('settings.read') && <IntegrationNotificationCategoriesCard workspaceId={workspace.id} />}
          </>}
          </>}
        </section>
      ) : null}
    </div>
  );
}
