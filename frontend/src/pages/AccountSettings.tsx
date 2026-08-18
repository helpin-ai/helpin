import { useEffect, useState, type FormEvent } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { useTitle } from '@/hooks/useTitle';
import { useOrganizationStore } from '@/stores/organizationStore';
import { useAuthStore } from '@/stores/authStore';
import { useWorkspaces } from '@/hooks/queries';
import { organizationsService } from '@/lib/services/organizationsService';
import { authService } from '@/lib/services/authService';
import type { MemberWithUser } from '@/lib/types';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Badge } from '@/components/ui/badge';
import { Skeleton } from '@/components/ui/skeleton';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { cn } from '@/lib/utils';
import { UserAvatar } from '@/components/pm/UserAvatar';
import { Building03Icon, Delete01Icon, UserGroupIcon } from '@/lib/icons';
import { ConfirmDialog } from '@/components/pm/ConfirmDialog';
import { toast } from 'sonner';
import {
  ASSIGNABLE_ORGANIZATION_ROLES,
  canEditOrganizationMemberRole,
  canRemoveOrganizationMember,
  organizationRoleLabel,
} from '@/components/settings/roleScopePresentation';
import { queryKeys } from '@/lib/queryKeys';

const ROLE_COLORS: Record<string, string> = {
  owner: 'bg-amber-100 text-amber-800',
  admin: 'bg-blue-100 text-blue-800',
  member: 'bg-gray-100 text-gray-700',
  viewer: 'bg-slate-100 text-slate-700',
};

export default function AccountSettings() {
  useTitle('Organization Settings');
  const { currentOrganization, setCurrentOrganization } = useOrganizationStore();
  const [name, setName] = useState('');
  const [saving, setSaving] = useState(false);
  const [removeMemberConfirm, setRemoveMemberConfirm] = useState<{ userId: string; name: string } | null>(null);
  const [members, setMembers] = useState<MemberWithUser[]>([]);
  const [loadingMembers, setLoadingMembers] = useState(true);
  const [updatingRoleUserIds, setUpdatingRoleUserIds] = useState<Set<string>>(() => new Set());
  const [newOwnerId, setNewOwnerId] = useState('');
  const [transferConfirmOpen, setTransferConfirmOpen] = useState(false);
  const [transferringOwnership, setTransferringOwnership] = useState(false);
  const queryClient = useQueryClient();

  const orgId = currentOrganization?.id;
  const myRole = currentOrganization?.role;
  const isOwner = myRole === 'owner';
  const isAdminOrOwner = isOwner || myRole === 'admin';

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
    if (!orgId || updatingRoleUserIds.has(userId)) return;
    const previousRole = members.find((member) => member.user_id === userId)?.role;
    if (!previousRole || previousRole === newRole) return;

    setUpdatingRoleUserIds((current) => new Set(current).add(userId));
    setMembers((current) => current.map((member) => (
      member.user_id === userId ? { ...member, role: newRole } : member
    )));
    const { error } = await organizationsService.updateMember(orgId, userId, { role: newRole });
    if (error) {
      setMembers((current) => current.map((member) => (
        member.user_id === userId ? { ...member, role: previousRole } : member
      )));
      toast.error("Couldn't update organization role", { description: error });
    } else {
      toast.success('Organization role updated');
      void queryClient.invalidateQueries({ queryKey: queryKeys.organizations.members(orgId) });
    }
    setUpdatingRoleUserIds((current) => {
      const next = new Set(current);
      next.delete(userId);
      return next;
    });
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
  const canTransferOwnership = isOwner && currentOrganization?.owner_id === user?.id;
  const newOwner = members.find((member) => member.user_id === newOwnerId);
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

  const handleTransferOwnership = async () => {
    if (!orgId || !user || !newOwnerId || !canTransferOwnership) return;
    setTransferringOwnership(true);
    const { error } = await organizationsService.transferOwnership(orgId, newOwnerId);
    setTransferringOwnership(false);
    setTransferConfirmOpen(false);
    if (error) {
      toast.error("Couldn't transfer organization ownership", { description: error });
      return;
    }

    setMembers((current) => current.map((member) => ({
      ...member,
      role: member.user_id === newOwnerId
        ? 'owner'
        : member.role === 'owner' ? 'admin' : member.role,
    })));
    setCurrentOrganization({ ...currentOrganization, owner_id: newOwnerId, role: 'admin' });
    setNewOwnerId('');
    queryClient.removeQueries({ queryKey: queryKeys.billing.org(orgId) });
    queryClient.removeQueries({ queryKey: queryKeys.billing.cards(orgId) });
    queryClient.removeQueries({ queryKey: queryKeys.billing.invoices(orgId) });
    void queryClient.invalidateQueries({ queryKey: queryKeys.organizations.all });
    void queryClient.invalidateQueries({ queryKey: queryKeys.organizations.members(orgId) });
    toast.success('Organization ownership transferred');
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
        <h2 className="text-xl font-semibold">Organization</h2>
      </div>

      {/* Organization & Default Workspace */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Building03Icon className="h-4 w-4" />
            Organization & Default Workspace
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
            <form onSubmit={handleSave} className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="org-name">Organization Name</Label>
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
            <div className="space-y-4">
              <div className="space-y-2">
                <Label>Default Workspace</Label>
                <Select value={defaultWsId || 'none'} onValueChange={handleDefaultWorkspaceChange} disabled={savingDefault}>
                  <SelectTrigger>
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
                <p className="text-xs text-muted-foreground">Redirect to this workspace after login</p>
              </div>
            </div>
          </div>
        </CardContent>
      </Card>

      {canTransferOwnership && (
        <Card>
          <CardHeader>
            <CardTitle>Transfer ownership</CardTitle>
            <CardDescription>
              The new owner will control organization billing and ownership. You will become an Organization admin.
            </CardDescription>
          </CardHeader>
          <CardContent className="flex flex-col gap-3 sm:flex-row sm:items-end">
            <div className="w-full space-y-2 sm:max-w-sm">
              <Label htmlFor="new-organization-owner">New owner</Label>
              <Select value={newOwnerId} onValueChange={setNewOwnerId} disabled={transferringOwnership}>
                <SelectTrigger id="new-organization-owner" className="w-full">
                  <SelectValue placeholder="Select an organization member" />
                </SelectTrigger>
                <SelectContent>
                  {members.filter((member) => member.user_id !== user?.id).map((member) => (
                    <SelectItem key={member.user_id} value={member.user_id}>
                      {member.full_name || member.email}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <Button
              variant="destructive"
              disabled={!newOwnerId || transferringOwnership}
              onClick={() => setTransferConfirmOpen(true)}
            >
              {transferringOwnership ? 'Transferring…' : 'Transfer ownership'}
            </Button>
          </CardContent>
        </Card>
      )}

      {/* Members */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <UserGroupIcon className="h-4 w-4" />
            Members
          </CardTitle>
          <CardDescription>
            Organization roles control organization settings and integrations. Workspace access and billing permissions are managed separately.
          </CardDescription>
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
                  <TableHead>Organization role</TableHead>
                  {isAdminOrOwner && <TableHead className="w-10" />}
                </TableRow>
              </TableHeader>
              <TableBody>
                {members.map((member) => {
                  const isSelf = member.user_id === user?.id;
                  const canEditRole = canEditOrganizationMemberRole(myRole, member.role, isSelf);
                  const canRemove = canRemoveOrganizationMember(myRole, member.role, isSelf);
                  const isUpdatingRole = updatingRoleUserIds.has(member.user_id);

                  return (
                    <TableRow key={member.id}>
                      <TableCell>
                        <div className="flex items-center gap-2">
                          <UserAvatar
                            name={member.full_name || member.email}
                            avatarUrl={member.avatar_url}
                            avatarStyle={member.avatar_style}
                            avatarSeed={member.avatar_seed}
                            avatarBackgroundMode={member.avatar_background_mode}
                            avatarBackgroundColor={member.avatar_background_color}
                            className="h-7 w-7"
                          />
                          <span className="text-sm font-medium">{member.full_name}</span>
                          {isSelf && <Badge variant="outline" className="text-[10px]">You</Badge>}
                        </div>
                      </TableCell>
                      <TableCell className="text-sm text-muted-foreground">{member.email}</TableCell>
                      <TableCell>
                        {canEditRole ? (
                          <Select
                            value={member.role}
                            onValueChange={(val) => handleRoleChange(member.user_id, val)}
                            disabled={isUpdatingRole}
                          >
                            <SelectTrigger
                              className="h-7 w-40 text-xs"
                              aria-label={`Change organization role for ${member.full_name || member.email}`}
                            >
                              <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                              {ASSIGNABLE_ORGANIZATION_ROLES.map((role) => (
                                <SelectItem key={role} value={role}>{organizationRoleLabel(role)}</SelectItem>
                              ))}
                            </SelectContent>
                          </Select>
                        ) : (
                          <Badge variant="secondary" className={cn('text-xs', ROLE_COLORS[member.role])}>
                            {organizationRoleLabel(member.role)}
                          </Badge>
                        )}
                      </TableCell>
                      {isAdminOrOwner && (
                        <TableCell>
                          {canRemove && (
                            <Button
                              variant="ghost"
                              size="sm"
                              className="h-7 w-7 p-0 text-muted-foreground hover:text-destructive"
                              onClick={() => setRemoveMemberConfirm({ userId: member.user_id, name: member.full_name })}
                            >
                              <Delete01Icon className="h-3.5 w-3.5" />
                            </Button>
                          )}
                        </TableCell>
                      )}
                    </TableRow>
                  );
                })}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      <ConfirmDialog
        open={transferConfirmOpen}
        onOpenChange={(open) => { if (!transferringOwnership) setTransferConfirmOpen(open); }}
        title="Transfer organization ownership?"
        description={`This gives ${newOwner?.full_name || newOwner?.email || 'the selected member'} control of organization billing and ownership. Your role will change to Organization admin.`}
        confirmLabel={transferringOwnership ? 'Transferring…' : 'Transfer ownership'}
        variant="destructive"
        onConfirm={() => { void handleTransferOwnership(); }}
      />

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
