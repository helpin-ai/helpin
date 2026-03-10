import { useTitle } from '@/hooks/useTitle';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useNotificationPreferences, useUpdateNotificationPreferences } from '@/hooks/queries';
import { NOTIFICATION_CATEGORIES } from '@/lib/notificationTypes';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Switch } from '@/components/ui/switch';
import { Skeleton } from '@/components/ui/skeleton';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Separator } from '@/components/ui/separator';
import { AtSign, Bell, Eye, MessageSquare, UserPlus, Zap, ArrowRightLeft } from 'lucide-react';
import { toast } from 'sonner';
import type { LucideIcon } from 'lucide-react';

const CATEGORY_ICONS: Record<string, LucideIcon> = {
  assignments: UserPlus,
  status_changes: ArrowRightLeft,
  comments: MessageSquare,
  mentions: AtSign,
  subscriptions: Eye,
  sprints: Zap,
};

export default function NotificationSettings() {
  useTitle('Notifications');
  const workspace = useWorkspaceStore((s) => s.currentWorkspace);
  const workspaceId = workspace?.id ?? '';

  const { data: prefs, isLoading } = useNotificationPreferences(workspaceId);
  const updatePrefs = useUpdateNotificationPreferences(workspaceId);

  const handleToggle = (field: 'do_not_disturb' | 'email_enabled', value: boolean) => {
    updatePrefs.mutate({ [field]: value }, {
      onError: () => toast.error('Failed to update notification preference'),
    });
  };

  const handleSelect = (field: 'email_digest_frequency' | 'badge_mode', value: string) => {
    updatePrefs.mutate({ [field]: value }, {
      onError: () => toast.error('Failed to update notification preference'),
    });
  };

  const isCategoryEnabled = (categoryKey: string): boolean => {
    const catPref = prefs?.channel_preferences?.[categoryKey];
    if (!catPref) return true; // default enabled
    // Category is enabled if either in_app or email is true (single toggle controls both)
    return catPref.in_app !== false;
  };

  const handleCategoryToggle = (categoryKey: string, enabled: boolean) => {
    const existing = prefs?.channel_preferences ?? {};
    const updated = {
      ...existing,
      [categoryKey]: { in_app: enabled, email: enabled },
    };
    updatePrefs.mutate({ channel_preferences: updated }, {
      onError: () => toast.error('Failed to update notification preference'),
    });
  };

  if (isLoading || !workspaceId) {
    return (
      <div className="space-y-4">
        <Skeleton className="h-48 w-full" />
        <Skeleton className="h-48 w-full" />
        <Skeleton className="h-64 w-full" />
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <h2 className="text-xl font-semibold">Notifications</h2>

      {/* Do Not Disturb */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Bell className="h-4 w-4" />
            Do Not Disturb
          </CardTitle>
          <CardDescription>Pause all notifications when you need focus time.</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium">Enable Do Not Disturb</p>
              <p className="text-xs text-muted-foreground">When enabled, you won't receive any notifications.</p>
            </div>
            <Switch
              checked={prefs?.do_not_disturb ?? false}
              onCheckedChange={(v) => handleToggle('do_not_disturb', v)}
            />
          </div>
        </CardContent>
      </Card>

      {/* Email */}
      <Card>
        <CardHeader>
          <CardTitle>Email</CardTitle>
          <CardDescription>Control email notification delivery and digest frequency.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium">Enable email notifications</p>
              <p className="text-xs text-muted-foreground">Receive notification emails for workspace activity.</p>
            </div>
            <Switch
              checked={prefs?.email_enabled ?? true}
              onCheckedChange={(v) => handleToggle('email_enabled', v)}
            />
          </div>
          <Separator />
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium">Email digest frequency</p>
              <p className="text-xs text-muted-foreground">How often to receive a summary of unread notifications.</p>
            </div>
            <Select
              value={prefs?.email_digest_frequency ?? 'daily'}
              onValueChange={(v) => handleSelect('email_digest_frequency', v)}
            >
              <SelectTrigger className="w-[140px]">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="daily">Daily</SelectItem>
                <SelectItem value="weekly">Weekly</SelectItem>
                <SelectItem value="never">Never</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </CardContent>
      </Card>

      {/* General Notifications — per-category toggles */}
      <Card>
        <CardHeader>
          <CardTitle>General notifications</CardTitle>
          <CardDescription>Choose which notifications you receive. Disabling a category turns off both in-app and email for that type.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-0">
          {NOTIFICATION_CATEGORIES.map((cat, idx) => {
            const Icon = CATEGORY_ICONS[cat.key] ?? Bell;
            return (
              <div key={cat.key}>
                {idx > 0 && <Separator className="my-3" />}
                <div className="flex items-center justify-between py-1">
                  <div className="flex items-center gap-3">
                    <div className="flex h-8 w-8 items-center justify-center rounded-md bg-muted">
                      <Icon className="h-4 w-4 text-muted-foreground" />
                    </div>
                    <div>
                      <p className="text-sm font-medium">{cat.label}</p>
                      <p className="text-xs text-muted-foreground">{cat.description}</p>
                    </div>
                  </div>
                  <Switch
                    checked={isCategoryEnabled(cat.key)}
                    onCheckedChange={(v) => handleCategoryToggle(cat.key, v)}
                  />
                </div>
              </div>
            );
          })}
        </CardContent>
      </Card>

      {/* Badge Mode */}
      <Card>
        <CardHeader>
          <CardTitle>Badge Mode</CardTitle>
          <CardDescription>Control which notifications show an unread badge.</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium">Show badge for</p>
              <p className="text-xs text-muted-foreground">Choose which notifications increment the unread counter.</p>
            </div>
            <Select
              value={prefs?.badge_mode ?? 'all'}
              onValueChange={(v) => handleSelect('badge_mode', v)}
            >
              <SelectTrigger className="w-[160px]">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">All notifications</SelectItem>
                <SelectItem value="mentions_only">Mentions only</SelectItem>
                <SelectItem value="none">None</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
