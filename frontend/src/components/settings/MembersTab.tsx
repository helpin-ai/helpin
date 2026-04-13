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
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Command, CommandInput, CommandList, CommandEmpty, CommandGroup, CommandItem } from '@/components/ui/command';
import { cn } from '@/lib/utils';
import { Copy01Icon, PlusSignIcon, ArrowReloadHorizontalIcon, Search01Icon, Delete01Icon, UserGroupIcon } from '@/lib/icons';
import { toast } from 'sonner';
import { useAuthStore } from '@/stores/authStore';
import { ConfirmDialog } from '@/components/pm/ConfirmDialog';

export function MembersTab({ workspaceId, organizationId, editable, teams, userMemberships }: {
  workspaceId: string;
  organizationId?: string;
  editable: boolean;
  teams: WorkspaceTeam[];
  userMemberships: TeamUserMembership[];
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
  const [updatingMemberId, setUpdatingMemberId] = useState<string | null>(null);
  const [removingMemberId, setRemovingMemberId] = useState<string | null>(null);
  const [removeMemberConfirm, setRemoveMemberConfirm] = useState<MemberWithUser | null>(null);
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

  const teamNamesByUserId = useMemo(() => {
    const teamNameById = new Map(teams.map((team) => [team.id, team.name]));
    const memberships = new Map<string, string[]>();

    userMemberships.forEach((membership) => {
      const teamName = teamNameById.get(membership.team_id);
      if (!teamName) return;
      const current = memberships.get(membership.user_id) ?? [];
      current.push(teamName);
      memberships.set(membership.user_id, current);
    });

    memberships.forEach((names, userId) => {
      memberships.set(userId, [...names].sort((a, b) => a.localeCompare(b)));
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
    const failedEmails: string[] = [];
    let lastJoinUrl: string | null = null;

    await Promise.all(
      emails.map(async (email) => {
        const { data, error } = await inviteService.send({ workspace_id: workspaceId, email, role: invRole });
        if (error) {
          failedEmails.push(email);
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

    if (sent > 0 && failedEmails.length > 0) {
      toast.warning(`${sent} of ${emails.length} invitations sent. Failed: ${failedEmails.join(', ')}`);
    } else if (sent > 0) {
      toast.success(`${sent} invitation${sent === 1 ? '' : 's'} sent`);
    } else {
      toast.error(`Failed to send invitations: ${failedEmails.join(', ')}`);
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
      { value: 'admin', label: 'Admin' },
      { value: 'member', label: 'Member' },
      { value: 'viewer', label: 'Viewer' },
    ];
    if (actorRole === 'owner') {
      options.unshift({ value: 'owner', label: 'Owner' });
    }
    if (actorRole !== 'owner' && member.role === 'admin') {
      return [{ value: 'admin', label: 'Admin' }];
    }
    return options;
  };

  const handleUpdateRole = async (member: MemberWithUser, role: 'owner' | 'admin' | 'member' | 'viewer') => {
    if (role === member.role) return;
    setUpdatingMemberId(member.id);
    const { error } = await workspacesService.updateMemberRole(workspaceId, member.id, { role });
    setUpdatingMemberId(null);
    if (error) {
      toast.error(error);
      return;
    }
    setMembers((prev) => prev.map((current) => current.id === member.id ? { ...current, role } : current));
    toast.success('Member role updated');
  };

  const canRemoveMember = (member: MemberWithUser) => {
    if (!editable || !user) return false;
    if (member.user_id === user.id) return false;
    if (actorRole !== 'owner' && actorRole !== 'admin') return false;
    if (member.role === 'owner') return false;
    if (member.role === 'admin' && actorRole !== 'owner') return false;
    return true;
  };

  const handleRemoveMember = async (member: MemberWithUser) => {
    setRemovingMemberId(member.id);
    const { error } = await workspacesService.removeMember(workspaceId, member.id);
    setRemovingMemberId(null);
    if (error) {
      toast.error(error);
      return;
    }
    setMembers((prev) => prev.filter((current) => current.id !== member.id));
    toast.success('Member removed from workspace');
  };

  if (loading) return <Skeleton className="h-96" />;

  return (
    <>
      <div className="space-y-6">
        <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
          <span className="text-sm text-muted-foreground">{members.length} {members.length === 1 ? 'member' : 'members'} in this workspace</span>
          <div className="flex items-center gap-3">
            {members.length > 10 && (
              <div className="relative">
                <Search01Icon className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  value={query}
                  onChange={(event) => setQuery(event.target.value)}
                  className="h-9 w-60 pl-9"
                  placeholder="Search members..."
                />
              </div>
            )}
            {editable && (
              <Button size="sm" onClick={openInviteDialog}>
                <PlusSignIcon className="h-4 w-4 mr-1" /> Invite Member
              </Button>
            )}
          </div>
        </div>

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
                  <TableHead className="w-[280px]">Name</TableHead>
                  <TableHead>Email</TableHead>
                  <TableHead>Teams</TableHead>
                  <TableHead className="w-[120px]">Role</TableHead>
                  {editable && <TableHead className="w-[72px] text-right">Actions</TableHead>}
                </TableRow>
              </TableHeader>
              <TableBody>
                {filteredMembers.map((m) => {
                  const teamNames = teamNamesByUserId.get(m.user_id) ?? [];
                  const hasWorkspaceWideTeamAccess = m.role === 'owner' || m.role === 'admin';
                  const hasAllTeams = hasWorkspaceWideTeamAccess || (teams.length > 0 && teamNames.length === teams.length);
                  const presenceStatus = memberPresenceByUserId?.get(m.user_id)?.status ?? null;

                  return (
                    <TableRow key={m.id}>
                      <TableCell>
                        <div className="flex items-center gap-3">
                          <UserAvatar
                            name={m.full_name || m.email}
                            avatarUrl={m.avatar_url}
                            avatarStyle={m.avatar_style}
                            avatarSeed={m.avatar_seed}
                            avatarBackgroundMode={m.avatar_background_mode}
                            avatarBackgroundColor={m.avatar_background_color}
                            presenceStatus={presenceStatus}
                            className="h-8 w-8"
                            fallbackClassName="text-[10px]"
                          />
                          <div className="min-w-0">
                            <p className="truncate font-medium">{m.full_name || '—'}</p>
                            {!m.full_name && (
                              <p className="truncate text-xs text-muted-foreground">{m.email}</p>
                            )}
                          </div>
                        </div>
                      </TableCell>
                      <TableCell className="text-muted-foreground">{m.email}</TableCell>
                      <TableCell>
                        {hasAllTeams ? (
                          <Badge variant="outline" className="text-xs font-normal">
                            All teams
                          </Badge>
                        ) : teamNames.length > 0 ? (
                          <div className="flex flex-wrap gap-1.5">
                            {teamNames.map((teamName) => (
                              <Badge key={`${m.user_id}-${teamName}`} variant="outline" className="text-xs font-normal">
                                {teamName}
                              </Badge>
                            ))}
                          </div>
                        ) : (
                          <span className="text-sm text-muted-foreground">-</span>
                        )}
                      </TableCell>
                      <TableCell>
                        {canEditMemberRole(m) ? (
                          <Select
                            value={m.role}
                            onValueChange={(value) => void handleUpdateRole(m, value as 'owner' | 'admin' | 'member' | 'viewer')}
                            disabled={updatingMemberId === m.id}
                          >
                            <SelectTrigger size="sm" className="h-7 w-[116px] px-2.5 text-xs">
                              <SelectValue />
                            </SelectTrigger>
                            <SelectContent className="text-xs">
                              {roleOptions(m).map((option) => (
                                <SelectItem key={`${m.id}-${option.value}`} value={option.value} className="py-1 text-xs">
                                  {option.label}
                                </SelectItem>
                              ))}
                            </SelectContent>
                          </Select>
                        ) : (
                          <Badge variant={m.role === 'owner' ? 'default' : 'outline'} className="text-xs">{m.role}</Badge>
                        )}
                      </TableCell>
                      {editable && (
                        <TableCell className="text-right">
                          {canRemoveMember(m) ? (
                            <Button
                              variant="ghost"
                              size="sm"
                              className="h-8 w-8 p-0 text-muted-foreground hover:text-destructive"
                              disabled={removingMemberId === m.id}
                              onClick={() => setRemoveMemberConfirm(m)}
                            >
                              <Delete01Icon className="h-3.5 w-3.5" />
                            </Button>
                          ) : null}
                        </TableCell>
                      )}
                    </TableRow>
                  );
                })}
                {filteredMembers.length === 0 && (
                  <TableRow>
                    <TableCell colSpan={editable ? 5 : 4} className="py-8 text-center text-sm text-muted-foreground">
                      No members match your search.
                    </TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          </div>
        )}

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
                    <TableHead className="w-[120px]">Role</TableHead>
                    <TableHead className="w-[140px]">Sent</TableHead>
                    <TableHead className="w-[120px]">Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {pendingInvitations.map((inv) => (
                    <TableRow key={inv.id}>
                      <TableCell className="font-medium">{inv.email}</TableCell>
                      <TableCell><Badge variant="outline" className="text-xs">{inv.role}</Badge></TableCell>
                      <TableCell className="text-muted-foreground">
                        {new Date(inv.created_at).toLocaleDateString()}
                      </TableCell>
                      <TableCell>
                        <div className="flex gap-1">
                          {inv.join_url && (
                            <QuickTooltip label="Copy invite link">
                              <Button size="icon" variant="ghost" onClick={() => handleCopyLink(inv.join_url!)}>
                                <Copy01Icon className="h-3.5 w-3.5" />
                              </Button>
                            </QuickTooltip>
                          )}
                          <QuickTooltip label="Resend">
                            <Button size="icon" variant="ghost" onClick={() => handleResend(inv.id)}>
                              <ArrowReloadHorizontalIcon className="h-3.5 w-3.5" />
                            </Button>
                          </QuickTooltip>
                          <QuickTooltip label="Revoke">
                            <Button size="icon" variant="ghost" onClick={() => handleRevoke(inv.id)}>
                              <Delete01Icon className="h-3.5 w-3.5" />
                            </Button>
                          </QuickTooltip>
                        </div>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          </div>
        )}
      </div>

      <ConfirmDialog
        open={removeMemberConfirm !== null}
        onOpenChange={(open) => {
          if (!open) {
            setRemoveMemberConfirm(null);
          }
        }}
        title="Remove member"
        description={(
          <>
            This will remove <span className="font-medium text-foreground">{removeMemberConfirm?.full_name || removeMemberConfirm?.email || 'this member'}</span> from this workspace.
            They will lose access immediately.
          </>
        )}
        confirmLabel={removingMemberId === removeMemberConfirm?.id ? 'Removing...' : 'Remove'}
        variant="destructive"
        onConfirm={() => {
          if (!removeMemberConfirm) return;
          void handleRemoveMember(removeMemberConfirm);
          setRemoveMemberConfirm(null);
        }}
      />

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
                  <Label>Role</Label>
                  <div className="space-y-2">
                    {([
                      { value: 'admin', label: 'Admin', description: 'Full access across all teams. Can manage settings, workflows, labels, and members.' },
                      { value: 'member', label: 'Member', description: 'Can create and edit tasks in their teams. Can be promoted to team manager to manage epics, sprints, and objectives.' },
                      { value: 'viewer', label: 'Viewer', description: 'Read-only access to tasks, epics, and sprints in their assigned teams only.' },
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
