import { useTitle } from '@/hooks/useTitle';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useNotificationPreferences, useUpdateNotificationPreferences } from '@/hooks/queries';
import { NOTIFICATION_CATEGORIES } from '@/lib/notificationTypes';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Switch } from '@/components/ui/switch';
import { Skeleton } from '@/components/ui/skeleton';
import { Separator } from '@/components/ui/separator';
import { AtSign, Bell, BellOff, Eye, MessageSquare, UserPlus, Zap, ArrowRightLeft } from 'lucide-react';
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

  const handleMuteToggle = (value: boolean) => {
    updatePrefs.mutate({ mute_workspace: value }, {
      onError: () => toast.error('Failed to update notification preference'),
    });
  };

  const isCategoryEnabled = (categoryKey: string): boolean => {
    const catPref = prefs?.channel_preferences?.[categoryKey];
    if (!catPref) return true; // default enabled
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
        <Skeleton className="h-64 w-full" />
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <div>
        <h2 className="text-xl font-semibold">Workspace Notifications</h2>
        <p className="text-sm text-muted-foreground mt-1">
          Control which notifications you receive for this workspace. Email delivery and Do Not Disturb are managed in Account settings.
        </p>
      </div>

      {/* Mute Workspace */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <BellOff className="h-4 w-4" />
            Mute Workspace
          </CardTitle>
          <CardDescription>Silence all notifications from this workspace.</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium">Mute all notifications</p>
              <p className="text-xs text-muted-foreground">You won't receive any notifications from {workspace?.name ?? 'this workspace'}.</p>
            </div>
            <Switch
              checked={prefs?.mute_workspace ?? false}
              onCheckedChange={handleMuteToggle}
            />
          </div>
        </CardContent>
      </Card>

      {/* General Notifications — per-category toggles */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Bell className="h-4 w-4" />
            Notification categories
          </CardTitle>
          <CardDescription>Choose which types of notifications you receive in this workspace.</CardDescription>
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
                    disabled={prefs?.mute_workspace}
                  />
                </div>
              </div>
            );
          })}
        </CardContent>
      </Card>
    </div>
  );
}
