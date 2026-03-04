import { useEffect, useState, type FormEvent } from 'react';
import { useTitle } from '@/hooks/useTitle';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useSessionStore } from '@/stores/sessionStore';
import { settingsService } from '@/lib/services/settingsService';
import { workspacesService } from '@/lib/services/workspacesService';
import { inviteService } from '@/lib/services/inviteService';
import type { WorkspaceSettings, WorkspaceTeam, WorkspacePerson, JobRoleCriteria, BonusTierConfig, MemberWithUser, Invitation } from '@/lib/types';
import { pmWorkflowService } from '@/lib/services/pmWorkflowService';
import { StateTypeIcon } from '@/lib/pmConstants';
import type { StateType, WorkflowState, WorkflowWithStates } from '@/lib/pmTypes';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { Skeleton } from '@/components/ui/skeleton';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { ArrowDown, ArrowUp, Award, Briefcase, Copy, GitBranch, ListTree, Pencil, Plus, RefreshCw, Settings2, Trash2, UserPlus, Users, type LucideIcon } from 'lucide-react';
import { toast } from 'sonner';

export type SettingsSection = 'members' | 'teams' | 'people' | 'jobroles' | 'tiers' | 'workflows' | 'workflowstates' | 'system';

export const SETTINGS_SECTIONS: { id: SettingsSection; label: string; description: string; icon: LucideIcon }[] = [
  {
    id: 'members',
    label: 'Members',
    description: 'Manage workspace members and invitations.',
    icon: Users,
  },
  {
    id: 'teams',
    label: 'Teams',
    description: 'Create teams and define managers.',
    icon: Users,
  },
  {
    id: 'people',
    label: 'People',
    description: 'Edit HR details, job roles, and compensation for workspace members.',
    icon: UserPlus,
  },
  {
    id: 'jobroles',
    label: 'Job Roles',
    description: 'Configure role-based individual evaluation criteria.',
    icon: Briefcase,
  },
  {
    id: 'tiers',
    label: 'Bonus Tiers',
    description: 'Set score bands and multipliers for payouts.',
    icon: Award,
  },
  {
    id: 'workflows',
    label: 'Workflows',
    description: 'Configure team workflows and ownership behavior.',
    icon: GitBranch,
  },
  {
    id: 'workflowstates',
    label: 'Workflow States',
    description: 'Manage state columns and rules within workflows.',
    icon: ListTree,
  },
  {
    id: 'system',
    label: 'System',
    description: 'Control global workspace behavior and defaults.',
    icon: Settings2,
  },
];

export const isSettingsSection = (value: string): value is SettingsSection =>
  SETTINGS_SECTIONS.some((section) => section.id === value);

const LINEAR_CARD_CLASS = 'rounded-none border-border shadow-none';

export default function Settings({ section }: { section: SettingsSection }) {
  useTitle('Settings');
  const { currentWorkspace } = useWorkspaceStore();
  const { isAdmin } = useSessionStore();
  const [settings, setSettings] = useState<WorkspaceSettings | null>(null);
  const [loading, setLoading] = useState(true);

  const load = async () => {
    const ws = useWorkspaceStore.getState().currentWorkspace;
    if (!ws?.id) {
      setSettings(null);
      setLoading(false);
      return;
    }
    setLoading(true);
    try {
      const { data } = await settingsService.getAll(ws.id);
      if (data) setSettings(data);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(); }, [currentWorkspace?.id]);

  if (loading) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-10 w-48" />
        <Skeleton className="h-96" />
      </div>
    );
  }

  if (!settings) {
    return (
      <div className="text-center py-12">
        <p className="text-muted-foreground">Could not load workspace settings.</p>
      </div>
    );
  }

  const workspaceId = currentWorkspace?.id ?? settings.settings.workspace_id;
  if (!workspaceId) {
    return (
      <div className="text-center py-12">
        <p className="text-muted-foreground">Could not determine workspace for settings.</p>
      </div>
    );
  }

  const sectionMeta = SETTINGS_SECTIONS.find((candidate) => candidate.id === section)!;

  const renderSection = () => {
    switch (section) {
      case 'members':
        return (
          <MembersTab
            workspaceId={workspaceId}
            editable={isAdmin()}
          />
        );
      case 'teams':
        return (
          <TeamsTab
            workspaceId={workspaceId}
            teams={settings.teams}
            editable={isAdmin()}
            onRefresh={load}
          />
        );
      case 'people':
        return (
          <PeopleTab
            workspaceId={workspaceId}
            people={settings.people}
            editable={isAdmin()}
            onRefresh={load}
          />
        );
      case 'jobroles':
        return (
          <JobRolesTab
            workspaceId={workspaceId}
            criteria={settings.job_role_criteria}
            editable={isAdmin()}
            onRefresh={load}
          />
        );
      case 'tiers':
        return (
          <BonusTiersTab
            workspaceId={workspaceId}
            tiers={settings.bonus_tiers}
            editable={isAdmin()}
            onRefresh={load}
          />
        );
      case 'system':
        return (
          <SystemTab
            workspaceId={workspaceId}
            config={settings.settings}
            editable={isAdmin()}
            onRefresh={load}
          />
        );
      case 'workflows':
        return (
          <WorkflowsTab
            workspaceId={workspaceId}
            teams={settings.teams}
            editable={isAdmin()}
          />
        );
      case 'workflowstates':
        return (
          <WorkflowStatesTab
            workspaceId={workspaceId}
            editable={isAdmin()}
          />
        );
      default:
        return null;
    }
  };

  return (
    <div className="space-y-4">
      <div>
        <h2 className="text-xl font-semibold">{sectionMeta.label}</h2>
        <p className="text-sm text-muted-foreground">{sectionMeta.description}</p>
      </div>
      {renderSection()}
    </div>
  );
}

/* ============ Members Tab ============ */

function MembersTab({ workspaceId, editable }: {
  workspaceId: string;
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

  const handleCopyLink = async (joinUrl: string) => {
    try {
      if (navigator.clipboard) {
        await navigator.clipboard.writeText(joinUrl);
      } else {
        const textarea = document.createElement('textarea');
        textarea.value = joinUrl;
        textarea.style.position = 'fixed';
        textarea.style.opacity = '0';
        document.body.appendChild(textarea);
        textarea.select();
        document.execCommand('copy');
        document.body.removeChild(textarea);
      }
      toast.success('Invite link copied to clipboard');
    } catch {
      toast.error('Failed to copy link');
    }
  };

  const handleResend = async (id: string) => {
    const { error } = await inviteService.resend(id);
    if (error) toast.error(error);
    else { toast.success('Invitation resent'); loadData(); }
  };

  const handleRevoke = async (id: string) => {
    const { error } = await inviteService.revoke(id);
    if (error) toast.error(error);
    else { toast.success('Invitation revoked'); loadData(); }
  };

  if (loading) return <Skeleton className="h-96" />;

  return (
    <Card className={LINEAR_CARD_CLASS}>
      <CardHeader>
        <div className="flex items-center justify-between">
          <div>
            <CardTitle className="text-base">Members</CardTitle>
            <CardDescription>{members.length} member{members.length !== 1 ? 's' : ''}</CardDescription>
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
                    <TableCell className="font-medium">{m.full_name || '—'}</TableCell>
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
                            <Button size="icon" variant="ghost" onClick={() => handleCopyLink(inv.join_url!)} title="Copy invite link">
                              <Copy className="h-3.5 w-3.5" />
                            </Button>
                          )}
                          <Button size="icon" variant="ghost" onClick={() => handleResend(inv.id)} title="Resend">
                            <RefreshCw className="h-3.5 w-3.5" />
                          </Button>
                          <Button size="icon" variant="ghost" onClick={() => handleRevoke(inv.id)} title="Revoke">
                            <Trash2 className="h-3.5 w-3.5" />
                          </Button>
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
                  <Input type="email" placeholder="colleague@example.com" value={invEmail} onChange={(e) => setInvEmail(e.target.value)} required />
                </div>
                <div className="space-y-2">
                  <Label>Role</Label>
                  <Select value={invRole} onValueChange={setInvRole}>
                    <SelectTrigger><SelectValue /></SelectTrigger>
                    <SelectContent>
                      <SelectItem value="admin">Admin</SelectItem>
                      <SelectItem value="manager">Manager</SelectItem>
                      <SelectItem value="member">Member</SelectItem>
                      <SelectItem value="viewer">Viewer</SelectItem>
                    </SelectContent>
                  </Select>
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

/* ============ Teams Tab ============ */

function TeamsTab({ workspaceId, teams, editable, onRefresh }: {
  workspaceId: string;
  teams: WorkspaceTeam[];
  editable: boolean;
  onRefresh: () => void;
}) {
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editTeam, setEditTeam] = useState<WorkspaceTeam | null>(null);
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [saving, setSaving] = useState(false);

  const openCreate = () => {
    setEditTeam(null);
    setName('');
    setDescription('');
    setDialogOpen(true);
  };

  const openEdit = (team: WorkspaceTeam) => {
    setEditTeam(team);
    setName(team.name);
    setDescription(team.description ?? '');
    setDialogOpen(true);
  };

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setSaving(true);
    if (editTeam) {
      const { error } = await settingsService.updateTeam(editTeam.id, { name, description: description || undefined });
      if (error) toast.error(error);
      else { toast.success('Team updated'); setDialogOpen(false); onRefresh(); }
    } else {
      const { error } = await settingsService.createTeam({ workspace_id: workspaceId, name, description: description || undefined });
      if (error) toast.error(error);
      else { toast.success('Team created'); setDialogOpen(false); onRefresh(); }
    }
    setSaving(false);
  };

  const handleDelete = async (id: string) => {
    const { error } = await settingsService.deleteTeam(id);
    if (error) toast.error(error);
    else { toast.success('Team deleted'); onRefresh(); }
  };

  return (
    <Card className={LINEAR_CARD_CLASS}>
      <CardHeader>
        <div className="flex items-center justify-between">
          <div>
            <CardTitle className="text-base">Teams</CardTitle>
            <CardDescription>{teams.length} team{teams.length !== 1 ? 's' : ''}</CardDescription>
          </div>
          {editable && (
            <Button size="sm" onClick={openCreate}>
              <Plus className="h-4 w-4 mr-1" /> Add Team
            </Button>
          )}
        </div>
      </CardHeader>
      <CardContent>
        {teams.length === 0 ? (
          <p className="text-sm text-muted-foreground text-center py-6">No teams yet.</p>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Description</TableHead>
                {editable && <TableHead className="w-24">Actions</TableHead>}
              </TableRow>
            </TableHeader>
            <TableBody>
              {teams.map(team => (
                <TableRow key={team.id}>
                  <TableCell className="font-medium">{team.name}</TableCell>
                  <TableCell className="text-muted-foreground">{team.description || '--'}</TableCell>
                  {editable && (
                    <TableCell>
                      <div className="flex gap-1">
                        <Button size="icon" variant="ghost" onClick={() => openEdit(team)}>
                          <Pencil className="h-3.5 w-3.5" />
                        </Button>
                        <Button size="icon" variant="ghost" onClick={() => handleDelete(team.id)}>
                          <Trash2 className="h-3.5 w-3.5" />
                        </Button>
                      </div>
                    </TableCell>
                  )}
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </CardContent>

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent>
          <form onSubmit={handleSubmit}>
            <DialogHeader>
              <DialogTitle>{editTeam ? 'Edit Team' : 'Add Team'}</DialogTitle>
            </DialogHeader>
            <div className="space-y-4 py-4">
              <div className="space-y-2">
                <Label>Name</Label>
                <Input value={name} onChange={e => setName(e.target.value)} required />
              </div>
              <div className="space-y-2">
                <Label>Description</Label>
                <Input value={description} onChange={e => setDescription(e.target.value)} />
              </div>
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setDialogOpen(false)}>Cancel</Button>
              <Button type="submit" disabled={saving}>{saving ? 'Saving...' : 'Save'}</Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </Card>
  );
}

/* ============ People Tab ============ */

function PeopleTab({ workspaceId, people, editable, onRefresh }: {
  workspaceId: string;
  people: WorkspacePerson[];
  editable: boolean;
  onRefresh: () => void;
}) {
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editPerson, setEditPerson] = useState<WorkspacePerson | null>(null);
  const [role, setRole] = useState<WorkspacePerson['role']>('employee');
  const [jobRole, setJobRole] = useState('');
  const [salary, setSalary] = useState('0');
  const [saving, setSaving] = useState(false);

  const openEdit = (p: WorkspacePerson) => {
    setEditPerson(p);
    setRole(p.role); setJobRole(p.job_role); setSalary(String(p.base_salary));
    setDialogOpen(true);
  };

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    if (!editPerson) return;
    setSaving(true);
    const { error } = await settingsService.updatePerson(editPerson.id, {
      role, job_role: jobRole, base_salary: Number(salary),
    });
    if (error) toast.error(error);
    else { toast.success('Person updated'); setDialogOpen(false); onRefresh(); }
    setSaving(false);
  };

  const handleDelete = async (id: string) => {
    const { error } = await settingsService.deletePerson(id);
    if (error) toast.error(error);
    else { toast.success('Person removed'); onRefresh(); }
  };

  return (
    <Card className={LINEAR_CARD_CLASS}>
      <CardHeader>
        <div>
          <CardTitle className="text-base">People</CardTitle>
          <CardDescription>{people.length} member{people.length !== 1 ? 's' : ''} — invite new members from the Members tab</CardDescription>
        </div>
      </CardHeader>
      <CardContent>
        {people.length === 0 ? (
          <p className="text-sm text-muted-foreground text-center py-6">No people yet. Invite members from the Members tab to get started.</p>
        ) : (
          <div className="overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead>
                  <TableHead>Email</TableHead>
                  <TableHead>Role</TableHead>
                  <TableHead>Job Role</TableHead>
                  <TableHead>Status</TableHead>
                  {editable && <TableHead className="w-24">Actions</TableHead>}
                </TableRow>
              </TableHeader>
              <TableBody>
                {people.map(p => (
                  <TableRow key={p.id}>
                    <TableCell className="font-medium">{p.name}</TableCell>
                    <TableCell className="text-muted-foreground">{p.email}</TableCell>
                    <TableCell><Badge variant="outline" className="text-xs">{p.role}</Badge></TableCell>
                    <TableCell>{p.job_role || '—'}</TableCell>
                    <TableCell>
                      <Badge variant={p.status === 'active' ? 'default' : 'secondary'} className="text-xs">
                        {p.status}
                      </Badge>
                    </TableCell>
                    {editable && (
                      <TableCell>
                        <div className="flex gap-1">
                          <Button size="icon" variant="ghost" onClick={() => openEdit(p)}>
                            <Pencil className="h-3.5 w-3.5" />
                          </Button>
                          <Button size="icon" variant="ghost" onClick={() => handleDelete(p.id)}>
                            <Trash2 className="h-3.5 w-3.5" />
                          </Button>
                        </div>
                      </TableCell>
                    )}
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        )}
      </CardContent>

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent>
          <form onSubmit={handleSubmit}>
            <DialogHeader>
              <DialogTitle>Edit Person</DialogTitle>
            </DialogHeader>
            <div className="space-y-4 py-4">
              <div className="space-y-2">
                <Label>Name</Label>
                <Input value={editPerson?.name ?? ''} disabled className="bg-muted" />
              </div>
              <div className="space-y-2">
                <Label>Email</Label>
                <Input value={editPerson?.email ?? ''} disabled className="bg-muted" />
              </div>
              <div className="space-y-2">
                <Label>Role</Label>
                <Select value={role} onValueChange={v => setRole(v as WorkspacePerson['role'])}>
                  <SelectTrigger><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="executive">Executive</SelectItem>
                    <SelectItem value="manager">Manager</SelectItem>
                    <SelectItem value="employee">Employee</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-2">
                <Label>Job Role</Label>
                <Input placeholder="e.g. Frontend Developer" value={jobRole} onChange={e => setJobRole(e.target.value)} />
              </div>
              <div className="space-y-2">
                <Label>Base Salary</Label>
                <Input type="number" value={salary} onChange={e => setSalary(e.target.value)} />
              </div>
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setDialogOpen(false)}>Cancel</Button>
              <Button type="submit" disabled={saving}>{saving ? 'Saving...' : 'Save'}</Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </Card>
  );
}

/* ============ Job Roles Tab ============ */

function JobRolesTab({ workspaceId, criteria, editable, onRefresh }: {
  workspaceId: string;
  criteria: JobRoleCriteria[];
  editable: boolean;
  onRefresh: () => void;
}) {
  // Group criteria by job_role
  const grouped = criteria.reduce<Record<string, JobRoleCriteria[]>>((acc, c) => {
    if (!acc[c.job_role]) acc[c.job_role] = [];
    acc[c.job_role].push(c);
    return acc;
  }, {});
  const jobRoles = Object.keys(grouped).sort();

  const handleDeleteRole = async (jobRole: string) => {
    const { error } = await settingsService.deleteJobRole(workspaceId, jobRole);
    if (error) toast.error(error);
    else { toast.success(`"${jobRole}" criteria deleted`); onRefresh(); }
  };

  return (
    <Card className={LINEAR_CARD_CLASS}>
      <CardHeader>
        <CardTitle className="text-base">Job Role Criteria</CardTitle>
        <CardDescription>Individual scoring criteria by job role</CardDescription>
      </CardHeader>
      <CardContent>
        {jobRoles.length === 0 ? (
          <p className="text-sm text-muted-foreground text-center py-6">No job role criteria configured.</p>
        ) : (
          <div className="space-y-4">
            {jobRoles.map(role => (
              <div key={role} className="border border-border rounded-none p-4">
                <div className="flex items-center justify-between mb-3">
                  <h3 className="font-medium">{role}</h3>
                  <div className="flex items-center gap-2">
                    <Badge variant="outline" className="text-xs">
                      {grouped[role].length} criteria
                    </Badge>
                    {editable && (
                      <Button size="icon" variant="ghost" onClick={() => handleDeleteRole(role)}>
                        <Trash2 className="h-3.5 w-3.5" />
                      </Button>
                    )}
                  </div>
                </div>
                <div className="space-y-1">
                  {grouped[role].map(c => (
                    <div key={c.criteria_id} className="flex items-center justify-between text-sm py-1">
                      <div>
                        <span className="font-medium">{c.name}</span>
                        <span className="text-muted-foreground ml-2">- {c.question}</span>
                      </div>
                      <Badge variant={c.enabled ? 'default' : 'secondary'} className="text-xs">
                        {c.enabled ? 'Enabled' : 'Disabled'}
                      </Badge>
                    </div>
                  ))}
                </div>
              </div>
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  );
}

/* ============ Bonus Tiers Tab ============ */

function BonusTiersTab({ workspaceId, tiers, editable, onRefresh }: {
  workspaceId: string;
  tiers: BonusTierConfig[];
  editable: boolean;
  onRefresh: () => void;
}) {
  const [editing, setEditing] = useState(false);
  const [localTiers, setLocalTiers] = useState(tiers);
  const [saving, setSaving] = useState(false);

  useEffect(() => { setLocalTiers(tiers); }, [tiers]);

  const handleSave = async () => {
    setSaving(true);
    const { error } = await settingsService.updateBonusTiers(
      workspaceId,
      localTiers.map(t => ({
        tier: t.tier,
        min_score: t.min_score,
        max_score: t.max_score,
        salary_multiplier: t.salary_multiplier,
        description: t.description,
        editable: t.editable,
      })),
    );
    setSaving(false);
    if (error) toast.error(error);
    else { toast.success('Bonus tiers updated'); setEditing(false); onRefresh(); }
  };

  const updateTier = (idx: number, field: string, value: string) => {
    setLocalTiers(prev => prev.map((t, i) => i === idx ? { ...t, [field]: field === 'description' ? value : Number(value) } : t));
  };

  const tierVariant = (tier: string): 'default' | 'secondary' | 'destructive' => {
    switch (tier) {
      case 'A': return 'default';
      case 'B': return 'secondary';
      default: return 'destructive';
    }
  };

  return (
    <Card className={LINEAR_CARD_CLASS}>
      <CardHeader>
        <div className="flex items-center justify-between">
          <div>
            <CardTitle className="text-base">Bonus Tiers</CardTitle>
            <CardDescription>Configure A/B/C tier thresholds and multipliers</CardDescription>
          </div>
          {editable && !editing && (
            <Button size="sm" variant="outline" onClick={() => setEditing(true)}>
              <Pencil className="h-3.5 w-3.5 mr-1" /> Edit
            </Button>
          )}
        </div>
      </CardHeader>
      <CardContent>
        <div className="space-y-4">
          {localTiers.map((tier, idx) => (
            <div key={tier.tier} className="border border-border rounded-none p-4 space-y-3">
              <div className="flex items-center gap-2">
                <Badge variant={tierVariant(tier.tier)}>Tier {tier.tier}</Badge>
                {tier.description && <span className="text-sm text-muted-foreground">{tier.description}</span>}
              </div>
              {editing ? (
                <div className="grid grid-cols-3 gap-3">
                  <div className="space-y-1">
                    <Label className="text-xs">Min Score</Label>
                    <Input type="number" value={tier.min_score} onChange={e => updateTier(idx, 'min_score', e.target.value)} />
                  </div>
                  <div className="space-y-1">
                    <Label className="text-xs">Max Score</Label>
                    <Input type="number" value={tier.max_score} onChange={e => updateTier(idx, 'max_score', e.target.value)} />
                  </div>
                  <div className="space-y-1">
                    <Label className="text-xs">Multiplier</Label>
                    <Input type="number" step="0.1" value={tier.salary_multiplier} onChange={e => updateTier(idx, 'salary_multiplier', e.target.value)} />
                  </div>
                </div>
              ) : (
                <div className="flex gap-6 text-sm">
                  <span>Score: {tier.min_score} - {tier.max_score}</span>
                  <span>Multiplier: {tier.salary_multiplier}x</span>
                </div>
              )}
            </div>
          ))}
        </div>
        {editing && (
          <div className="flex gap-2 mt-4 justify-end">
            <Button variant="outline" onClick={() => { setEditing(false); setLocalTiers(tiers); }}>Cancel</Button>
            <Button onClick={handleSave} disabled={saving}>{saving ? 'Saving...' : 'Save'}</Button>
          </div>
        )}
      </CardContent>
    </Card>
  );
}

/* ============ System Tab ============ */

function SystemTab({ workspaceId, config, editable, onRefresh }: {
  workspaceId: string;
  config: WorkspaceSettings['settings'];
  editable: boolean;
  onRefresh: () => void;
}) {
  const [sprintDuration, setSprintDuration] = useState(config.sprint_duration_weeks);
  const [teamWeight, setTeamWeight] = useState(config.team_weight);
  const [notifications, setNotifications] = useState(config.notifications_enabled);
  const [autoCalc, setAutoCalc] = useState(config.auto_calculate_bonuses);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    setSprintDuration(config.sprint_duration_weeks);
    setTeamWeight(config.team_weight);
    setNotifications(config.notifications_enabled);
    setAutoCalc(config.auto_calculate_bonuses);
  }, [config]);

  const handleSave = async () => {
    setSaving(true);
    const { error } = await settingsService.updateSystem(workspaceId, {
      sprint_duration_weeks: sprintDuration,
      team_weight: teamWeight,
      notifications_enabled: notifications,
      auto_calculate_bonuses: autoCalc,
    });
    setSaving(false);
    if (error) toast.error(error);
    else { toast.success('System settings updated'); onRefresh(); }
  };

  return (
    <Card className={LINEAR_CARD_CLASS}>
      <CardHeader>
        <CardTitle className="text-base">System Settings</CardTitle>
        <CardDescription>General workspace configuration</CardDescription>
      </CardHeader>
      <CardContent className="space-y-6">
        <div className="grid gap-4 sm:grid-cols-2">
          <div className="space-y-2">
            <Label>Sprint Duration (weeks)</Label>
            <Input
              type="number"
              min="1"
              max="4"
              value={sprintDuration}
              onChange={e => setSprintDuration(Number(e.target.value))}
              disabled={!editable}
            />
          </div>
          <div className="space-y-2">
            <Label>Team Weight (%)</Label>
            <Input
              type="number"
              min="0"
              max="100"
              value={teamWeight}
              onChange={e => setTeamWeight(Number(e.target.value))}
              disabled={!editable}
            />
            <p className="text-xs text-muted-foreground">Individual weight: {100 - teamWeight}%</p>
          </div>
        </div>

        <div className="space-y-4">
          <div className="flex items-center justify-between">
            <div>
              <Label>Notifications</Label>
              <p className="text-xs text-muted-foreground">Send email notifications for sprint events</p>
            </div>
            <Switch checked={notifications} onCheckedChange={setNotifications} disabled={!editable} />
          </div>
          <div className="flex items-center justify-between">
            <div>
              <Label>Auto-calculate Bonuses</Label>
              <p className="text-xs text-muted-foreground">Automatically recalculate bonuses when scores change</p>
            </div>
            <Switch checked={autoCalc} onCheckedChange={setAutoCalc} disabled={!editable} />
          </div>
        </div>

        {editable && (
          <div className="flex justify-end">
            <Button onClick={handleSave} disabled={saving}>
              {saving ? 'Saving...' : 'Save Settings'}
            </Button>
          </div>
        )}
      </CardContent>
    </Card>
  );
}

/* ============ Workflows Tab ============ */

function WorkflowsTab({ workspaceId, teams, editable }: {
  workspaceId: string;
  teams: WorkspaceTeam[];
  editable: boolean;
}) {
  const [workflows, setWorkflows] = useState<WorkflowWithStates[]>([]);
  const [loading, setLoading] = useState(true);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editWorkflow, setEditWorkflow] = useState<WorkflowWithStates | null>(null);
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [teamID, setTeamID] = useState<string>('none');
  const [autoAssignOwner, setAutoAssignOwner] = useState(false);
  const [saving, setSaving] = useState(false);

  const loadWorkflows = async () => {
    setLoading(true);
    const { data, error } = await pmWorkflowService.list(workspaceId);
    if (error) {
      toast.error(error);
      setWorkflows([]);
    } else {
      setWorkflows(data ?? []);
    }
    setLoading(false);
  };

  useEffect(() => { loadWorkflows(); }, [workspaceId]);

  const openCreate = () => {
    setEditWorkflow(null);
    setName('');
    setDescription('');
    setTeamID('none');
    setAutoAssignOwner(false);
    setDialogOpen(true);
  };

  const openEdit = (workflow: WorkflowWithStates) => {
    setEditWorkflow(workflow);
    setName(workflow.workflow.name);
    setDescription(workflow.workflow.description ?? '');
    setTeamID(workflow.workflow.team_id ?? 'none');
    setAutoAssignOwner(workflow.workflow.auto_assign_owner);
    setDialogOpen(true);
  };

  const handleSave = async (e: FormEvent) => {
    e.preventDefault();
    if (!name.trim()) return;
    setSaving(true);

    const normalizedTeamID = teamID === 'none' ? undefined : teamID;
    if (editWorkflow) {
      const { error } = await pmWorkflowService.update(workspaceId, editWorkflow.workflow.id, {
        name: name.trim(),
        description: description.trim() || undefined,
        team_id: normalizedTeamID,
        auto_assign_owner: autoAssignOwner,
      });
      if (error) toast.error(error);
      else {
        toast.success('Workflow updated');
        setDialogOpen(false);
        await loadWorkflows();
      }
    } else {
      const { error } = await pmWorkflowService.create({
        workspace_id: workspaceId,
        name: name.trim(),
        description: description.trim() || undefined,
        team_id: normalizedTeamID,
        auto_assign_owner: autoAssignOwner,
      });
      if (error) toast.error(error);
      else {
        toast.success('Workflow created');
        setDialogOpen(false);
        await loadWorkflows();
      }
    }
    setSaving(false);
  };

  const handleDelete = async (workflowID: string) => {
    const { error } = await pmWorkflowService.remove(workspaceId, workflowID);
    if (error) toast.error(error);
    else {
      toast.success('Workflow deleted');
      await loadWorkflows();
    }
  };

  const findTeamName = (id?: string) => {
    if (!id) return 'Workspace Default';
    return teams.find((team) => team.id === id)?.name ?? 'Unknown Team';
  };

  return (
    <Card className={LINEAR_CARD_CLASS}>
      <CardHeader>
        <div className="flex items-center justify-between">
          <div>
            <CardTitle className="text-base">Workflows</CardTitle>
            <CardDescription>{workflows.length} workflow{workflows.length !== 1 ? 's' : ''}</CardDescription>
          </div>
          {editable && (
            <Button size="sm" onClick={openCreate}>
              <Plus className="h-4 w-4 mr-1" /> Create Workflow
            </Button>
          )}
        </div>
      </CardHeader>
      <CardContent>
        {loading ? (
          <p className="text-sm text-muted-foreground text-center py-6">Loading workflows...</p>
        ) : workflows.length === 0 ? (
          <p className="text-sm text-muted-foreground text-center py-6">No workflows yet.</p>
        ) : (
          <div className="space-y-2">
            {workflows.map((workflow) => (
              <div key={workflow.workflow.id} className="rounded-none border border-border px-4 py-3">
                <div className="flex items-center justify-between gap-3">
                  <div>
                    <div className="flex items-center gap-2">
                      <p className="font-medium">{workflow.workflow.name}</p>
                      {!workflow.workflow.team_id && (
                        <Badge variant="secondary" className="text-xs">Workspace Default</Badge>
                      )}
                    </div>
                    <p className="text-sm text-muted-foreground">
                      {workflow.workflow.description || 'No description'}
                    </p>
                  </div>
                  <div className="flex items-center gap-2 text-sm text-muted-foreground">
                    <span>{workflow.states.length} states</span>
                    <span>•</span>
                    <span>{findTeamName(workflow.workflow.team_id)}</span>
                    <span>•</span>
                    <span>{workflow.workflow.auto_assign_owner ? 'Auto-assign owner' : 'Manual owner'}</span>
                  </div>
                </div>
                {editable && (
                  <div className="mt-2 flex justify-end gap-1">
                    <Button size="icon" variant="ghost" onClick={() => openEdit(workflow)}>
                      <Pencil className="h-3.5 w-3.5" />
                    </Button>
                    <Button size="icon" variant="ghost" onClick={() => handleDelete(workflow.workflow.id)}>
                      <Trash2 className="h-3.5 w-3.5" />
                    </Button>
                  </div>
                )}
              </div>
            ))}
          </div>
        )}
      </CardContent>

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent>
          <form onSubmit={handleSave}>
            <DialogHeader>
              <DialogTitle>{editWorkflow ? 'Edit Workflow' : 'Create Workflow'}</DialogTitle>
            </DialogHeader>
            <div className="space-y-4 py-4">
              <div className="space-y-2">
                <Label>Name</Label>
                <Input value={name} onChange={(e) => setName(e.target.value)} required />
              </div>
              <div className="space-y-2">
                <Label>Description</Label>
                <Input value={description} onChange={(e) => setDescription(e.target.value)} />
              </div>
              <div className="space-y-2">
                <Label>Team</Label>
                <Select value={teamID} onValueChange={setTeamID}>
                  <SelectTrigger><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="none">Workspace Default</SelectItem>
                    {teams.map((team) => (
                      <SelectItem key={team.id} value={team.id}>{team.name}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="flex items-center justify-between rounded-none border border-border px-3 py-2">
                <div>
                  <Label>Auto assign owner when moved to started state</Label>
                  <p className="text-xs text-muted-foreground">Assign current user when story enters a started state and has no owner.</p>
                </div>
                <Switch checked={autoAssignOwner} onCheckedChange={setAutoAssignOwner} />
              </div>
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setDialogOpen(false)}>Cancel</Button>
              <Button type="submit" disabled={saving}>{saving ? 'Saving...' : 'Save'}</Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </Card>
  );
}

/* ============ Workflow States Tab ============ */

const STATE_TYPE_ORDER: StateType[] = ['backlog', 'unstarted', 'started', 'done'];
const STATE_TYPE_LABEL: Record<StateType, string> = {
  backlog: 'Backlog',
  unstarted: 'Unstarted',
  started: 'Started',
  done: 'Done',
};

function WorkflowStatesTab({ workspaceId, editable }: {
  workspaceId: string;
  editable: boolean;
}) {
  const [workflows, setWorkflows] = useState<WorkflowWithStates[]>([]);
  const [selectedWorkflowID, setSelectedWorkflowID] = useState<string>('');
  const [loading, setLoading] = useState(true);

  const [dialogOpen, setDialogOpen] = useState(false);
  const [editState, setEditState] = useState<WorkflowState | null>(null);
  const [newStateType, setNewStateType] = useState<StateType>('unstarted');
  const [stateName, setStateName] = useState('');
  const [stateDescription, setStateDescription] = useState('');
  const [stateColor, setStateColor] = useState('');
  const [stateWIP, setStateWIP] = useState('');
  const [stateDefault, setStateDefault] = useState(false);
  const [saving, setSaving] = useState(false);

  const loadWorkflows = async () => {
    setLoading(true);
    const { data, error } = await pmWorkflowService.list(workspaceId);
    if (error) {
      toast.error(error);
      setWorkflows([]);
      setLoading(false);
      return;
    }
    const next = data ?? [];
    setWorkflows(next);
    setSelectedWorkflowID((prev) => {
      if (prev && next.some((workflow) => workflow.workflow.id === prev)) return prev;
      return next[0]?.workflow.id ?? '';
    });
    setLoading(false);
  };

  useEffect(() => { loadWorkflows(); }, [workspaceId]);

  const selectedWorkflow = workflows.find((workflow) => workflow.workflow.id === selectedWorkflowID) ?? null;
  const sortedStates = selectedWorkflow
    ? [...selectedWorkflow.states].sort((a, b) => a.position - b.position)
    : [];

  const statesByType: Record<StateType, WorkflowState[]> = {
    backlog: sortedStates.filter((state) => state.state_type === 'backlog'),
    unstarted: sortedStates.filter((state) => state.state_type === 'unstarted'),
    started: sortedStates.filter((state) => state.state_type === 'started'),
    done: sortedStates.filter((state) => state.state_type === 'done'),
  };

  const orderedStateIDs = (workflow: WorkflowWithStates) =>
    STATE_TYPE_ORDER.flatMap((type) =>
      [...workflow.states]
        .filter((state) => state.state_type === type)
        .sort((a, b) => a.position - b.position)
        .map((state) => state.id)
    );

  const normalizeOrdering = async (workflow: WorkflowWithStates) => {
    const targetIDs = orderedStateIDs(workflow);
    const currentIDs = [...workflow.states].sort((a, b) => a.position - b.position).map((state) => state.id);
    if (targetIDs.length === currentIDs.length && targetIDs.every((id, idx) => currentIDs[idx] === id)) return;
    await pmWorkflowService.reorderStates(workspaceId, workflow.workflow.id, targetIDs);
  };

  const openCreateForType = (type: StateType) => {
    setEditState(null);
    setNewStateType(type);
    setStateName('');
    setStateDescription('');
    setStateColor('');
    setStateWIP('');
    setStateDefault(false);
    setDialogOpen(true);
  };

  const openEdit = (state: WorkflowState) => {
    setEditState(state);
    setNewStateType(state.state_type);
    setStateName(state.name);
    setStateDescription(state.description ?? '');
    setStateColor(state.color ?? '');
    setStateWIP(state.wip_limit ? String(state.wip_limit) : '');
    setStateDefault(state.is_default);
    setDialogOpen(true);
  };

  const handleSaveState = async (e: FormEvent) => {
    e.preventDefault();
    if (!selectedWorkflow || !stateName.trim()) return;
    setSaving(true);
    const payload = {
      name: stateName.trim(),
      state_type: newStateType,
      description: stateDescription.trim() || undefined,
      color: stateColor.trim() || undefined,
      wip_limit: stateWIP.trim() ? Number(stateWIP) : undefined,
      is_default: stateDefault,
    };

    if (editState) {
      const { error } = await pmWorkflowService.updateState(workspaceId, selectedWorkflow.workflow.id, editState.id, payload);
      if (error) toast.error(error);
      else toast.success('State updated');
    } else {
      const { error } = await pmWorkflowService.createState(workspaceId, selectedWorkflow.workflow.id, payload);
      if (error) toast.error(error);
      else {
        toast.success('State created');
        const refreshed = await pmWorkflowService.get(workspaceId, selectedWorkflow.workflow.id);
        if (refreshed.data) {
          await normalizeOrdering(refreshed.data);
        }
      }
    }
    await loadWorkflows();
    setDialogOpen(false);
    setSaving(false);
  };

  const handleDeleteState = async () => {
    if (!selectedWorkflow || !editState) return;
    const { error } = await pmWorkflowService.removeState(workspaceId, selectedWorkflow.workflow.id, editState.id);
    if (error) toast.error(error);
    else {
      toast.success('State deleted');
      setDialogOpen(false);
      await loadWorkflows();
    }
  };

  const handleMoveWithinType = async (type: StateType, stateID: string, direction: 'up' | 'down') => {
    if (!selectedWorkflow) return;
    const typed = [...statesByType[type]];
    const idx = typed.findIndex((state) => state.id === stateID);
    if (idx < 0) return;
    const swapIdx = direction === 'up' ? idx - 1 : idx + 1;
    if (swapIdx < 0 || swapIdx >= typed.length) return;
    const copy = [...typed];
    const [current] = copy.splice(idx, 1);
    copy.splice(swapIdx, 0, current);

    const idsByType: Record<StateType, string[]> = {
      backlog: statesByType.backlog.map((state) => state.id),
      unstarted: statesByType.unstarted.map((state) => state.id),
      started: statesByType.started.map((state) => state.id),
      done: statesByType.done.map((state) => state.id),
    };
    idsByType[type] = copy.map((state) => state.id);
    const nextIDs = STATE_TYPE_ORDER.flatMap((stateType) => idsByType[stateType]);

    const { error } = await pmWorkflowService.reorderStates(workspaceId, selectedWorkflow.workflow.id, nextIDs);
    if (error) toast.error(error);
    else await loadWorkflows();
  };

  const toggleAutoAssignOwner = async (checked: boolean) => {
    if (!selectedWorkflow) return;
    const { error } = await pmWorkflowService.update(workspaceId, selectedWorkflow.workflow.id, {
      auto_assign_owner: checked,
    });
    if (error) toast.error(error);
    else await loadWorkflows();
  };

  return (
    <Card className={LINEAR_CARD_CLASS}>
      <CardHeader>
        <CardTitle className="text-base">Workflow States</CardTitle>
        <CardDescription>Manage state columns grouped by backlog/unstarted/started/done.</CardDescription>
      </CardHeader>
      <CardContent className="space-y-5">
        {loading ? (
          <p className="text-sm text-muted-foreground">Loading workflows...</p>
        ) : workflows.length === 0 ? (
          <p className="text-sm text-muted-foreground">No workflows found. Create one in the Workflows tab first.</p>
        ) : (
          <>
            <div className="grid gap-3 md:grid-cols-2">
              <div className="space-y-2">
                <Label>Workflow</Label>
                <Select value={selectedWorkflowID} onValueChange={setSelectedWorkflowID}>
                  <SelectTrigger><SelectValue /></SelectTrigger>
                  <SelectContent>
                    {workflows.map((workflow) => (
                      <SelectItem key={workflow.workflow.id} value={workflow.workflow.id}>
                        {workflow.workflow.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              {selectedWorkflow && (
                <div className="flex items-end">
                  <div className="flex w-full items-center justify-between rounded-none border border-border px-3 py-2.5">
                    <div>
                      <Label>Auto assign owner</Label>
                      <p className="text-xs text-muted-foreground">Assign current user when stories move into started state without owner.</p>
                    </div>
                    <Switch
                      checked={selectedWorkflow.workflow.auto_assign_owner}
                      disabled={!editable}
                      onCheckedChange={toggleAutoAssignOwner}
                    />
                  </div>
                </div>
              )}
            </div>

            {selectedWorkflow && (
              <div className="space-y-5">
                {STATE_TYPE_ORDER.map((type) => (
                  <section key={type} className="space-y-2">
                    <div className="flex items-center justify-between">
                      <h4 className="flex items-center gap-1.5 text-sm font-medium">
                        <StateTypeIcon stateType={type} className="h-4 w-4" />
                        {STATE_TYPE_LABEL[type]}
                      </h4>
                      {editable && (
                        <Button variant="ghost" size="sm" onClick={() => openCreateForType(type)}>
                          <Plus className="h-3.5 w-3.5 mr-1" /> Add
                        </Button>
                      )}
                    </div>
                    {statesByType[type].length === 0 ? (
                      <div className="rounded-none border border-dashed border-border px-3 py-3 text-sm text-muted-foreground">
                        No states in this group.
                      </div>
                    ) : (
                      <div className="space-y-2">
                        {statesByType[type].map((state, idx) => (
                          <div key={state.id} className="rounded-none border border-border px-3 py-2.5">
                            <div className="flex items-start justify-between gap-3">
                              <div>
                                <div className="flex items-center gap-2">
                                  <StateTypeIcon stateType={state.state_type} className="h-4 w-4" />
                                  <p className="font-medium">{state.name}</p>
                                  {state.is_default && (
                                    <Badge variant="secondary" className="text-xs">Default</Badge>
                                  )}
                                  {state.wip_limit ? (
                                    <Badge variant="outline" className="text-xs">WIP {state.wip_limit}</Badge>
                                  ) : null}
                                </div>
                                <p className="text-sm text-muted-foreground">{state.description || 'No description'}</p>
                              </div>
                              {editable && (
                                <div className="flex items-center gap-1">
                                  <Button
                                    size="icon"
                                    variant="ghost"
                                    disabled={idx === 0}
                                    onClick={() => handleMoveWithinType(type, state.id, 'up')}
                                  >
                                    <ArrowUp className="h-3.5 w-3.5" />
                                  </Button>
                                  <Button
                                    size="icon"
                                    variant="ghost"
                                    disabled={idx === statesByType[type].length - 1}
                                    onClick={() => handleMoveWithinType(type, state.id, 'down')}
                                  >
                                    <ArrowDown className="h-3.5 w-3.5" />
                                  </Button>
                                  <Button size="icon" variant="ghost" onClick={() => openEdit(state)}>
                                    <Pencil className="h-3.5 w-3.5" />
                                  </Button>
                                </div>
                              )}
                            </div>
                          </div>
                        ))}
                      </div>
                    )}
                  </section>
                ))}
              </div>
            )}
          </>
        )}
      </CardContent>

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent>
          <form onSubmit={handleSaveState}>
            <DialogHeader>
              <DialogTitle>{editState ? 'Edit Workflow State' : 'Add Workflow State'}</DialogTitle>
            </DialogHeader>
            <div className="space-y-4 py-4">
              <div className="space-y-2">
                <Label>State Type</Label>
                <Input value={STATE_TYPE_LABEL[newStateType]} disabled />
              </div>
              <div className="space-y-2">
                <Label>State Name</Label>
                <Input value={stateName} onChange={(e) => setStateName(e.target.value)} required />
              </div>
              <div className="space-y-2">
                <Label>Description</Label>
                <Input value={stateDescription} onChange={(e) => setStateDescription(e.target.value)} />
              </div>
              <div className="grid gap-3 sm:grid-cols-2">
                <div className="space-y-2">
                  <Label>Color</Label>
                  <Input value={stateColor} onChange={(e) => setStateColor(e.target.value)} placeholder="#3b82f6" />
                </div>
                <div className="space-y-2">
                  <Label>WIP Limit</Label>
                  <Input type="number" min={0} value={stateWIP} onChange={(e) => setStateWIP(e.target.value)} />
                </div>
              </div>
              <div className="flex items-center justify-between rounded-none border border-border px-3 py-2">
                <div>
                  <Label>Default state</Label>
                  <p className="text-xs text-muted-foreground">Stories are created in this state by default.</p>
                </div>
                <Switch checked={stateDefault} onCheckedChange={setStateDefault} />
              </div>
            </div>
            <DialogFooter className="justify-between">
              <div>
                {editState && editable && (
                  <Button type="button" variant="ghost" className="text-destructive" onClick={handleDeleteState}>
                    <Trash2 className="h-3.5 w-3.5 mr-1" /> Delete State
                  </Button>
                )}
              </div>
              <div className="flex gap-2">
                <Button type="button" variant="outline" onClick={() => setDialogOpen(false)}>Cancel</Button>
                <Button type="submit" disabled={saving || !editable}>{saving ? 'Saving...' : 'Save Changes'}</Button>
              </div>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </Card>
  );
}
