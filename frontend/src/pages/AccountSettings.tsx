import { useEffect, useState, type FormEvent } from 'react';
import { useTitle } from '@/hooks/useTitle';
import { useOrganizationStore } from '@/stores/organizationStore';
import { useAuthStore } from '@/stores/authStore';
import { useWorkspaces, useUserNotificationSettings, useUpdateUserNotificationSettings } from '@/hooks/queries';
import { organizationsService } from '@/lib/services/organizationsService';
import { authService } from '@/lib/services/authService';
import type { MemberWithUser } from '@/lib/types';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Badge } from '@/components/ui/badge';
import { Skeleton } from '@/components/ui/skeleton';
import { Switch } from '@/components/ui/switch';
import { Separator } from '@/components/ui/separator';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { cn } from '@/lib/utils';
import { UserAvatar } from '@/components/pm/UserAvatar';
import { Bell, Building2, Globe, Mail, Trash2, Users } from 'lucide-react';
import { ConfirmDialog } from '@/components/pm/ConfirmDialog';
import { toast } from 'sonner';

const ROLE_COLORS: Record<string, string> = {
  owner: 'bg-amber-100 text-amber-800',
  admin: 'bg-blue-100 text-blue-800',
  member: 'bg-gray-100 text-gray-700',
};

export default function AccountSettings() {
  useTitle('Account Settings');
  const { currentOrganization, setCurrentOrganization } = useOrganizationStore();
  const [name, setName] = useState('');
  const [saving, setSaving] = useState(false);
  const [removeMemberConfirm, setRemoveMemberConfirm] = useState<{ userId: string; name: string } | null>(null);
  const [members, setMembers] = useState<MemberWithUser[]>([]);
  const [loadingMembers, setLoadingMembers] = useState(true);

  const orgId = currentOrganization?.id;
  const isAdminOrOwner = currentOrganization?.role === 'owner' || currentOrganization?.role === 'admin';

  useEffect(() => {
    if (currentOrganization) {
      setName(currentOrganization.name);
    }
  }, [currentOrganization]);

  useEffect(() => {
    if (!orgId) return;
    setLoadingMembers(true);
    organizationsService.listMembers(orgId).then(({ data }) => {
      setMembers(data ?? []);
      setLoadingMembers(false);
    });
  }, [orgId]);

  const handleSave = async (e: FormEvent) => {
    e.preventDefault();
    if (!orgId) return;
    setSaving(true);
    const { data, error } = await organizationsService.update(orgId, { name });
    setSaving(false);
    if (error) {
      toast.error(error);
    } else {
      toast.success('Organization updated');
      if (data && currentOrganization) {
        setCurrentOrganization({ ...currentOrganization, name: data.name, slug: data.slug });
      }
    }
  };

  const handleRoleChange = async (userId: string, newRole: string) => {
    if (!orgId) return;
    const { error } = await organizationsService.updateMember(orgId, userId, { role: newRole });
    if (error) {
      toast.error(error);
    } else {
      toast.success('Role updated');
      setMembers((prev) => prev.map((m) => (m.user_id === userId ? { ...m, role: newRole } : m)));
    }
  };

  const handleRemoveMember = async (userId: string) => {
    if (!orgId) return;
    const { error } = await organizationsService.removeMember(orgId, userId);
    if (error) {
      toast.error(error);
    } else {
      toast.success('Member removed');
      setMembers((prev) => prev.filter((m) => m.user_id !== userId));
    }
  };

  const { user } = useAuthStore();
  const { data: workspaces = [] } = useWorkspaces(currentOrganization?.id);
  const [defaultWsId, setDefaultWsId] = useState<string>(user?.default_workspace_id ?? '');
  const [savingDefault, setSavingDefault] = useState(false);

  useEffect(() => {
    setDefaultWsId(user?.default_workspace_id ?? '');
  }, [user?.default_workspace_id]);

  const handleDefaultWorkspaceChange = async (value: string) => {
    const wsId = value === 'none' ? '' : value;
    setDefaultWsId(wsId);
    setSavingDefault(true);
    const { data, error } = await authService.updateProfile({ default_workspace_id: wsId || undefined });
    setSavingDefault(false);
    if (error) {
      toast.error(error);
      setDefaultWsId(user?.default_workspace_id ?? '');
    } else {
      toast.success('Default workspace updated');
      if (data) useAuthStore.setState({ user: data });
    }
  };

  if (!currentOrganization) {
    return (
      <div className="flex items-center justify-center py-16">
        <p className="text-muted-foreground">No organization selected</p>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <div>
        <h2 className="text-xl font-semibold">Account Settings</h2>
      </div>

      {/* Default Workspace */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Globe className="h-4 w-4" />
            Default Workspace
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="space-y-2">
            <Label>Redirect to this workspace after login</Label>
            <Select value={defaultWsId || 'none'} onValueChange={handleDefaultWorkspaceChange} disabled={savingDefault}>
              <SelectTrigger className="w-64">
                <SelectValue placeholder="Select a workspace" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="none">None (first available)</SelectItem>
                {workspaces.map((ws) => (
                  <SelectItem key={ws.id} value={ws.id}>
                    {ws.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        </CardContent>
      </Card>

      {/* Organization Name */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Building2 className="h-4 w-4" />
            Organization
          </CardTitle>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSave} className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="org-name">Name</Label>
              <Input
                id="org-name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                disabled={!isAdminOrOwner}
                required
              />
            </div>
            {isAdminOrOwner && (
              <Button type="submit" disabled={saving || name === currentOrganization.name}>
                {saving ? 'Saving...' : 'Save Changes'}
              </Button>
            )}
          </form>
        </CardContent>
      </Card>

      {/* Members */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Users className="h-4 w-4" />
            Members
          </CardTitle>
        </CardHeader>
        <CardContent>
          {loadingMembers ? (
            <div className="space-y-3">
              {[1, 2, 3].map((i) => (
                <Skeleton key={i} className="h-10 w-full" />
              ))}
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Member</TableHead>
                  <TableHead>Email</TableHead>
                  <TableHead>Role</TableHead>
                  {isAdminOrOwner && <TableHead className="w-10" />}
                </TableRow>
              </TableHeader>
              <TableBody>
                {members.map((member) => (
                  <TableRow key={member.id}>
                    <TableCell>
                      <div className="flex items-center gap-2">
                        <UserAvatar name={member.full_name} className="h-7 w-7" />
                        <span className="text-sm font-medium">{member.full_name}</span>
                      </div>
                    </TableCell>
                    <TableCell className="text-sm text-muted-foreground">{member.email}</TableCell>
                    <TableCell>
                      {isAdminOrOwner && member.role !== 'owner' ? (
                        <Select
                          value={member.role}
                          onValueChange={(val) => handleRoleChange(member.user_id, val)}
                        >
                          <SelectTrigger className="h-7 w-24 text-xs">
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem value="admin">Admin</SelectItem>
                            <SelectItem value="member">Member</SelectItem>
                          </SelectContent>
                        </Select>
                      ) : (
                        <Badge variant="secondary" className={cn('text-xs', ROLE_COLORS[member.role])}>
                          {member.role}
                        </Badge>
                      )}
                    </TableCell>
                    {isAdminOrOwner && (
                      <TableCell>
                        {member.role !== 'owner' && (
                          <Button
                            variant="ghost"
                            size="sm"
                            className="h-7 w-7 p-0 text-muted-foreground hover:text-destructive"
                            onClick={() => setRemoveMemberConfirm({ userId: member.user_id, name: member.full_name })}
                          >
                            <Trash2 className="h-3.5 w-3.5" />
                          </Button>
                        )}
                      </TableCell>
                    )}
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      {/* Notification Delivery Preferences (account-level) */}
      <NotificationDeliveryCard />

      <ConfirmDialog
        open={removeMemberConfirm !== null}
        onOpenChange={(open) => { if (!open) setRemoveMemberConfirm(null); }}
        title="Remove member"
        description={`This will remove ${removeMemberConfirm?.name ?? 'this member'} from the organization. They will lose access to all workspaces in this organization.`}
        confirmLabel="Remove"
        variant="destructive"
        onConfirm={() => { if (removeMemberConfirm) handleRemoveMember(removeMemberConfirm.userId); setRemoveMemberConfirm(null); }}
      />
    </div>
  );
}

function NotificationDeliveryCard() {
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
    <>
      {/* Do Not Disturb */}
      <Card>
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
              onCheckedChange={(v) => handleToggle('do_not_disturb', v)}
            />
          </div>
        </CardContent>
      </Card>

      {/* Email Delivery */}
      <Card>
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
              value={settings?.email_digest_frequency ?? 'daily'}
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
              value={settings?.badge_mode ?? 'all'}
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
    </>
  );
}
