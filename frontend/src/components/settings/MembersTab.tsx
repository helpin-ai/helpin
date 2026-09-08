import { useEffect, useMemo, useState, type FormEvent } from 'react';
import { useCopyToClipboard } from '@/hooks/useCopyToClipboard';
import { useOrganizationMembers, useWorkspaceMemberPresenceMap } from '@/hooks/queries';
import { workspacesService } from '@/lib/services/workspacesService';
import { inviteService } from '@/lib/services/inviteService';
import { settingsService } from '@/lib/services/settingsService';
import type { MemberWithUser, Invitation, TeamUserMembership, WorkspaceTeam } from '@/lib/types';
import { UserAvatar } from '@/components/pm/UserAvatar';
import { QuickTooltip } from '@/components/ui/quick-tooltip';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { EmailChipInput, classifyEmailChipInput, mergeEmailChips } from '@/components/ui/email-chip-input';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Skeleton } from '@/components/ui/skeleton';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle, DialogDescription } from '@/components/ui/dialog';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Command, CommandInput, CommandList, CommandEmpty, CommandGroup, CommandItem } from '@/components/ui/command';
import { cn } from '@/lib/utils';
import { Copy01Icon, PlusSignIcon, ArrowReloadHorizontalIcon, UserGroupIcon, SecurityCheckIcon, Shield01Icon } from '@/lib/icons';
import { QuietSearchInput } from '@/components/design-system/quiet';
import { toast } from 'sonner';
import { useAuthStore } from '@/stores/authStore';
import { ConfirmDialog } from '@/components/pm/ConfirmDialog';
import { workspaceRoleLabel } from './roleScopePresentation';
import { useWorkspaceModuleAccess } from '@/hooks/queries/useSettings';
import { EditMemberDialog } from './EditMemberDialog';
import { getMemberModuleAccess } from './memberAccess';

export function MembersTab({ workspaceId, organizationId, editable, canManageTeams = false, canManageModuleAccess = false, teams, userMemberships, onRefresh }: {
  workspaceId: string;
  organizationId?: string;
  editable: boolean;
  canManageTeams?: boolean;
  canManageModuleAccess?: boolean;
  teams: WorkspaceTeam[];
  userMemberships: TeamUserMembership[];
  onRefresh?: () => void | Promise<void>;
}) {
  const [members, setMembers] = useState<MemberWithUser[]>([]);
  const [invitations, setInvitations] = useState<Invitation[]>([]);
  const [loading, setLoading] = useState(true);
  const [inviteOpen, setInviteOpen] = useState(false);
  const [invEmails, setInvEmails] = useState<string[]>([]);
  const [invEmailInput, setInvEmailInput] = useState('');
  const [invRole, setInvRole] = useState('member');
  const [sending, setSending] = useState(false);
  const [createdJoinUrl, setCreatedJoinUrl] = useState<string | null>(null);
  const [selectedTeamIds, setSelectedTeamIds] = useState<string[]>([]);
  const [query, setQuery] = useState('');
  const [editingMember, setEditingMember] = useState<MemberWithUser | null>(null);
  const [removingMember, setRemovingMember] = useState<MemberWithUser | null>(null);
  const [removingMemberId, setRemovingMemberId] = useState<string | null>(null);
  const [managingInvitation, setManagingInvitation] = useState<Invitation | null>(null);
  const [invitationBusy, setInvitationBusy] = useState(false);
  const [revokeInvitationConfirm, setRevokeInvitationConfirm] = useState(false);
  const moduleAccess = useWorkspaceModuleAccess(workspaceId, { enabled: canManageModuleAccess });
  const { user } = useAuthStore();

  const { data: orgMembers } = useOrganizationMembers(organizationId);
  const { data: memberPresenceByUserId } = useWorkspaceMemberPresenceMap(workspaceId);

  // Org members not already in this workspace (and not pending invitation)
  const availableOrgMembers = useMemo(() => {
    if (!orgMembers) return [];
    const wsEmails = new Set(members.map((m) => m.email.toLowerCase()));
    const pendingEmails = new Set(
      invitations.filter((inv) => inv.status === 'pending').map((inv) => inv.email.toLowerCase())
    );
    return orgMembers.filter(
      (om) => !wsEmails.has(om.email.toLowerCase()) && !pendingEmails.has(om.email.toLowerCase())
    );
  }, [orgMembers, members, invitations]);

  const teamsByUserId = useMemo(() => {
    const teamById = new Map(teams.map((team) => [team.id, team]));
    const memberships = new Map<string, { id: string; name: string }[]>();

    userMemberships.forEach((membership) => {
      const team = teamById.get(membership.team_id);
      if (!team) return;
      const current = memberships.get(membership.user_id) ?? [];
      current.push({ id: team.id, name: team.name });
      memberships.set(membership.user_id, current);
    });

    memberships.forEach((teamList, userId) => {
      memberships.set(userId, [...teamList].sort((a, b) => a.name.localeCompare(b.name)));
    });

    return memberships;
  }, [teams, userMemberships]);

  const filteredMembers = useMemo(() => {
    const search = query.trim().toLowerCase();
    if (!search) return members;
    return members.filter((member) =>
      [member.full_name, member.email, member.role]
        .filter(Boolean)
        .some((value) => value!.toLowerCase().includes(search))
    );
  }, [members, query]);

  const pendingInvitations = useMemo(() =>
    invitations.filter((inv) => inv.status === 'pending'),
    [invitations],
  );

  const actorMember = useMemo(
    () => members.find((member) => member.user_id === user?.id) ?? null,
    [members, user?.id],
  );

  const actorRole = actorMember?.role ?? '';

  const loadData = async () => {
    setLoading(true);
    const [membersRes, invitationsRes] = await Promise.all([
      workspacesService.listMembers(workspaceId),
      editable ? inviteService.list(workspaceId) : Promise.resolve({ data: null, error: null }),
    ]);
    if (membersRes.data) setMembers(membersRes.data);
    if (invitationsRes.data) setInvitations(invitationsRes.data);
    setLoading(false);
  };

  useEffect(() => { loadData(); }, [workspaceId]);

  const openInviteDialog = () => {
    setCreatedJoinUrl(null);
    setInvEmails([]);
    setInvEmailInput('');
    setInvRole('member');
    setSelectedTeamIds([]);
    setInviteOpen(true);
  };

  const closeInviteDialog = () => {
    setInviteOpen(false);
    if (createdJoinUrl) loadData();
  };

  const pendingInviteEmails = useMemo(
    () => mergeEmailChips(invEmails, invEmailInput),
    [invEmailInput, invEmails],
  );
  const hasInvalidInviteInput = useMemo(
    () => classifyEmailChipInput(invEmailInput).invalid.length > 0,
    [invEmailInput],
  );

  const handleInvite = async (e: FormEvent) => {
    e.preventDefault();
    const emails = pendingInviteEmails;
    if (emails.length === 0) {
      toast.error('Enter at least one valid email address');
      return;
    }
    setInvEmails(emails);
    setInvEmailInput('');
    setSending(true);
    let sent = 0;
    const failures: { email: string; error: string }[] = [];
    let lastJoinUrl: string | null = null;

    await Promise.all(
      emails.map(async (email) => {
        const { data, error } = await inviteService.send({ workspace_id: workspaceId, email, role: invRole });
        if (error) {
          failures.push({ email, error });
        } else {
          if (data?.id && selectedTeamIds.length > 0) {
            await Promise.all(
              selectedTeamIds.map((teamId) => settingsService.addTeamInvitation(workspaceId, teamId, data.id)),
            );
          }
          if (data?.join_url) lastJoinUrl = data.join_url;
          sent++;
        }
      }),
    );

    setSending(false);

    const failureLines = failures.map((f) => `${f.email}: ${f.error}`);

    if (sent > 0 && failures.length > 0) {
      toast.warning(`${sent} of ${emails.length} invitations sent`, {
        description: failureLines.join('\n'),
      });
    } else if (sent > 0) {
      toast.success(`${sent} invitation${sent === 1 ? '' : 's'} sent`);
    } else if (failures.length === 1) {
      toast.error(`Failed to invite ${failures[0].email}`, {
        description: failures[0].error,
      });
    } else {
      toast.error('Failed to send invitations', {
        description: failureLines.join('\n'),
      });
    }

    if (sent > 0) {
      if (emails.length === 1 && lastJoinUrl) {
        setCreatedJoinUrl(lastJoinUrl);
      } else {
        setInviteOpen(false);
        loadData();
      }
    }
  };

  const { copy: copyToClipboard } = useCopyToClipboard();
  const handleCopyLink = (joinUrl: string) => {
    copyToClipboard(joinUrl);
    toast.success('Invite link copied to clipboard');
  };

  const handleResend = async (id: string) => {
    const { error } = await inviteService.resend(id, workspaceId);
    if (error) toast.error(error);
    else toast.success('Invitation resent');
  };

  const handleRevoke = async (id: string) => {
    const { error } = await inviteService.revoke(id, workspaceId);
    if (error) toast.error(error);
    else {
      toast.success('Invitation revoked');
      setInvitations((prev) => prev.filter((inv) => inv.id !== id));
      setManagingInvitation(null);
    }
  };

  const canEditMemberRole = (member: MemberWithUser) => {
    if (!editable || !user) return false;
    if (member.user_id === user.id) return false;
    if (actorRole !== 'owner' && actorRole !== 'admin') return false;
    if (actorRole !== 'owner' && (member.role === 'owner' || member.role === 'admin')) return false;
    return true;
  };

  const roleOptions = (member: MemberWithUser): Array<{ value: 'owner' | 'admin' | 'member' | 'viewer'; label: string }> => {
    const options: Array<{ value: 'owner' | 'admin' | 'member' | 'viewer'; label: string }> = [
      { value: 'admin', label: workspaceRoleLabel('admin') },
      { value: 'member', label: workspaceRoleLabel('member') },
      { value: 'viewer', label: workspaceRoleLabel('viewer') },
    ];
    if (actorRole === 'owner') {
      options.unshift({ value: 'owner', label: workspaceRoleLabel('owner') });
    }
    if (actorRole !== 'owner' && member.role === 'admin') {
      return [{ value: 'admin', label: workspaceRoleLabel('admin') }];
    }
    return options;
  };

  const canRemoveMember = (member: MemberWithUser) => {
    if (!editable || !user) return false;
    if (member.user_id === user.id) return false;
    if (actorRole !== 'owner' && actorRole !== 'admin') return false;
    if (member.role === 'owner') return false;
    if (member.role === 'admin' && actorRole !== 'owner') return false;
    return true;
  };

  if (loading) return <Skeleton className="h-96" />;

  return (
    <>
      <div className="space-y-6">
        <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
          <span className="text-sm text-muted-foreground">{members.length} {members.length === 1 ? 'member' : 'members'} in this workspace</span>
          <div className="flex flex-col gap-3 sm:flex-row sm:items-center">
            {members.length > 0 && (
              <QuietSearchInput
                containerClassName="w-full sm:w-60"
                value={query}
                onChange={(event) => setQuery(event.target.value)}
                placeholder="Search members..."
              />
            )}
            {editable && (
              <Button size="sm" onClick={openInviteDialog}>
                <PlusSignIcon className="h-4 w-4 mr-1" /> Invite Member
              </Button>
            )}
          </div>
        </div>

        {editable && pendingInvitations.length > 0 && (
          <div className="space-y-3">
            <div>
              <h3 className="text-sm font-medium">Pending Invitations</h3>
              <p className="text-sm text-muted-foreground">
                Invitations that have been sent but not yet accepted.
              </p>
            </div>
            <div className="overflow-hidden rounded-lg border border-border">
              <Table>
                <TableHeader>
                  <TableRow className="bg-muted/40 hover:bg-muted/40">
                    <TableHead>Email</TableHead>
                    <TableHead className="w-[160px]">Workspace role</TableHead>
                    <TableHead className="w-[140px]">Sent</TableHead>
                    <TableHead className="w-[120px]">Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {pendingInvitations.map((inv) => (
                    <TableRow key={inv.id}>
                      <TableCell className="font-medium">{inv.email}</TableCell>
                      <TableCell><Badge variant="outline" className="text-xs">{workspaceRoleLabel(inv.role)}</Badge></TableCell>
                      <TableCell className="text-muted-foreground">
                        {new Date(inv.created_at).toLocaleDateString()}
                      </TableCell>
                      <TableCell>
                        <Button variant="ghost" size="sm" aria-label={`Manage invitation for ${inv.email}`} onClick={() => setManagingInvitation(inv)}>Manage</Button>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          </div>
        )}

        {members.length === 0 ? (
          <div className="flex min-h-[320px] flex-col items-center justify-center gap-3 rounded-lg border border-dashed border-border text-center">
            <div className="space-y-1">
              <p className="font-medium">No members yet</p>
              <p className="text-sm text-muted-foreground">
                Invite teammates to give them access to this workspace.
              </p>
            </div>
            {editable && (
              <Button onClick={openInviteDialog}>
                <PlusSignIcon className="h-4 w-4 mr-1" />
                Invite Member
              </Button>
            )}
          </div>
        ) : (
          <div className="overflow-hidden rounded-lg border border-border">
            <Table>
              <TableHeader>
                <TableRow className="bg-muted/40 hover:bg-muted/40">
                  <TableHead className="min-w-[240px]">Member</TableHead>
                  <TableHead className="w-[160px]">Workspace role</TableHead>
                  <TableHead className="min-w-[160px]">Teams</TableHead>
                  <TableHead className="min-w-[240px]">Module access</TableHead>
                  <TableHead className="w-[64px] text-center">2FA</TableHead>
                  {editable && <TableHead className="w-[144px] text-right"><span className="sr-only">Actions</span></TableHead>}
                </TableRow>
              </TableHeader>
              <TableBody>
                {filteredMembers.map((member) => {
                  const memberTeams = teamsByUserId.get(member.user_id) ?? [];
                  const workspaceWide = member.role === 'owner' || member.role === 'admin';
                  const modules = getMemberModuleAccess(member.id, member.role, memberTeams, moduleAccess.data?.grants ?? []);
                  const canEdit = canEditMemberRole(member) || canRemoveMember(member) || (editable && canManageTeams && !workspaceWide) || (canManageModuleAccess && !workspaceWide);
                  return (
                    <TableRow key={member.id} className="group/member">
                      <TableCell className="py-4">
                        <div className="flex items-center gap-3">
                          <UserAvatar name={member.full_name || member.email} avatarUrl={member.avatar_url} avatarStyle={member.avatar_style} avatarSeed={member.avatar_seed} avatarBackgroundMode={member.avatar_background_mode} avatarBackgroundColor={member.avatar_background_color} presenceStatus={memberPresenceByUserId?.get(member.user_id)?.status ?? null} className="h-9 w-9 shrink-0" fallbackClassName="text-xs" />
                          <div className="min-w-0 max-w-[260px]">
                            <p className="truncate text-sm font-medium">{member.full_name || member.email.split('@')[0]}{member.user_id === user?.id && <span className="ml-1.5 text-xs font-normal text-muted-foreground">(you)</span>}</p>
                            <p className="mt-0.5 truncate text-xs text-muted-foreground" title={member.email}>{member.email}</p>
                          </div>
                        </div>
                      </TableCell>
                      <TableCell><span className="text-sm">{workspaceRoleLabel(member.role).replace('Workspace ', '')}</span></TableCell>
                      <TableCell>
                        {workspaceWide ? <span className="text-sm text-muted-foreground">All teams</span> : memberTeams.length ? (
                          <div className="flex max-w-[240px] flex-wrap gap-1.5">{memberTeams.map(team => <Badge key={team.id} variant="outline" className="max-w-full text-xs font-normal"><span className="truncate">{team.name}</span></Badge>)}</div>
                        ) : <span className="text-sm text-muted-foreground">No teams</span>}
                      </TableCell>
                      <TableCell>
                        {canManageModuleAccess && moduleAccess.isLoading && !workspaceWide ? <Skeleton className="h-6 w-36" /> : !workspaceWide && (!canManageModuleAccess || moduleAccess.isError) ? (
                          <span className="text-xs text-muted-foreground">{canManageModuleAccess ? 'Access unavailable' : 'Access restricted'}</span>
                        ) : (
                          <div className="flex max-w-[300px] flex-wrap gap-1.5">{modules.map(item => <QuickTooltip key={item.module} label={item.description}><Badge variant="secondary" className="text-[11px] font-normal">{item.label}</Badge></QuickTooltip>)}</div>
                        )}
                      </TableCell>
                      <TableCell className="text-center">
                        <QuickTooltip label={member.two_fa_enabled ? 'Two-factor authentication enabled' : 'Two-factor authentication not enabled'}>
                          <span aria-label={member.two_fa_enabled ? '2FA enabled' : '2FA not enabled'} className={cn('inline-flex items-center justify-center', member.two_fa_enabled ? 'text-emerald-600' : 'text-muted-foreground/60')}>
                            {member.two_fa_enabled ? <SecurityCheckIcon className="h-4 w-4" /> : <Shield01Icon className="h-4 w-4" />}
                          </span>
                        </QuickTooltip>
                      </TableCell>
                      {editable && <TableCell className="text-right">
                        <div className="flex justify-end gap-1 transition-opacity [@media(hover:hover)]:opacity-0 group-hover/member:opacity-100 group-focus-within/member:opacity-100">
                          {canEdit && <Button variant="ghost" size="sm" disabled={removingMemberId === member.id || (canManageModuleAccess && moduleAccess.isLoading)} aria-label={`Edit ${member.full_name || member.email}`} onClick={() => setEditingMember(member)}>Edit</Button>}
                          {canRemoveMember(member) && <Button variant="ghost" size="sm" className="text-destructive hover:bg-destructive/10 hover:text-destructive" disabled={removingMemberId !== null} aria-label={`Remove ${member.full_name || member.email}`} onClick={() => setRemovingMember(member)}>{removingMemberId === member.id ? 'Removing…' : 'Remove'}</Button>}
                        </div>
                      </TableCell>}
                    </TableRow>
                  );
                })}
                {filteredMembers.length === 0 && <TableRow><TableCell colSpan={editable ? 6 : 5} className="py-12 text-center text-sm text-muted-foreground">No members match your search.</TableCell></TableRow>}
              </TableBody>
            </Table>
          </div>
        )}
      </div>

      <ConfirmDialog
        open={removingMember !== null}
        onOpenChange={open => { if (!open) setRemovingMember(null); }}
        title="Remove member?"
        description={`${removingMember?.full_name || removingMember?.email || 'This member'} will lose access to this workspace. This does not delete their account.`}
        confirmLabel="Remove member"
        onConfirm={() => {
          if (!removingMember || removingMemberId || !canRemoveMember(removingMember)) return;
          const member = removingMember;
          setRemovingMember(null);
          setRemovingMemberId(member.id);
          void (async () => {
            try {
              const { error } = await workspacesService.removeMember(workspaceId, member.id);
              if (error) throw new Error(error);
              setMembers(current => current.filter(item => item.id !== member.id));
              toast.success('Member removed from workspace');
              void onRefresh?.();
            } catch (error) {
              toast.error(error instanceof Error ? error.message : 'Could not remove member.');
            } finally {
              setRemovingMemberId(null);
            }
          })();
        }}
      />

      {editingMember && <EditMemberDialog
        key={editingMember.id}
        workspaceId={workspaceId}
        member={editingMember}
        teams={teams}
        initialTeamIds={(teamsByUserId.get(editingMember.user_id) ?? []).map(team => team.id)}
        grants={moduleAccess.data?.grants ?? null}
        canEditRole={canEditMemberRole(editingMember)}
        canEditTeams={editable && canManageTeams}
        isSelf={editingMember.user_id === user?.id}
        canEditModules={canManageModuleAccess}
        roleOptions={roleOptions(editingMember)}
        onClose={() => setEditingMember(null)}
        onApplied={role => {
          setMembers(current => current.map(member => member.id === editingMember.id ? { ...member, role } : member));
          void onRefresh?.();
        }}
      />}

      <Dialog open={managingInvitation !== null} onOpenChange={open => { if (!open && !invitationBusy) setManagingInvitation(null); }}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader><DialogTitle>Manage invitation</DialogTitle><DialogDescription>{managingInvitation?.email}</DialogDescription></DialogHeader>
          <div className="space-y-4 py-2">
            <div className="flex justify-between text-sm"><span className="text-muted-foreground">Workspace role</span><span>{workspaceRoleLabel(managingInvitation?.role ?? '')}</span></div>
            {managingInvitation?.join_url && <Button variant="outline" className="w-full" onClick={() => handleCopyLink(managingInvitation.join_url!)}><Copy01Icon className="mr-2 h-4 w-4" />Copy invite link</Button>}
            <Button variant="outline" className="w-full" disabled={invitationBusy} onClick={async () => { if (!managingInvitation) return; setInvitationBusy(true); try { await handleResend(managingInvitation.id); } finally { setInvitationBusy(false); } }}><ArrowReloadHorizontalIcon className="mr-2 h-4 w-4" />Resend invitation</Button>
            <div className="border-t pt-4"><Button variant="ghost" className="w-full text-destructive hover:text-destructive" disabled={invitationBusy} onClick={() => setRevokeInvitationConfirm(true)}>Revoke invitation</Button></div>
          </div>
          <DialogFooter><Button variant="outline" disabled={invitationBusy} onClick={() => setManagingInvitation(null)}>Done</Button></DialogFooter>
        </DialogContent>
      </Dialog>
      <ConfirmDialog open={revokeInvitationConfirm} onOpenChange={setRevokeInvitationConfirm} title="Revoke invitation?" description={`The invite link for ${managingInvitation?.email ?? 'this person'} will no longer work.`} confirmLabel="Revoke invitation" onConfirm={() => {
        if (!managingInvitation) return;
        setRevokeInvitationConfirm(false);
        setInvitationBusy(true);
        void handleRevoke(managingInvitation.id).finally(() => setInvitationBusy(false));
      }} />

      <Dialog open={inviteOpen} onOpenChange={closeInviteDialog}>
        <DialogContent className="sm:max-w-2xl">
          {createdJoinUrl ? (
            <>
              <DialogHeader>
                <DialogTitle>Invitation Sent</DialogTitle>
              </DialogHeader>
              <div className="space-y-4 py-4">
                <p className="text-sm text-muted-foreground">Share this link with <span className="font-medium text-foreground">{pendingInviteEmails[0]}</span> to join the workspace.</p>
                <div className="flex gap-2">
                  <Input value={createdJoinUrl} readOnly className="bg-muted text-xs" />
                  <Button type="button" variant="outline" size="icon" onClick={() => handleCopyLink(createdJoinUrl)}>
                    <Copy01Icon className="h-4 w-4" />
                  </Button>
                </div>
              </div>
              <DialogFooter>
                <Button type="button" onClick={closeInviteDialog}>Done</Button>
              </DialogFooter>
            </>
          ) : (
            <form onSubmit={handleInvite}>
              <DialogHeader>
                <DialogTitle>Invite Members</DialogTitle>
              </DialogHeader>
              <div className="space-y-4 py-4">
                <div className="space-y-2">
                  <div className="flex items-center justify-between">
                    <Label>Emails</Label>
                    {availableOrgMembers.length > 0 && (
                      <Popover>
                        <PopoverTrigger asChild>
                          <button type="button" className="inline-flex items-center gap-1 text-xs text-primary hover:text-primary/80 transition-colors">
                            <UserGroupIcon className="h-3 w-3" />
                            Add from organization
                          </button>
                        </PopoverTrigger>
                        <PopoverContent align="end" className="w-72 p-0">
                          <Command>
                            <CommandInput placeholder="Search members..." />
                            <CommandList>
                              <CommandEmpty>No members found.</CommandEmpty>
                              <CommandGroup>
                                {availableOrgMembers.map((m) => {
                                  const alreadyAdded = pendingInviteEmails.includes(m.email.toLowerCase());
                                  return (
                                    <CommandItem
                                      key={m.user_id}
                                      value={`${m.full_name ?? ''} ${m.email}`}
                                      disabled={alreadyAdded}
                                      onSelect={() => {
                                        if (!alreadyAdded) {
                                          setInvEmails((prev) => mergeEmailChips(prev, [m.email]));
                                        }
                                      }}
                                      className={alreadyAdded ? 'opacity-40' : ''}
                                    >
                                      <UserAvatar
                                        name={m.full_name || m.email}
                                        avatarUrl={m.avatar_url}
                                        avatarStyle={m.avatar_style}
                                        avatarSeed={m.avatar_seed}
                                        avatarBackgroundMode={m.avatar_background_mode}
                                        avatarBackgroundColor={m.avatar_background_color}
                                        className="h-6 w-6"
                                        fallbackClassName="text-[10px]"
                                      />
                                      <div className="min-w-0 flex-1">
                                        {m.full_name && <p className="truncate text-xs font-medium">{m.full_name}</p>}
                                        <p className="truncate text-xs text-muted-foreground">{m.email}</p>
                                      </div>
                                      {alreadyAdded && <span className="text-[10px] text-muted-foreground shrink-0">Added</span>}
                                    </CommandItem>
                                  );
                                })}
                              </CommandGroup>
                            </CommandList>
                          </Command>
                        </PopoverContent>
                      </Popover>
                    )}
                  </div>
                  <EmailChipInput
                    placeholder="name@example.com, name2@example.com"
                    value={invEmails}
                    onValueChange={setInvEmails}
                    inputValue={invEmailInput}
                    onInputValueChange={setInvEmailInput}
                  />
                  <p className="text-xs text-muted-foreground">Press comma, Enter, or Tab to turn each email into a chip. You can also paste a list.</p>
                </div>
                <div className="space-y-2">
                  <Label>Workspace role</Label>
                  <div className="space-y-2">
                    {([
                      { value: 'admin', label: workspaceRoleLabel('admin'), description: 'Full access across all teams. Can manage settings, workflows, labels, and members.' },
                      { value: 'member', label: workspaceRoleLabel('member'), description: 'Can create and edit tasks in their teams. Can be promoted to team manager to manage epics, sprints, and objectives.' },
                      { value: 'viewer', label: workspaceRoleLabel('viewer'), description: 'Read-only access to tasks, epics, and sprints in their assigned teams only.' },
                    ] as const).map((role) => (
                      <button
                        key={role.value}
                        type="button"
                        onClick={() => { setInvRole(role.value); if (role.value === 'admin') setSelectedTeamIds([]); }}
                        className={cn(
                          'flex w-full items-start gap-3 rounded-md border p-3 text-left transition-colors',
                          invRole === role.value
                            ? 'border-primary bg-primary/5'
                            : 'border-border hover:bg-muted/50'
                        )}
                      >
                        <div className={cn(
                          'mt-0.5 h-4 w-4 shrink-0 rounded-full border-2',
                          invRole === role.value
                            ? 'border-primary bg-primary'
                            : 'border-muted-foreground/40'
                        )} />
                        <div className="min-w-0">
                          <p className="text-sm font-medium">{role.label}</p>
                          <p className="text-xs text-muted-foreground">{role.description}</p>
                        </div>
                      </button>
                    ))}
                  </div>
                </div>
                {invRole !== 'admin' && (
                <div className="space-y-2">
                  <Label>Teams <span className="text-muted-foreground font-normal">(optional)</span></Label>
                  {teams.length > 0 ? (
                    <>
                      <p className="text-xs text-muted-foreground">Auto-assign to teams when the invite is accepted.</p>
                      <div className="flex flex-wrap gap-1.5">
                        {teams.map((team) => {
                          const selected = selectedTeamIds.includes(team.id);
                          return (
                            <button
                              key={team.id}
                              type="button"
                              onClick={() =>
                                setSelectedTeamIds((prev) =>
                                  selected
                                    ? prev.filter((id) => id !== team.id)
                                    : [...prev, team.id],
                                )
                              }
                              className={`rounded-md border px-2.5 py-1 text-xs transition-colors ${
                                selected
                                  ? 'border-primary bg-primary/10 text-primary font-medium'
                                  : 'border-border text-muted-foreground hover:border-primary/50 hover:text-foreground'
                              }`}
                            >
                              {team.name}
                            </button>
                          );
                        })}
                      </div>
                    </>
                  ) : (
                    <p className="text-xs text-muted-foreground">No teams created yet. You can assign this member to teams later from the Teams settings.</p>
                  )}
                </div>
                )}
              </div>
              <DialogFooter>
                <Button type="button" variant="outline" onClick={closeInviteDialog}>Cancel</Button>
                <Button type="submit" disabled={sending || hasInvalidInviteInput}>{sending ? 'Sending invites...' : 'Send Invites'}</Button>
              </DialogFooter>
            </form>
          )}
        </DialogContent>
      </Dialog>
    </>
  );
}
