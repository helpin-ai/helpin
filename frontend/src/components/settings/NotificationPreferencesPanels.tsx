import { useMemo, useState, type ReactNode } from 'react';
import { Notification02Icon, NotificationOff02Icon, ArrowRight01Icon, GlobeIcon, Mail01Icon, Search01Icon } from '@/lib/icons';
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
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
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

function StatusBadge({ active, children }: { active: boolean; children: string }) {
  return (
    <Badge
      variant="outline"
      className={cn(
        'h-6 rounded-md px-2 font-medium',
        active
          ? 'border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-900/70 dark:bg-emerald-950/30 dark:text-emerald-300'
          : 'border-muted-foreground/20 bg-muted/40 text-muted-foreground',
      )}
    >
      {children}
    </Badge>
  );
}

function PreferenceRow({
  title,
  description,
  children,
  className,
}: {
  title: string;
  description: string;
  children: ReactNode;
  className?: string;
}) {
  return (
    <div className={cn('flex flex-col gap-4 rounded-md border bg-background p-4 sm:flex-row sm:items-center sm:justify-between', className)}>
      <div className="min-w-0">
        <p className="text-sm font-medium">{title}</p>
        <p className="mt-1 text-xs leading-5 text-muted-foreground">{description}</p>
      </div>
      <div className="shrink-0">{children}</div>
    </div>
  );
}

function digestLabel(value: string) {
  switch (value) {
    case 'immediate':
      return 'Immediate';
    case 'daily':
      return 'Daily digest';
    case 'weekly':
      return 'Weekly digest';
    case 'never':
      return 'Never';
    default:
      return value;
  }
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
  const [search, setSearch] = useState('');
  const selectedTz = useMemo(() => TIMEZONE_LIST.find((tz) => tz.id === value), [value]);
  const filteredTimezones = useMemo(() => {
    if (!search) return TIMEZONE_LIST;
    const query = search.toLowerCase();
    return TIMEZONE_LIST.filter((tz) => tz.searchKey.includes(query));
  }, [search]);

  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button variant="outline" className="w-full justify-between font-normal" disabled={disabled}>
          <span className="flex items-center gap-2 overflow-hidden">
            <GlobeIcon className="h-4 w-4 shrink-0 text-muted-foreground" />
            <span className="truncate">{value}</span>
            {selectedTz ? <span className="truncate text-muted-foreground">({selectedTz.offset})</span> : null}
          </span>
          <ArrowRight01Icon className="h-4 w-4 shrink-0 text-muted-foreground" />
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-[320px] p-0" align="end">
        <div className="border-b p-2">
          <div className="flex items-center gap-2 px-2">
            <Search01Icon className="h-4 w-4 shrink-0 text-muted-foreground" />
            <input
              className="flex-1 bg-transparent text-sm outline-none placeholder:text-muted-foreground"
              placeholder="Search timezones..."
              value={search}
              onChange={(event) => setSearch(event.target.value)}
            />
          </div>
        </div>
        <div className="max-h-[280px] overflow-y-auto p-1">
          {filteredTimezones.length === 0 ? (
            <p className="py-4 text-center text-xs text-muted-foreground">No timezones found</p>
          ) : (
            filteredTimezones.map((tz) => (
              <button
                key={tz.id}
                type="button"
                className={cn(
                  'flex w-full items-center justify-between rounded-sm px-2 py-1.5 text-left text-sm hover:bg-accent',
                  tz.id === value && 'bg-accent font-medium',
                )}
                onClick={() => {
                  onSelect(tz.id);
                  setSearch('');
                }}
              >
                <span>{tz.id}</span>
                <span className="ml-2 shrink-0 text-xs text-muted-foreground">{tz.offset}</span>
              </button>
            ))
          )}
        </div>
      </PopoverContent>
    </Popover>
  );
}

export function AccountNotificationPreferences({ cardClassName }: CardClassNameProps) {
  const { data: settings, isLoading } = useUserNotificationSettings();
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

  return (
    <Card className={cardClassName}>
      <CardHeader className="space-y-3">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
          <div>
            <CardTitle className="flex items-center gap-2 text-base">
              <Mail01Icon className="h-4 w-4" />
              Delivery rules
            </CardTitle>
            <CardDescription className="mt-1">
              Account-level settings decide whether notifications can reach you outside the app.
            </CardDescription>
          </div>
          <div className="flex flex-wrap gap-2">
            <StatusBadge active={!dndActive}>{dndActive ? 'Paused' : 'Active'}</StatusBadge>
            <StatusBadge active={emailEnabled}>{emailEnabled ? digestLabel(digestFrequency) : 'Email off'}</StatusBadge>
          </div>
        </div>
      </CardHeader>
      <CardContent className="space-y-4">
        <PreferenceRow
          title="Pause notifications"
          description="Stop new in-app and email notifications across every workspace until you turn this off."
        >
          <Switch
            checked={dndActive}
            disabled={updateSettings.isPending}
            onCheckedChange={handleDndToggle}
          />
        </PreferenceRow>

        <PreferenceRow
          title="Email notifications"
          description="Allow Helpin to send notification emails. Category-specific email choices are set per workspace below."
        >
          <Switch
            checked={emailEnabled}
            disabled={updateSettings.isPending}
            onCheckedChange={(value) => handleToggle('email_enabled', value)}
          />
        </PreferenceRow>

        <div className="rounded-md border bg-background p-4">
          <div className="grid gap-4 md:grid-cols-2">
            <div className="space-y-2">
              <Label>Email delivery</Label>
              <Select
                value={digestFrequency}
                disabled={updateSettings.isPending || !emailEnabled}
                onValueChange={(value) => handleSelect('email_digest_frequency', value)}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="immediate">Immediate</SelectItem>
                  <SelectItem value="daily">Daily digest</SelectItem>
                  <SelectItem value="weekly">Weekly digest</SelectItem>
                  <SelectItem value="never">Never</SelectItem>
                </SelectContent>
              </Select>
              <p className="text-xs text-muted-foreground">
                Daily and weekly options batch normal activity into one digest.
              </p>
            </div>

            <div className="space-y-2">
              <Label>Badge mode</Label>
              <Select
                value={settings?.badge_mode ?? 'all'}
                disabled={updateSettings.isPending}
                onValueChange={(value) => handleSelect('badge_mode', value)}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">All notifications</SelectItem>
                  <SelectItem value="mentions_only">Mentions only</SelectItem>
                  <SelectItem value="none">No unread badge</SelectItem>
                </SelectContent>
              </Select>
              <p className="text-xs text-muted-foreground">Choose which notifications count toward the unread badge.</p>
            </div>
          </div>

          {usesDigestSchedule ? (
            <div className="mt-4 space-y-4 border-t pt-4">
              <div className="grid gap-4 md:grid-cols-2">
                <div className="space-y-2">
                  <Label htmlFor="digest-time">Digest time</Label>
                  <Input
                    id="digest-time"
                    type="time"
                    value={settings?.email_digest_time ?? '09:00'}
                    disabled={updateSettings.isPending || !emailEnabled}
                    onChange={(event) => handleDigestTimeChange(event.target.value)}
                  />
                  <p className="text-xs text-muted-foreground">Used for scheduled digests in your selected timezone.</p>
                </div>

                {digestFrequency === 'weekly' ? (
                  <div className="space-y-2">
                    <Label>Weekly digest day</Label>
                    <Select
                      value={String(settings?.email_digest_day ?? 1)}
                      disabled={updateSettings.isPending || !emailEnabled}
                      onValueChange={handleDigestDayChange}
                    >
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        {WEEKDAY_OPTIONS.map((option) => (
                          <SelectItem key={option.value} value={option.value}>{option.label}</SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                    <p className="text-xs text-muted-foreground">Choose the day for your weekly digest.</p>
                  </div>
                ) : null}
              </div>

              <div className="space-y-2">
                <Label>Digest timezone</Label>
                <TimezonePicker
                  value={settings?.timezone ?? DEFAULT_TIMEZONE}
                  disabled={updateSettings.isPending || !emailEnabled}
                  onSelect={handleTimezoneChange}
                />
                <p className="text-xs text-muted-foreground">Scheduled digests and DND resume times use this timezone.</p>
              </div>
            </div>
          ) : null}
        </div>
      </CardContent>
    </Card>
  );
}

export function WorkspaceMuteNotificationsCard({
  workspaceId,
  workspaceName,
  cardClassName,
}: CardClassNameProps & { workspaceId: string; workspaceName?: string }) {
  const { data: prefs, isLoading } = useNotificationPreferences(workspaceId);
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

  return (
    <Card className={cardClassName}>
      <CardHeader className="space-y-3">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
          <div>
            <CardTitle className="flex items-center gap-2 text-base">
              <NotificationOff02Icon className="h-4 w-4" />
              Workspace mute
            </CardTitle>
            <CardDescription className="mt-1">
              A single personal override for all notifications from this workspace.
            </CardDescription>
          </div>
          <StatusBadge active={!(prefs?.mute_workspace ?? false)}>
            {prefs?.mute_workspace ? 'Muted' : 'Delivering'}
          </StatusBadge>
        </div>
      </CardHeader>
      <CardContent>
        <PreferenceRow
          title={`Mute ${workspaceName ?? 'this workspace'}`}
          description="Keep the category settings below saved, but stop delivery until mute is turned off."
        >
          <Switch
            checked={prefs?.mute_workspace ?? false}
            disabled={updatePrefs.isPending}
            onCheckedChange={handleMuteToggle}
          />
        </PreferenceRow>
      </CardContent>
    </Card>
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
  description,
  categories,
  mutedWorkspaceMessage,
  emailDisabledMessage,
  extraNote,
}: CardClassNameProps & {
  workspaceId: string;
  title: string;
  description: string;
  categories: NotificationCategory[];
  mutedWorkspaceMessage: string;
  emailDisabledMessage: string;
  extraNote?: string;
}) {
  const { data: prefs, isLoading } = useNotificationPreferences(workspaceId);
  const { data: accountSettings } = useUserNotificationSettings();
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

  const workspaceMuted = prefs?.mute_workspace ?? false;
  const emailEnabled = accountSettings?.email_enabled ?? true;
  const enabledInAppCount = categories.filter((category) => isChannelEnabled(prefs, category.key, 'in_app')).length;
  const enabledEmailCount = categories.filter((category) => category.supportsEmail !== false && isChannelEnabled(prefs, category.key, 'email')).length;

  return (
    <Card className={cardClassName}>
      <CardHeader className="space-y-3">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
          <div>
            <CardTitle className="flex items-center gap-2 text-base">
              <Notification02Icon className="h-4 w-4" />
              {title}
            </CardTitle>
            <CardDescription className="mt-1">{description}</CardDescription>
          </div>
          <div className="flex flex-wrap gap-2">
            <StatusBadge active={!workspaceMuted}>{workspaceMuted ? 'Muted' : `${enabledInAppCount} in-app`}</StatusBadge>
            <StatusBadge active={emailEnabled && enabledEmailCount > 0}>
              {!emailEnabled ? 'Email off' : `${enabledEmailCount} email`}
            </StatusBadge>
          </div>
        </div>
      </CardHeader>
      <CardContent className="space-y-4">
        {workspaceMuted ? (
          <div className="rounded-md border bg-muted/30 px-3 py-2 text-xs leading-5 text-muted-foreground">
            {mutedWorkspaceMessage}
          </div>
        ) : null}

        {accountSettings && !emailEnabled ? (
          <div className="rounded-md border bg-muted/30 px-3 py-2 text-xs leading-5 text-muted-foreground">
            {emailDisabledMessage}
          </div>
        ) : null}

        {extraNote ? (
          <div className="rounded-md border bg-muted/30 px-3 py-2 text-xs leading-5 text-muted-foreground">
            {extraNote}
          </div>
        ) : null}

        <div className="overflow-hidden rounded-md border">
          <div className="hidden grid-cols-[minmax(0,1fr)_96px_96px] items-center gap-4 border-b bg-muted/30 px-4 py-2 text-xs font-medium text-muted-foreground md:grid">
            <span>Activity type</span>
            <span className="text-center">In-app</span>
            <span className="text-center">Email</span>
          </div>
          <div className="divide-y">
            {categories.map((category) => (
              <div key={category.key} className="grid gap-3 bg-background p-4 md:grid-cols-[minmax(0,1fr)_96px_96px] md:items-center md:gap-4">
                <div className="min-w-0">
                  <p className="text-sm font-medium">{category.label}</p>
                  <p className="mt-1 text-xs leading-5 text-muted-foreground">{category.description}</p>
                </div>
                <div className="flex items-center justify-between gap-3 md:justify-center">
                  <span className="text-xs font-medium text-muted-foreground md:hidden">In-app</span>
                  <Switch
                    checked={isChannelEnabled(prefs, category.key, 'in_app')}
                    disabled={updatePrefs.isPending}
                    onCheckedChange={(value) => handleToggle(category.key, 'in_app', value)}
                  />
                </div>
                <div className="flex items-center justify-between gap-3 md:justify-center">
                  <span className="text-xs font-medium text-muted-foreground md:hidden">Email</span>
                  {category.supportsEmail === false ? (
                    <span className="text-xs text-muted-foreground">Not available</span>
                  ) : (
                    <Switch
                      checked={isChannelEnabled(prefs, category.key, 'email')}
                      disabled={updatePrefs.isPending}
                      onCheckedChange={(value) => handleToggle(category.key, 'email', value)}
                    />
                  )}
                </div>
              </div>
            ))}
          </div>
        </div>
      </CardContent>
    </Card>
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
      title="Workspace activity"
      description="Choose which project updates appear in-app and which can also send email."
      categories={WORKSPACE_NOTIFICATION_CATEGORIES}
      mutedWorkspaceMessage="Workspace mute is on. These preferences are saved, but nothing from this workspace will deliver until mute is off."
      emailDisabledMessage="Account email is off. Email preferences are saved here, but no emails will send until account email is on."
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
      description="Choose how customer replies and teammate mentions reach you."
      categories={SUPPORT_NOTIFICATION_CATEGORIES}
      mutedWorkspaceMessage="Workspace mute is on. Support preferences are saved, but inbox alerts will not deliver until mute is off."
      emailDisabledMessage="Account email is off. Support email preferences are saved here, but no emails will send until account email is on."
      extraNote="Customer reply emails are fallback alerts. They send only if the conversation is still unread and unanswered after 3 minutes."
    />
  );
}
