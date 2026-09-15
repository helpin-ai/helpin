import { type ReactNode } from 'react';
import { ArrowDown01Icon, InformationCircleIcon } from '@/lib/icons';
import { toast } from 'sonner';

import {
  useNotificationPreferences,
  useUpdateNotificationPreferences,
  useUserNotificationSettings,
  useUpdateUserNotificationSettings,
} from '@/hooks/queries';
import {
  SUPPORT_NOTIFICATION_CATEGORIES,
  WORKSPACE_NOTIFICATION_CATEGORIES,
  type NotificationCategory,
  type NotificationPreferences,
  type UpdateUserNotificationSettingsRequest,
} from '@/lib/notificationTypes';
import { cn } from '@/lib/utils';
import { QuickTooltip } from '@/components/ui/quick-tooltip';
import { pickerTriggerVariants } from '@/components/ui/picker-trigger';
import { QuietUnderlineInput } from '@/components/design-system/quiet';
import { QuietDropdown } from '@/components/design-system/quiet-dropdown';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/design-system/quiet-dropdown-select';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { Switch } from '@/components/ui/switch';

type CardClassNameProps = {
  cardClassName?: string;
};

const TIMEZONE_LIST: { id: string; offset: string; searchKey: string }[] = (() => {
  const names = Intl.supportedValuesOf('timeZone');
  const now = new Date();
  return names.map((tz) => {
    const fmt = new Intl.DateTimeFormat('en-US', { timeZone: tz, timeZoneName: 'shortOffset' });
    const parts = fmt.formatToParts(now);
    const gmtStr = parts.find((part) => part.type === 'timeZoneName')?.value ?? '';
    const offset = gmtStr === 'GMT' ? 'UTC+00:00' : gmtStr.replace('GMT', 'UTC');
    return { id: tz, offset, searchKey: `${tz} ${offset}`.toLowerCase() };
  });
})();

const WEEKDAY_OPTIONS = [
  { value: '0', label: 'Sunday' },
  { value: '1', label: 'Monday' },
  { value: '2', label: 'Tuesday' },
  { value: '3', label: 'Wednesday' },
  { value: '4', label: 'Thursday' },
  { value: '5', label: 'Friday' },
  { value: '6', label: 'Saturday' },
];

const DEFAULT_TIMEZONE = Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC';

function PreferenceHelp({ title, children }: { title: string; children: string }) {
  return (
    <QuickTooltip label={children}>
      <button type="button" aria-label={`About ${title.toLowerCase()}`} className="inline-flex size-5 shrink-0 items-center justify-center rounded-sm text-quiet-text-tertiary hover:text-quiet-text-primary focus-visible:outline-2 focus-visible:outline-ring">
        <InformationCircleIcon className="size-3.5" />
      </button>
    </QuickTooltip>
  );
}

function PreferenceRow({ title, description, children }: { title: string; description?: string; children: ReactNode }) {
  return (
    <div className="flex items-center justify-between gap-4 py-3">
      <div className="min-w-0">
        <p className="text-sm font-medium">{title}</p>
        {description ? <p className="mt-1 text-xs text-quiet-text-secondary">{description}</p> : null}
      </div>
      <div className="shrink-0">{children}</div>
    </div>
  );
}

function isActiveDnd(doNotDisturb?: boolean, dndUntil?: string | null) {
  if (dndUntil) {
    const date = new Date(dndUntil);
    if (!Number.isNaN(date.getTime())) {
      return date.getTime() > Date.now();
    }
  }
  return Boolean(doNotDisturb);
}

function TimezonePicker({
  value,
  disabled,
  onSelect,
}: {
  value: string;
  disabled?: boolean;
  onSelect: (timezone: string) => void;
}) {
  return (
    <QuietDropdown
      label="Timezone"
      selected={[value]}
      disabled={disabled}
      onSelect={onSelect}
      searchPlaceholder="Search timezones…"
      trigger={
        <button type="button" role="combobox" aria-label="Timezone" disabled={disabled} className={cn('flex h-9 w-full items-center justify-between gap-2 px-3 text-sm', pickerTriggerVariants({ variant: 'underline' }))}>
          <span className="truncate">{value}</span><ArrowDown01Icon className="size-3.5 shrink-0 text-quiet-text-tertiary" />
        </button>
      }
      groups={[{ id: 'timezones', options: TIMEZONE_LIST.map((tz) => ({ value: tz.id, label: tz.id, keywords: [tz.offset], content: <span className="flex w-full justify-between gap-3"><span>{tz.id}</span><span className="text-quiet-text-secondary">{tz.offset}</span></span> })) }]}
    />
  );
}

export function AccountNotificationPreferences({ cardClassName }: CardClassNameProps) {
  const { data: settings, isLoading, isError, refetch } = useUserNotificationSettings();
  const updateSettings = useUpdateUserNotificationSettings();

  const handleSettingsUpdate = (payload: UpdateUserNotificationSettingsRequest) => {
    updateSettings.mutate(payload, {
      onError: () => toast.error('Failed to update notification setting'),
    });
  };

  const handleToggle = (field: 'email_enabled', value: boolean) => {
    handleSettingsUpdate({ [field]: value });
  };

  const handleSelect = (field: 'email_digest_frequency' | 'badge_mode', value: string) => {
    handleSettingsUpdate({ [field]: value });
  };

  const handleDndToggle = (enabled: boolean) => {
    if (!enabled) {
      handleSettingsUpdate({ do_not_disturb: false, dnd_until: null });
      return;
    }

    handleSettingsUpdate({
      do_not_disturb: true,
      dnd_until: null,
    });
  };

  const handleDigestDayChange = (value: string) => {
    handleSettingsUpdate({ email_digest_day: Number(value) });
  };

  const handleDigestTimeChange = (value: string) => {
    handleSettingsUpdate({ email_digest_time: value });
  };

  const handleTimezoneChange = (value: string) => {
    handleSettingsUpdate({ timezone: value });
  };

  const digestFrequency = settings?.email_digest_frequency ?? 'daily';
  const emailEnabled = settings?.email_enabled ?? true;
  const dndActive = isActiveDnd(settings?.do_not_disturb, settings?.dnd_until);
  const usesDigestSchedule = digestFrequency === 'daily' || digestFrequency === 'weekly';

  if (isLoading) {
    return <Skeleton className="h-[420px] w-full" />;
  }
  if (isError) {
    return <div role="alert" className="text-sm text-quiet-text-secondary">Couldn’t load notification settings. <Button variant="ghost" onClick={() => refetch()}>Try again</Button></div>;
  }

  return (
    <div className={cardClassName}>
      <div className="divide-y divide-quiet-divider-strong">
        <PreferenceRow title="Pause all notifications" description="Pause in-app and email updates across all workspaces.">
          <Switch aria-label="Pause all notifications" checked={dndActive} disabled={updateSettings.isPending} onCheckedChange={handleDndToggle} />
        </PreferenceRow>
        <PreferenceRow title="Email notifications">
          <Switch aria-label="Email notifications" checked={emailEnabled} disabled={updateSettings.isPending} onCheckedChange={(value) => handleToggle('email_enabled', value)} />
        </PreferenceRow>
      </div>
      {emailEnabled ? (
        <div className="grid gap-x-6 gap-y-4 border-b border-quiet-divider-strong pb-5 pt-2 sm:grid-cols-2">
          <div className="space-y-2">
            <div className="flex items-center gap-1"><Label htmlFor="email-delivery">Email frequency</Label><PreferenceHelp title="Email frequency">Daily and weekly digests group routine updates. High-priority alerts can still arrive immediately.</PreferenceHelp></div>
            <Select value={digestFrequency} disabled={updateSettings.isPending} onValueChange={(value) => handleSelect('email_digest_frequency', value)}>
              <SelectTrigger id="email-delivery" variant="underline" className="w-full"><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="immediate">Immediately</SelectItem>
                <SelectItem value="daily">Daily digest</SelectItem>
                <SelectItem value="weekly">Weekly digest</SelectItem>
                <SelectItem value="never">High-priority only</SelectItem>
              </SelectContent>
            </Select>
          </div>
          {usesDigestSchedule ? <>
            <div className="space-y-2">
              <Label htmlFor="digest-time">Delivery time</Label>
              <QuietUnderlineInput id="digest-time" type="time" className="dark:[color-scheme:dark]" value={settings?.email_digest_time ?? '09:00'} disabled={updateSettings.isPending} onChange={(event) => handleDigestTimeChange(event.target.value)} />
            </div>
            {digestFrequency === 'weekly' ? (
              <div className="space-y-2">
                <Label htmlFor="digest-day">Delivery day</Label>
                <Select value={String(settings?.email_digest_day ?? 1)} disabled={updateSettings.isPending} onValueChange={handleDigestDayChange}>
                  <SelectTrigger id="digest-day" variant="underline" className="w-full"><SelectValue /></SelectTrigger>
                  <SelectContent>{WEEKDAY_OPTIONS.map((option) => <SelectItem key={option.value} value={option.value}>{option.label}</SelectItem>)}</SelectContent>
                </Select>
              </div>
            ) : null}
            <div className="space-y-2">
              <Label>Timezone</Label>
              <TimezonePicker value={settings?.timezone ?? DEFAULT_TIMEZONE} disabled={updateSettings.isPending} onSelect={handleTimezoneChange} />
            </div>
          </> : null}
        </div>
      ) : null}
      <div className="flex flex-wrap items-center justify-between gap-3 pt-4">
        <div className="flex items-center gap-1"><Label htmlFor="unread-badge">Unread badge</Label><PreferenceHelp title="Unread badge">Choose which notifications count toward the unread badge.</PreferenceHelp></div>
        <Select value={settings?.badge_mode ?? 'all'} disabled={updateSettings.isPending} onValueChange={(value) => handleSelect('badge_mode', value)}>
          <SelectTrigger id="unread-badge" variant="underline" className="w-48"><SelectValue /></SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All notifications</SelectItem>
            <SelectItem value="mentions_only">Mentions only</SelectItem>
            <SelectItem value="none">No badge</SelectItem>
          </SelectContent>
        </Select>
      </div>
    </div>
  );
}

export function WorkspaceMuteNotificationsCard({
  workspaceId,
  cardClassName,
}: CardClassNameProps & { workspaceId: string }) {
  const { data: prefs, isLoading } = useNotificationPreferences(workspaceId);
  const { data: accountSettings } = useUserNotificationSettings();
  const updatePrefs = useUpdateNotificationPreferences(workspaceId);

  const handleMuteToggle = (value: boolean) => {
    updatePrefs.mutate({ mute_workspace: value }, {
      onError: () => toast.error('Failed to update notification preference'),
    });
  };

  if (!workspaceId) {
    return null;
  }

  if (isLoading) {
    return <Skeleton className="h-32 w-full" />;
  }

  const paused = isActiveDnd(accountSettings?.do_not_disturb, accountSettings?.dnd_until);
  const notice = paused
    ? 'All notifications are paused. Your choices below will apply when you resume.'
    : prefs?.mute_workspace
      ? 'This workspace is muted. Your choices below will apply when you unmute.'
      : accountSettings?.email_enabled === false
        ? 'Email notifications are off. Your email choices below are saved for when you turn them on.'
        : null;

  return (
    <div className={cardClassName}>
      <PreferenceRow title="Mute this workspace" description="Only affects your notifications, including support alerts.">
        <Switch aria-label="Mute this workspace" checked={prefs?.mute_workspace ?? false} disabled={updatePrefs.isPending} onCheckedChange={handleMuteToggle} />
      </PreferenceRow>
      {notice ? <p role="status" className="pt-1 text-xs text-quiet-accent">{notice}</p> : null}
    </div>
  );
}

const isChannelEnabled = (
  prefs: NotificationPreferences | undefined,
  categoryKey: string,
  channel: 'in_app' | 'email',
) => prefs?.channel_preferences?.[categoryKey]?.[channel] ?? true;

function NotificationCategoriesCard({
  workspaceId,
  cardClassName,
  title,
  categories,
}: CardClassNameProps & {
  workspaceId: string;
  title: string;
  categories: NotificationCategory[];
}) {
  const { data: prefs, isLoading } = useNotificationPreferences(workspaceId);
  const updatePrefs = useUpdateNotificationPreferences(workspaceId);

  const handleToggle = (categoryKey: string, channel: 'in_app' | 'email', value: boolean) => {
    const nextPreferences = { ...(prefs?.channel_preferences ?? {}) };
    nextPreferences[categoryKey] = {
      ...(nextPreferences[categoryKey] ?? {}),
      [channel]: value,
    };

    updatePrefs.mutate({ channel_preferences: nextPreferences }, {
      onError: () => toast.error('Failed to update notification preference'),
    });
  };

  if (!workspaceId) {
    return null;
  }

  if (isLoading) {
    return <Skeleton className="h-80 w-full" />;
  }

  return (
    <section aria-label={title} className={cn('space-y-2', cardClassName)}>
      <h3 className="text-sm font-semibold text-quiet-text-primary">{title}</h3>
      <div>
        <div className="grid grid-cols-[minmax(0,1fr)_56px_56px] items-center gap-2 border-b border-quiet-divider-strong py-2 text-xs text-quiet-text-secondary sm:grid-cols-[minmax(0,1fr)_80px_80px]">
          <span><span className="sr-only">Activity type</span></span>
          <span className="text-center">In-app</span><span className="text-center">Email</span>
        </div>
        <div className="divide-y divide-quiet-divider-strong">
          {categories.map((category) => (
            <div key={category.key} className="grid grid-cols-[minmax(0,1fr)_56px_56px] items-center gap-2 py-3 sm:grid-cols-[minmax(0,1fr)_80px_80px]">
              <div className="flex min-w-0 items-center gap-1">
                <span className="text-sm">{category.label}</span>
                <PreferenceHelp title={category.label}>
                  {category.description + (category.key === 'support_replies' ? '. Email alerts arrive after 3 minutes if the conversation is still unread and unanswered.' : '.')}
                </PreferenceHelp>
              </div>
              <div className="flex justify-center">
                <Switch aria-label={`${category.label}: in-app`} checked={isChannelEnabled(prefs, category.key, 'in_app')} disabled={updatePrefs.isPending} onCheckedChange={(value) => handleToggle(category.key, 'in_app', value)} />
              </div>
              <div className="flex justify-center">
                {category.supportsEmail === false ? <span aria-label="Email not available" className="text-xs text-quiet-text-tertiary">—</span> : (
                  <Switch aria-label={`${category.label}: email`} checked={isChannelEnabled(prefs, category.key, 'email')} disabled={updatePrefs.isPending} onCheckedChange={(value) => handleToggle(category.key, 'email', value)} />
                )}
              </div>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}

export function WorkspaceNotificationCategoriesCard({
  workspaceId,
  cardClassName,
}: CardClassNameProps & { workspaceId: string }) {
  return (
    <NotificationCategoriesCard
      workspaceId={workspaceId}
      cardClassName={cardClassName}
      title="Projects & docs"
      categories={WORKSPACE_NOTIFICATION_CATEGORIES}
    />
  );
}

export function SupportNotificationCategoriesCard({
  workspaceId,
  cardClassName,
}: CardClassNameProps & { workspaceId: string }) {
  return (
    <NotificationCategoriesCard
      workspaceId={workspaceId}
      cardClassName={cardClassName}
      title="Support inbox"
      categories={SUPPORT_NOTIFICATION_CATEGORIES}
    />
  );
}

export function CRMNotificationCategoriesCard({ workspaceId }: { workspaceId: string }) {
  return <NotificationCategoriesCard workspaceId={workspaceId} title="CRM" categories={[{ key: 'crm_signals', label: 'Signals ready for review', description: 'Signals routed to you or your team' }]} />;
}

export function IntegrationNotificationCategoriesCard({ workspaceId }: { workspaceId: string }) {
  return <NotificationCategoriesCard workspaceId={workspaceId} title="Integrations" categories={[{ key: 'integrations', label: 'Connection issues', description: 'External tools that need you to reconnect', supportsEmail: false }]} />;
}
