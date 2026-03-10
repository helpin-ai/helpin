import { useEffect, useMemo, useState } from 'react';
import { Bell, BellOff, ChevronRight, Globe, Mail, Search } from 'lucide-react';
import { toast } from 'sonner';

import {
  useNotificationPreferences,
  useUpdateNotificationPreferences,
  useUserNotificationSettings,
  useUpdateUserNotificationSettings,
} from '@/hooks/queries';
import {
  NOTIFICATION_CATEGORIES,
  type NotificationPreferences,
  type UpdateUserNotificationSettingsRequest,
} from '@/lib/notificationTypes';
import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Separator } from '@/components/ui/separator';
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

function formatDateTimeLocal(value?: string | null) {
  if (!value) return '';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '';

  const pad = (part: number) => String(part).padStart(2, '0');
  return [
    date.getFullYear(),
    pad(date.getMonth() + 1),
    pad(date.getDate()),
  ].join('-') + `T${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

function toISOStringFromLocal(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return null;
  return date.toISOString();
}

function formatResumeTime(value?: string | null, timezone?: string) {
  if (!value) return null;
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return null;

  return new Intl.DateTimeFormat('en-US', {
    timeZone: timezone || DEFAULT_TIMEZONE,
    month: 'short',
    day: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
    hour12: true,
  }).format(date);
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
            <Globe className="h-4 w-4 shrink-0 text-muted-foreground" />
            <span className="truncate">{value}</span>
            {selectedTz ? <span className="truncate text-muted-foreground">({selectedTz.offset})</span> : null}
          </span>
          <ChevronRight className="h-4 w-4 shrink-0 text-muted-foreground" />
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-[320px] p-0" align="end">
        <div className="border-b p-2">
          <div className="flex items-center gap-2 px-2">
            <Search className="h-4 w-4 shrink-0 text-muted-foreground" />
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
  const [dndUntilInput, setDndUntilInput] = useState('');

  useEffect(() => {
    setDndUntilInput(formatDateTimeLocal(settings?.dnd_until));
  }, [settings?.dnd_until]);

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
      setDndUntilInput('');
      handleSettingsUpdate({ do_not_disturb: false, dnd_until: null });
      return;
    }

    const dndUntil = dndUntilInput ? toISOStringFromLocal(dndUntilInput) : null;
    handleSettingsUpdate({
      do_not_disturb: true,
      dnd_until: dndUntil,
    });
  };

  const handleApplyDndUntil = () => {
    const dndUntil = dndUntilInput ? toISOStringFromLocal(dndUntilInput) : null;
    if (dndUntilInput && !dndUntil) {
      toast.error('Enter a valid date and time');
      return;
    }
    handleSettingsUpdate({
      do_not_disturb: true,
      dnd_until: dndUntil,
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
  const resumeTime = formatResumeTime(settings?.dnd_until, settings?.timezone);

  if (isLoading) {
    return <Skeleton className="h-96 w-full" />;
  }

  return (
    <div className="space-y-4">
      <Card className={cardClassName}>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Bell className="h-4 w-4" />
            Do Not Disturb
          </CardTitle>
          <CardDescription>Pause inbox and email notifications across every workspace on your account.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex items-center justify-between gap-4">
            <div>
              <p className="text-sm font-medium">Pause all notifications</p>
              <p className="text-xs text-muted-foreground">Turn this on to stop new in-app and email notifications everywhere.</p>
            </div>
            <Switch
              checked={dndActive}
              disabled={updateSettings.isPending}
              onCheckedChange={handleDndToggle}
            />
          </div>

          {dndActive ? (
            <>
              <Separator />
              <div className="space-y-3">
                <div className="space-y-2">
                  <Label htmlFor="dnd-until">Pause until</Label>
                  <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
                    <Input
                      id="dnd-until"
                      type="datetime-local"
                      value={dndUntilInput}
                      disabled={updateSettings.isPending}
                      onChange={(event) => setDndUntilInput(event.target.value)}
                    />
                    <Button
                      type="button"
                      variant="outline"
                      disabled={updateSettings.isPending}
                      onClick={handleApplyDndUntil}
                    >
                      Apply
                    </Button>
                    <Button
                      type="button"
                      variant="ghost"
                      disabled={updateSettings.isPending}
                      onClick={() => {
                        setDndUntilInput('');
                        handleSettingsUpdate({ do_not_disturb: true, dnd_until: null });
                      }}
                    >
                      Pause indefinitely
                    </Button>
                  </div>
                  <p className="text-xs text-muted-foreground">
                    Set an end time for DND, or leave it blank to keep notifications paused until you turn them back on.
                  </p>
                </div>
                <div className="rounded-md border bg-muted/30 px-3 py-2 text-xs text-muted-foreground">
                  {resumeTime ? `Notifications resume on ${resumeTime} (${settings?.timezone ?? DEFAULT_TIMEZONE}).` : 'Notifications are paused indefinitely.'}
                </div>
              </div>
            </>
          ) : null}
        </CardContent>
      </Card>

      <Card className={cardClassName}>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Mail className="h-4 w-4" />
            Email Notifications
          </CardTitle>
          <CardDescription>
            Account-level email delivery rules apply everywhere. Workspace email toggles only decide which activity is eligible inside each workspace.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex items-center justify-between gap-4">
            <div>
              <p className="text-sm font-medium">Enable email notifications</p>
              <p className="text-xs text-muted-foreground">Turn this off to stop all notification emails across all workspaces.</p>
            </div>
            <Switch
              checked={emailEnabled}
              disabled={updateSettings.isPending}
              onCheckedChange={(value) => handleToggle('email_enabled', value)}
            />
          </div>
          <Separator />
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
                High-priority notifications still send right away. Daily and weekly modes batch normal activity into digests.
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
            <>
              <Separator />
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
            </>
          ) : null}
        </CardContent>
      </Card>
    </div>
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
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <BellOff className="h-4 w-4" />
          Workspace Notifications
        </CardTitle>
        <CardDescription>This personal workspace override sits on top of the notification types below.</CardDescription>
      </CardHeader>
      <CardContent>
        <div className="flex items-center justify-between gap-4">
          <div>
            <p className="text-sm font-medium">Mute notifications from this workspace</p>
            <p className="text-xs text-muted-foreground">
              When muted, you won&apos;t receive inbox or email notifications from {workspaceName ?? 'this workspace'}, even if the notification types below stay enabled.
            </p>
          </div>
          <Switch
            checked={prefs?.mute_workspace ?? false}
            disabled={updatePrefs.isPending}
            onCheckedChange={handleMuteToggle}
          />
        </div>
      </CardContent>
    </Card>
  );
}

const isChannelEnabled = (
  prefs: NotificationPreferences | undefined,
  categoryKey: string,
  channel: 'in_app' | 'email',
) => prefs?.channel_preferences?.[categoryKey]?.[channel] ?? true;

export function WorkspaceNotificationCategoriesCard({
  workspaceId,
  cardClassName,
}: CardClassNameProps & { workspaceId: string }) {
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

  return (
    <Card className={cardClassName}>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <Bell className="h-4 w-4" />
          Notification Types
        </CardTitle>
        <CardDescription>
          Choose which activity from this workspace appears in-app and which activity may also send email when your account email settings allow it.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {prefs?.mute_workspace ? (
          <div className="rounded-md border bg-muted/30 px-3 py-2 text-xs text-muted-foreground">
            This workspace is muted, so the notification types below are saved as preferences but won&apos;t deliver until workspace mute is turned off.
          </div>
        ) : null}

        {accountSettings && !accountSettings.email_enabled ? (
          <div className="rounded-md border bg-muted/30 px-3 py-2 text-xs text-muted-foreground">
            Account email notifications are off. The email column below sets your workspace preferences, but no emails will send until email is re-enabled in account Notifications.
          </div>
        ) : null}

        <div className="grid grid-cols-[minmax(0,1fr)_88px_88px] items-center gap-4 text-xs font-medium text-muted-foreground">
          <span>Activity</span>
          <span className="text-center">In-app</span>
          <span className="text-center">Email</span>
        </div>
        {NOTIFICATION_CATEGORIES.map((category, index) => (
          <div key={category.key}>
            {index > 0 ? <Separator className="mb-4" /> : null}
            <div className="grid grid-cols-[minmax(0,1fr)_88px_88px] items-center gap-4">
              <div className="min-w-0">
                <p className="text-sm font-medium">{category.label}</p>
                <p className="text-xs text-muted-foreground">{category.description}</p>
              </div>
              <div className="flex justify-center">
                <Switch
                  checked={isChannelEnabled(prefs, category.key, 'in_app')}
                  disabled={updatePrefs.isPending}
                  onCheckedChange={(value) => handleToggle(category.key, 'in_app', value)}
                />
              </div>
              <div className="flex justify-center">
                <Switch
                  checked={isChannelEnabled(prefs, category.key, 'email')}
                  disabled={updatePrefs.isPending}
                  onCheckedChange={(value) => handleToggle(category.key, 'email', value)}
                />
              </div>
            </div>
          </div>
        ))}
      </CardContent>
    </Card>
  );
}
