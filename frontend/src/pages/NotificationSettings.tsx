import { MODULE_NOTIFICATION_SECTIONS } from '@/lib/slackNotificationRows';
import { useNotificationPreferences } from '@/hooks/queries/useNotifications';
import { usePermissions, useWorkspaceAccess } from '@/hooks/queries/useSession';
import { SlackConnectionNotice, useSlackNotificationSettings } from '@/components/settings/SlackNotificationSettings';
import { SettingsSection } from '@/components/settings/SettingsSection';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Skeleton } from '@/components/ui/skeleton';
import { Button } from '@/components/ui/button';
import { useTitle } from '@/hooks/useTitle';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import {
  AccountNotificationPreferences,
  NotificationCategoriesCard,
  WorkspaceMuteNotificationsCard,
} from '@/components/settings/NotificationPreferencesPanels';
import { QuietPageHeader } from '@/components/design-system/quiet';

export default function NotificationSettings({ slackRow }: { slackRow?: string }) {
  useTitle('Notifications');
  const workspace = useWorkspaceStore((s) => s.currentWorkspace);

  const slack = useSlackNotificationSettings(workspace?.id ?? '');
  const access = useWorkspaceAccess(workspace?.id ?? '');
  const permissions = usePermissions(access.data);
  const preferences = useNotificationPreferences(workspace?.id ?? '');

  return (
    <div className="space-y-7">
      <QuietPageHeader title="Notifications" description="Choose how you receive updates. Changes save automatically." />
      <Tabs key={slackRow ?? 'account'} defaultValue={slackRow ? 'workspace' : 'account'} className="gap-5">
        <TabsList variant="quiet" aria-label="Notification scope">
          <TabsTrigger value="account">Account</TabsTrigger>
          <TabsTrigger value="workspace" disabled={!workspace?.id}>Workspace</TabsTrigger>
        </TabsList>
        <TabsContent value="account" forceMount className="data-[state=inactive]:hidden">
          <SettingsSection title="General" description="Applies across all workspaces" defaultOpen>
            <AccountNotificationPreferences />
          </SettingsSection>
        </TabsContent>
        <TabsContent value="workspace" forceMount className="data-[state=inactive]:hidden">
          {workspace?.id ? (
            <section aria-label="This workspace" className="space-y-3">
              {preferences.isError ? <div role="alert" className="text-sm text-quiet-text-secondary">Couldn’t load workspace preferences. <Button variant="ghost" onClick={() => preferences.refetch()}>Try again</Button></div> : <>
              <WorkspaceMuteNotificationsCard workspaceId={workspace.id} />
              {access.isLoading ? <Skeleton className="h-40" /> : access.isError ? (
                <div role="alert" className="text-sm text-quiet-text-secondary">Couldn’t load your module access. <Button variant="ghost" onClick={() => access.refetch()}>Try again</Button></div>
              ) : <>
                <SlackConnectionNotice controls={slack} slug={workspace.slug} canManage={permissions.has('settings.manage')} />
                {MODULE_NOTIFICATION_SECTIONS.filter(section => section.module === 'integrations'
                  ? permissions.has('settings.read')
                  : permissions.canAccessModule(section.module) || (section.module === 'agents' && Boolean(access.data?.can_use_ask_agents || permissions.canAccessModule('pm')))).map(section =>
                    <NotificationCategoriesCard key={section.module} workspaceId={workspace.id} title={section.title}
                      categories={section.rows} target={slackRow} controls={slack} />
                  )}
              </>}
              </>}
            </section>
          ) : null}
        </TabsContent>
      </Tabs>
    </div>
  );
}
