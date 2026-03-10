import { useEffect, useMemo, useRef, useState, type FormEvent } from 'react';
import { useCopyToClipboard } from '@/hooks/useCopyToClipboard';
import { useOrganizationMembers } from '@/hooks/queries';
import { workspacesService } from '@/lib/services/workspacesService';
import { inviteService } from '@/lib/services/inviteService';
import type { MemberWithUser, Invitation } from '@/lib/types';
import { UserAvatar } from '@/components/pm/UserAvatar';
import { QuickTooltip } from '@/components/ui/quick-tooltip';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Skeleton } from '@/components/ui/skeleton';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { cn } from '@/lib/utils';
import { Copy, Plus, RefreshCw, Trash2 } from 'lucide-react';
import { toast } from 'sonner';
import { LINEAR_CARD_CLASS } from './settingsConstants';

export function MembersTab({ workspaceId, organizationId, editable }: {
  workspaceId: string;
  organizationId?: string;
  editable: boolean;
}) {
  const [members, setMembers] = useState<MemberWithUser[]>([]);
  const [invitations, setInvitations] = useState<Invitation[]>([]);
  const [loading, setLoading] = useState(true);
  const [inviteOpen, setInviteOpen] = useState(false);
  const [invEmail, setInvEmail] = useState('');
  const [invRole, setInvRole] = useState('member');
  const [sending, setSending] = useState(false);
  const [createdJoinUrl, setCreatedJoinUrl] = useState<string | null>(null);
  const [showSuggestions, setShowSuggestions] = useState(false);
  const suggestionsRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  const { data: orgMembers } = useOrganizationMembers(organizationId);

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

  // Filter suggestions based on email input
  const filteredSuggestions = useMemo(() => {
    if (!invEmail) return availableOrgMembers;
    const q = invEmail.toLowerCase();
    return availableOrgMembers.filter(
      (m) => m.email.toLowerCase().includes(q) || (m.full_name && m.full_name.toLowerCase().includes(q))
    );
  }, [availableOrgMembers, invEmail]);

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
    setInvEmail('');
    setInvRole('member');
    setInviteOpen(true);
  };

  const closeInviteDialog = () => {
    setInviteOpen(false);
    if (createdJoinUrl) loadData();
  };

  const handleInvite = async (e: FormEvent) => {
    e.preventDefault();
    setSending(true);
    const { data, error } = await inviteService.send({ workspace_id: workspaceId, email: invEmail, role: invRole });
    setSending(false);
    if (error) {
      toast.error(error);
    } else {
      toast.success(`Invitation sent to ${invEmail}`);
      if (data?.join_url) {
        setCreatedJoinUrl(data.join_url);
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

  if (loading) return <Skeleton className="h-96" />;

  return (
    <Card className={LINEAR_CARD_CLASS}>
      <CardHeader>
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <CardTitle className="text-base">Members</CardTitle>
            <Badge variant="outline" className="text-xs font-normal">{members.length}</Badge>
          </div>
          {editable && (
            <Button size="sm" onClick={openInviteDialog}>
              <Plus className="h-4 w-4 mr-1" /> Invite Member
            </Button>
          )}
        </div>
      </CardHeader>
      <CardContent>
        {members.length === 0 ? (
          <p className="text-sm text-muted-foreground text-center py-6">No members yet.</p>
        ) : (
          <div className="overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead>
                  <TableHead>Email</TableHead>
                  <TableHead>Role</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {members.map((m) => (
                  <TableRow key={m.id}>
                    <TableCell className="font-medium">
                      <div className="flex items-center gap-2.5">
                        <UserAvatar name={m.full_name || m.email} className="h-7 w-7" fallbackClassName="text-[10px]" />
                        {m.full_name || '—'}
                      </div>
                    </TableCell>
                    <TableCell className="text-muted-foreground">{m.email}</TableCell>
                    <TableCell>
                      <Badge variant={m.role === 'owner' ? 'default' : 'outline'} className="text-xs">{m.role}</Badge>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        )}

        {/* Pending Invitations */}
        {editable && invitations.filter((inv) => inv.status === 'pending').length > 0 && (
          <div className="mt-6">
            <h4 className="text-sm font-medium mb-2">Pending Invitations</h4>
            <div className="overflow-x-auto">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Email</TableHead>
                    <TableHead>Role</TableHead>
                    <TableHead>Sent</TableHead>
                    <TableHead className="w-24">Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {invitations.filter((inv) => inv.status === 'pending').map((inv) => (
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
                                <Copy className="h-3.5 w-3.5" />
                              </Button>
                            </QuickTooltip>
                          )}
                          <QuickTooltip label="Resend">
                            <Button size="icon" variant="ghost" onClick={() => handleResend(inv.id)}>
                              <RefreshCw className="h-3.5 w-3.5" />
                            </Button>
                          </QuickTooltip>
                          <QuickTooltip label="Revoke">
                            <Button size="icon" variant="ghost" onClick={() => handleRevoke(inv.id)}>
                              <Trash2 className="h-3.5 w-3.5" />
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
      </CardContent>

      <Dialog open={inviteOpen} onOpenChange={closeInviteDialog}>
        <DialogContent>
          {createdJoinUrl ? (
            <>
              <DialogHeader>
                <DialogTitle>Invitation Sent</DialogTitle>
              </DialogHeader>
              <div className="space-y-4 py-4">
                <p className="text-sm text-muted-foreground">Share this link with <span className="font-medium text-foreground">{invEmail}</span> to join the workspace.</p>
                <div className="flex gap-2">
                  <Input value={createdJoinUrl} readOnly className="bg-muted text-xs" />
                  <Button type="button" variant="outline" size="icon" onClick={() => handleCopyLink(createdJoinUrl)}>
                    <Copy className="h-4 w-4" />
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
                <DialogTitle>Invite Member</DialogTitle>
              </DialogHeader>
              <div className="space-y-4 py-4">
                <div className="space-y-2">
                  <Label>Email</Label>
                  <div className="relative">
                    <Input
                      ref={inputRef}
                      type="email"
                      placeholder="colleague@example.com"
                      value={invEmail}
                      onChange={(e) => { setInvEmail(e.target.value); setShowSuggestions(true); }}
                      onFocus={() => setShowSuggestions(true)}
                      onBlur={() => { setTimeout(() => setShowSuggestions(false), 150); }}
                      required
                      autoComplete="off"
                    />
                    {showSuggestions && filteredSuggestions.length > 0 && (
                      <div ref={suggestionsRef} className="absolute z-50 top-full left-0 right-0 mt-1 max-h-48 overflow-y-auto rounded-md border bg-popover shadow-md">
                        {availableOrgMembers.length > 0 && !invEmail && (
                          <div className="px-3 py-1.5 text-xs text-muted-foreground font-medium border-b">
                            Organization members
                          </div>
                        )}
                        {filteredSuggestions.map((m) => (
                          <button
                            key={m.user_id}
                            type="button"
                            className="flex w-full items-center gap-2.5 px-3 py-2 text-left text-sm hover:bg-accent transition-colors"
                            onMouseDown={(e) => {
                              e.preventDefault();
                              setInvEmail(m.email);
                              setShowSuggestions(false);
                            }}
                          >
                            <UserAvatar name={m.full_name || m.email} className="h-6 w-6" fallbackClassName="text-[10px]" />
                            <div className="min-w-0 flex-1">
                              {m.full_name && <p className="truncate text-sm font-medium">{m.full_name}</p>}
                              <p className="truncate text-xs text-muted-foreground">{m.email}</p>
                            </div>
                          </button>
                        ))}
                      </div>
                    )}
                  </div>
                </div>
                <div className="space-y-2">
                  <Label>Role</Label>
                  <div className="space-y-2">
                    {([
                      { value: 'admin', label: 'Admin', description: 'Full access to all settings, members, billing, and workspace configuration.' },
                      { value: 'manager', label: 'Manager', description: 'Can manage teams, projects, sprints, and view performance data.' },
                      { value: 'member', label: 'Member', description: 'Can create and edit stories, epics, and participate in sprints.' },
                      { value: 'viewer', label: 'Viewer', description: 'Read-only access. Can view projects and dashboards but cannot make changes.' },
                    ] as const).map((role) => (
                      <button
                        key={role.value}
                        type="button"
                        onClick={() => setInvRole(role.value)}
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
              </div>
              <DialogFooter>
                <Button type="button" variant="outline" onClick={closeInviteDialog}>Cancel</Button>
                <Button type="submit" disabled={sending}>{sending ? 'Sending...' : 'Send Invite'}</Button>
              </DialogFooter>
            </form>
          )}
        </DialogContent>
      </Dialog>
    </Card>
  );
}
