import type { ReactNode } from 'react';
import { useTitle } from '@/hooks/useTitle';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { Message01Icon, Notification02Icon, Setting07Icon } from '@/lib/icons';
import {
  AccountNotificationPreferences,
  SupportNotificationCategoriesCard,
  WorkspaceMuteNotificationsCard,
  WorkspaceNotificationCategoriesCard,
} from '@/components/settings/NotificationPreferencesPanels';

const SETTINGS_CARD_CLASS = 'border-border/70 shadow-none';

function PageIntroItem({
  icon: Icon,
  title,
  description,
}: {
  icon: typeof Notification02Icon;
  title: string;
  description: string;
}) {
  return (
    <div className="flex min-w-0 gap-3">
      <div className="mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-md border bg-muted/40">
        <Icon className="h-4 w-4 text-muted-foreground" />
      </div>
      <div className="min-w-0">
        <p className="text-sm font-medium">{title}</p>
        <p className="mt-1 text-xs leading-5 text-muted-foreground">{description}</p>
      </div>
    </div>
  );
}

function SettingsSection({
  eyebrow,
  title,
  description,
  children,
}: {
  eyebrow: string;
  title: string;
  description: string;
  children: ReactNode;
}) {
  return (
    <section className="space-y-3">
      <div className="flex flex-col gap-1 border-b pb-3">
        <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-muted-foreground">{eyebrow}</span>
        <h3 className="text-base font-semibold">{title}</h3>
        <p className="max-w-3xl text-sm leading-6 text-muted-foreground">{description}</p>
      </div>
      {children}
    </section>
  );
}

export default function NotificationSettings() {
  useTitle('Notifications');
  const workspace = useWorkspaceStore((s) => s.currentWorkspace);
  const workspaceId = workspace?.id ?? '';
  const workspaceName = workspace?.name ?? 'this workspace';

  return (
    <div className="space-y-8">
      <div className="space-y-5">
        <div>
          <h2 className="text-xl font-semibold">Notifications</h2>
          <p className="mt-1 max-w-2xl text-sm leading-6 text-muted-foreground">
            Set your default delivery rules, then fine-tune what this workspace can send.
          </p>
        </div>

        <div className="grid gap-4 rounded-lg border bg-card p-4 md:grid-cols-3">
          <PageIntroItem
            icon={Setting07Icon}
            title="Account delivery"
            description="Pause notifications, email delivery, digest timing, and unread badge behavior."
          />
          <PageIntroItem
            icon={Notification02Icon}
            title="Workspace activity"
            description="Mute a workspace or choose which project events appear in-app and by email."
          />
          <PageIntroItem
            icon={Message01Icon}
            title="Support inbox"
            description="Tune alerts for customer replies and teammate collaboration."
          />
        </div>
      </div>

      <SettingsSection
        eyebrow="Account"
        title="Delivery defaults"
        description="These settings apply across every workspace before any workspace-specific preferences are considered."
      >
        <AccountNotificationPreferences cardClassName={SETTINGS_CARD_CLASS} />
      </SettingsSection>

      {workspaceId ? (
        <SettingsSection
          eyebrow="Workspace"
          title={workspaceName}
          description="Use these personal preferences to reduce noise from project and workspace activity."
        >
          <WorkspaceMuteNotificationsCard
            workspaceId={workspaceId}
            workspaceName={workspaceName}
            cardClassName={SETTINGS_CARD_CLASS}
          />
          <WorkspaceNotificationCategoriesCard
            workspaceId={workspaceId}
            cardClassName={SETTINGS_CARD_CLASS}
          />
        </SettingsSection>
      ) : null}

      {workspaceId ? (
        <SettingsSection
          eyebrow="Support"
          title="Inbox alerts"
          description={`Choose how customer and teammate activity from ${workspaceName} reaches you.`}
        >
          <SupportNotificationCategoriesCard
            workspaceId={workspaceId}
            cardClassName={SETTINGS_CARD_CLASS}
          />
        </SettingsSection>
      ) : null}
    </div>
  );
}
