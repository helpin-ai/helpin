import { Bell, BellOff, Mail } from 'lucide-react';
import { toast } from 'sonner';

import { useNotificationPreferences, useUpdateNotificationPreferences, useUserNotificationSettings, useUpdateUserNotificationSettings } from '@/hooks/queries';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Separator } from '@/components/ui/separator';
import { Skeleton } from '@/components/ui/skeleton';
import { Switch } from '@/components/ui/switch';

type CardClassNameProps = {
  cardClassName?: string;
};

export function AccountNotificationPreferences({ cardClassName }: CardClassNameProps) {
  const { data: settings, isLoading } = useUserNotificationSettings();
  const updateSettings = useUpdateUserNotificationSettings();

  const handleToggle = (field: 'do_not_disturb' | 'email_enabled', value: boolean) => {
    updateSettings.mutate({ [field]: value }, {
      onError: () => toast.error('Failed to update notification setting'),
    });
  };

  const handleSelect = (field: 'email_digest_frequency' | 'badge_mode', value: string) => {
    updateSettings.mutate({ [field]: value }, {
      onError: () => toast.error('Failed to update notification setting'),
    });
  };

  if (isLoading) {
    return <Skeleton className="h-64 w-full" />;
  }

  return (
    <div className="space-y-4">
      <Card className={cardClassName}>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Bell className="h-4 w-4" />
            Do Not Disturb
          </CardTitle>
          <CardDescription>Pause all notifications across all workspaces.</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium">Enable Do Not Disturb</p>
              <p className="text-xs text-muted-foreground">When enabled, you won't receive any notifications.</p>
            </div>
            <Switch
              checked={settings?.do_not_disturb ?? false}
              onCheckedChange={(value) => handleToggle('do_not_disturb', value)}
            />
          </div>
        </CardContent>
      </Card>

      <Card className={cardClassName}>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Mail className="h-4 w-4" />
            Email Notifications
          </CardTitle>
          <CardDescription>Control email delivery and digest frequency across all workspaces.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium">Enable email notifications</p>
              <p className="text-xs text-muted-foreground">Receive notification emails for activity.</p>
            </div>
            <Switch
              checked={settings?.email_enabled ?? true}
              onCheckedChange={(value) => handleToggle('email_enabled', value)}
            />
          </div>
          <Separator />
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium">Email digest frequency</p>
              <p className="text-xs text-muted-foreground">How often to receive a summary of unread notifications.</p>
            </div>
            <Select
              value={settings?.email_digest_frequency ?? 'daily'}
              onValueChange={(value) => handleSelect('email_digest_frequency', value)}
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

      <Card className={cardClassName}>
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
              value={settings?.badge_mode ?? 'all'}
              onValueChange={(value) => handleSelect('badge_mode', value)}
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

export function WorkspaceMuteNotificationsCard({ workspaceId, workspaceName, cardClassName }: CardClassNameProps & { workspaceId: string; workspaceName?: string; }) {
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
        <CardDescription>Personal preference for this workspace only.</CardDescription>
      </CardHeader>
      <CardContent>
        <div className="flex items-center justify-between">
          <div>
            <p className="text-sm font-medium">Mute notifications from this workspace</p>
            <p className="text-xs text-muted-foreground">
              You won't receive notifications from {workspaceName ?? 'this workspace'}, but your account-level notification preferences stay unchanged.
            </p>
          </div>
          <Switch
            checked={prefs?.mute_workspace ?? false}
            onCheckedChange={handleMuteToggle}
          />
        </div>
      </CardContent>
    </Card>
  );
}
