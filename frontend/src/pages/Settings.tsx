import { useCallback, useEffect, useMemo, useState, type FormEvent } from 'react';
import { useCopyToClipboard } from '@/hooks/useCopyToClipboard';
import { useTitle } from '@/hooks/useTitle';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useSessionStore } from '@/stores/sessionStore';
import { settingsService } from '@/lib/services/settingsService';
import { useTeamEstimateStore } from '@/stores/teamEstimateStore';
import { useTeamFieldVisibilityStore } from '@/stores/teamFieldVisibilityStore';
import { workspacesService } from '@/lib/services/workspacesService';
import { inviteService } from '@/lib/services/inviteService';
import type { WorkspaceSettings, WorkspaceTeam, WorkspacePerson, JobRoleCriteria, BonusTierConfig, MemberWithUser, Invitation, TeamUserMembership, InvitationTeamPreassignment, TeamEstimateSettings, TeamFieldVisibility, EstimateScale } from '@/lib/types';
import { pmWorkflowService } from '@/lib/services/pmWorkflowService';
import { pmAutomationService } from '@/lib/services/pmAutomationService';
import { StateTypeIcon } from '@/lib/pmConstants';
import { LabelsSettings } from '@/components/pm/LabelsSettings';
import { UserAvatar } from '@/components/pm/UserAvatar';
import type { StateType, WorkflowState, WorkflowWithStates, EpicWorkflowState, PMAutomation, AutomationType } from '@/lib/pmTypes';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { Skeleton } from '@/components/ui/skeleton';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Textarea } from '@/components/ui/textarea';
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { cn, getInitials } from '@/lib/utils';
import { ArrowDown, ArrowUp, Award, Briefcase, ChevronRight, Copy, Eye, GitBranch, Globe, Info, LayoutGrid, ListTree, Pencil, Plus, RefreshCw, Search, Settings2, Tag, Trash2, UserPlus, Users, X, type LucideIcon } from 'lucide-react';
import { SCALE_LABELS, SCALE_DESCRIPTIONS, getEstimateOptions } from '@/lib/estimateScales';
import { useNavigate } from '@tanstack/react-router';
import { toast } from 'sonner';

export type SettingsSection = 'general' | 'members' | 'teams' | 'people' | 'jobroles' | 'tiers' | 'workflows' | 'workflowstates' | 'labels' | 'automations' | 'system';

export const SETTINGS_SECTIONS: { id: SettingsSection; label: string; description: string; icon: LucideIcon; group: string }[] = [
  {
    id: 'general',
    label: 'General',
    description: 'Workspace name, description, and timezone.',
    icon: Settings2,
    group: 'Workspace',
  },
  {
    id: 'members',
    label: 'Members',
    description: 'Manage workspace members and invitations.',
    icon: Users,
    group: 'Workspace',
  },
  {
    id: 'teams',
    label: 'Teams',
    description: 'Create teams, assign members, and manage team-level PM settings.',
    icon: Users,
    group: 'Workspace',
  },
  {
    id: 'workflows',
    label: 'Workflows',
    description: 'Configure team workflows and ownership behavior.',
    icon: GitBranch,
    group: 'Project Settings',
  },
  {
    id: 'workflowstates',
    label: 'Workflow States',
    description: 'Manage state columns and rules within workflows.',
    icon: ListTree,
    group: 'Project Settings',
  },
  {
    id: 'labels',
    label: 'Labels',
    description: 'Create and manage labels for stories, epics, and sprints.',
    icon: Tag,
    group: 'Project Settings',
  },
  {
    id: 'automations',
    label: 'Automations',
    description: 'Automate epic transitions and sprint management.',
    icon: RefreshCw,
    group: 'Project Settings',
  },
  {
    id: 'people',
    label: 'People',
    description: 'Edit HR details, job roles, and compensation for workspace members.',
    icon: UserPlus,
    group: 'Reward Settings',
  },
  {
    id: 'jobroles',
    label: 'Job Roles',
    description: 'Configure role-based individual evaluation criteria.',
    icon: Briefcase,
    group: 'Reward Settings',
  },
  {
    id: 'tiers',
    label: 'Bonus Tiers',
    description: 'Set score bands and multipliers for payouts.',
    icon: Award,
    group: 'Reward Settings',
  },
  {
    id: 'system',
    label: 'Reward Defaults',
    description: 'Control reward calculation behavior and defaults.',
    icon: Settings2,
    group: 'Reward Settings',
  },
];

export const isSettingsSection = (value: string): value is SettingsSection =>
  SETTINGS_SECTIONS.some((section) => section.id === value);

const LINEAR_CARD_CLASS = 'rounded-none border-border shadow-none';

const slugifyTeamHandle = (value: string) =>
  value
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');

export default function Settings({ section, initialWorkflowId, initialTeamId }: { section: SettingsSection; initialWorkflowId?: string; initialTeamId?: string }) {
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
      if (data) {
        setSettings(data);
        useTeamEstimateStore.getState().setSettings(data.team_estimate_settings ?? []);
        useTeamFieldVisibilityStore.getState().setSettings(data.team_field_visibility ?? []);
      }
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
      case 'general':
        return (
          <GeneralTab
            workspaceId={workspaceId}
            editable={isAdmin()}
          />
        );
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
            userMemberships={settings.user_memberships}
            invitationPreassignments={settings.invitation_team_preassignments}
            teamEstimateSettings={settings.team_estimate_settings}
            teamFieldVisibility={settings.team_field_visibility}
            editable={isAdmin()}
            onRefresh={load}
          />
        );
      case 'people':
        return (
          <PeopleTab
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
            initialTeamId={initialTeamId}
          />
        );
      case 'workflowstates':
        return (
          <WorkflowStatesTab
            workspaceId={workspaceId}
            editable={isAdmin()}
            initialWorkflowId={initialWorkflowId}
          />
        );
      case 'labels':
        return <LabelsSettings workspaceId={workspaceId} initialTeamId={initialTeamId} />;
      case 'automations':
        return <AutomationsTab workspaceId={workspaceId} teams={settings.teams} />;
      default:
        return null;
    }
  };

  return (
    <div className="space-y-4">
      {section !== 'teams' && (
        <div>
          <h2 className="text-xl font-semibold">{sectionMeta.label}</h2>
          <p className="text-sm text-muted-foreground">{sectionMeta.description}</p>
        </div>
      )}
      {renderSection()}
    </div>
  );
}

/* ============ General Tab ============ */

const TIMEZONE_LIST: { id: string; offset: string; searchKey: string }[] = (() => {
  const names = Intl.supportedValuesOf('timeZone');
  const now = new Date();
  return names.map((tz) => {
    const fmt = new Intl.DateTimeFormat('en-US', { timeZone: tz, timeZoneName: 'shortOffset' });
    const parts = fmt.formatToParts(now);
    const gmtStr = parts.find((p) => p.type === 'timeZoneName')?.value ?? '';
    const offset = gmtStr === 'GMT' ? 'UTC+00:00' : gmtStr.replace('GMT', 'UTC');
    return { id: tz, offset, searchKey: `${tz} ${offset}`.toLowerCase() };
  });
})();

function GeneralTab({ workspaceId, editable }: {
  workspaceId: string;
  editable: boolean;
}) {
  const workspace = useWorkspaceStore((s) => s.currentWorkspace);
  const [name, setName] = useState(workspace?.name ?? '');
  const [description, setDescription] = useState(workspace?.description ?? '');
  const [timezone, setTimezone] = useState(workspace?.timezone ?? 'UTC');
  const [saving, setSaving] = useState(false);
  const [tzSearch, setTzSearch] = useState('');

  useEffect(() => {
    setName(workspace?.name ?? '');
    setDescription(workspace?.description ?? '');
    setTimezone(workspace?.timezone ?? 'UTC');
  }, [workspace?.id, workspace?.updated_at]);

  const selectedTz = useMemo(() => TIMEZONE_LIST.find((tz) => tz.id === timezone), [timezone]);

  const formatNow = useCallback(() =>
    new Intl.DateTimeFormat('en-US', {
      timeZone: timezone,
      month: 'short', day: 'numeric',
      hour: 'numeric', minute: '2-digit',
      hour12: true,
    }).format(new Date()),
    [timezone],
  );
  const [currentTime, setCurrentTime] = useState(formatNow);
  useEffect(() => {
    setCurrentTime(formatNow());
    const id = setInterval(() => setCurrentTime(formatNow()), 60_000);
    return () => clearInterval(id);
  }, [formatNow]);

  const filteredTimezones = useMemo(() => {
    if (!tzSearch) return TIMEZONE_LIST;
    const q = tzSearch.toLowerCase();
    return TIMEZONE_LIST.filter((tz) => tz.searchKey.includes(q));
  }, [tzSearch]);

  const handleSave = async () => {
    if (!name.trim()) {
      toast.error('Workspace name is required');
      return;
    }
    setSaving(true);
    const { data, error } = await workspacesService.update(workspaceId, {
      name: name.trim(),
      description: description.trim() || undefined,
      timezone,
    });
    setSaving(false);
    if (error) {
      toast.error(error);
    } else {
      toast.success('Workspace updated');
      if (data) {
        useWorkspaceStore.getState().setCurrentWorkspace(data);
      }
    }
  };

  return (
    <Card className={LINEAR_CARD_CLASS}>
      <CardHeader>
        <CardTitle>Workspace Settings</CardTitle>
        <CardDescription>Manage your workspace name, description, and timezone.</CardDescription>
      </CardHeader>
      <CardContent className="space-y-6">
        <div className="space-y-2">
          <Label htmlFor="ws-name">Workspace Name</Label>
          <Input
            id="ws-name"
            value={name}
            onChange={(e) => setName(e.target.value)}
            disabled={!editable}
            placeholder="My Workspace"
          />
        </div>

        <div className="space-y-2">
          <Label htmlFor="ws-desc">Description</Label>
          <Textarea
            id="ws-desc"
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            disabled={!editable}
            placeholder="A brief description of this workspace"
            rows={3}
          />
        </div>

        <div className="space-y-2">
          <Label htmlFor="ws-tz">Timezone</Label>
          <p className="text-xs text-muted-foreground">
            Used for sprint boundaries, due dates, and reporting. All members see the same deadlines.
          </p>
          <Popover>
            <PopoverTrigger asChild>
              <Button variant="outline" className="w-full justify-between font-normal" disabled={!editable}>
                <span className="flex items-center gap-2">
                  <Globe className="h-4 w-4 text-muted-foreground" />
                  {timezone}
                  {selectedTz && <span className="text-muted-foreground">({selectedTz.offset})</span>}
                </span>
                <ChevronRight className="h-4 w-4 text-muted-foreground" />
              </Button>
            </PopoverTrigger>
            <PopoverContent className="w-[320px] p-0" align="start">
              <div className="p-2 border-b">
                <div className="flex items-center gap-2 px-2">
                  <Search className="h-4 w-4 text-muted-foreground shrink-0" />
                  <input
                    className="flex-1 bg-transparent text-sm outline-none placeholder:text-muted-foreground"
                    placeholder="Search timezones..."
                    value={tzSearch}
                    onChange={(e) => setTzSearch(e.target.value)}
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
                        'w-full rounded-sm px-2 py-1.5 text-left text-sm hover:bg-accent flex items-center justify-between',
                        tz.id === timezone && 'bg-accent font-medium',
                      )}
                      onClick={() => { setTimezone(tz.id); setTzSearch(''); }}
                    >
                      <span>{tz.id}</span>
                      <span className="text-xs text-muted-foreground ml-2 shrink-0">{tz.offset}</span>
                    </button>
                  ))
                )}
              </div>
            </PopoverContent>
          </Popover>
          <p className="text-xs text-muted-foreground">
            Current date and time: <span className="font-medium text-foreground">{currentTime}</span>
          </p>
        </div>

        {editable && (
          <div className="flex justify-end">
            <Button onClick={handleSave} disabled={saving}>
              {saving ? 'Saving...' : 'Save'}
            </Button>
          </div>
        )}
      </CardContent>
    </Card>
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

  const { copy: copyToClipboard } = useCopyToClipboard();
  const handleCopyLink = (joinUrl: string) => {
    copyToClipboard(joinUrl);
    toast.success('Invite link copied to clipboard');
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

/* ============ Estimate Settings Form ============ */

function EstimateSettingsForm({ teamId, initial, saving, onSave }: {
  teamId: string;
  initial: TeamEstimateSettings | null;
  saving: boolean;
  onSave: (data: { enabled?: boolean; scale?: EstimateScale; extended?: boolean; allow_zero?: boolean; count_unestimated_as_one?: boolean }) => void;
}) {
  const [enabled, setEnabled] = useState(initial?.enabled ?? false);
  const [scale, setScale] = useState<EstimateScale>(initial?.scale ?? 'linear');
  const [extended, setExtended] = useState(initial?.extended ?? false);
  const [allowZero, setAllowZero] = useState(initial?.allow_zero ?? false);
  const [countUnestimated, setCountUnestimated] = useState(initial?.count_unestimated_as_one ?? true);

  useEffect(() => {
    setEnabled(initial?.enabled ?? false);
    setScale(initial?.scale ?? 'linear');
    setExtended(initial?.extended ?? false);
    setAllowZero(initial?.allow_zero ?? false);
    setCountUnestimated(initial?.count_unestimated_as_one ?? true);
  }, [initial, teamId]);

  const scaleOptions = getEstimateOptions(scale, extended, allowZero);

  return (
    <div className="space-y-5 py-2">
      {/* Enable toggle */}
      <div className="flex items-center justify-between">
        <div>
          <p className="text-sm font-medium">Enable estimates</p>
          <p className="text-xs text-muted-foreground">Show effort estimates on issues</p>
        </div>
        <Switch checked={enabled} onCheckedChange={setEnabled} />
      </div>

      {enabled && (
        <>
          {/* Scale selection */}
          <div className="space-y-2">
            <Label className="text-sm">Scale</Label>
            <Select value={scale} onValueChange={(v) => setScale(v as EstimateScale)}>
              <SelectTrigger className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {(Object.keys(SCALE_LABELS) as EstimateScale[]).map((s) => (
                  <SelectItem key={s} value={s}>
                    <span className="font-medium">{SCALE_LABELS[s]}</span>
                    <span className="ml-2 text-muted-foreground">{SCALE_DESCRIPTIONS[s]}</span>
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          {/* Preview */}
          <div className="rounded-md border border-border bg-muted/40 px-3 py-2">
            <p className="text-xs text-muted-foreground mb-1">Scale values</p>
            <div className="flex flex-wrap gap-1.5">
              {scaleOptions.map((opt) => (
                <span key={opt.value} className="inline-flex items-center rounded-md border border-border bg-background px-2 py-0.5 text-xs font-medium">
                  {opt.label}
                </span>
              ))}
            </div>
          </div>

          {/* Extended scale */}
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium">Extended scale</p>
              <p className="text-xs text-muted-foreground">Add two additional larger values</p>
            </div>
            <Switch checked={extended} onCheckedChange={setExtended} />
          </div>

          {/* Allow zero */}
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium">Allow zero estimates</p>
              <p className="text-xs text-muted-foreground">Allow issues to be estimated as zero effort</p>
            </div>
            <Switch checked={allowZero} onCheckedChange={setAllowZero} />
          </div>

          {/* Count unestimated as one */}
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium">Count unestimated as 1 point</p>
              <p className="text-xs text-muted-foreground">Unestimated issues count as 1 point in calculations</p>
            </div>
            <Switch checked={countUnestimated} onCheckedChange={setCountUnestimated} />
          </div>
        </>
      )}

      <DialogFooter>
        <Button
          disabled={saving}
          onClick={() => onSave({ enabled, scale, extended, allow_zero: allowZero, count_unestimated_as_one: countUnestimated })}
        >
          {saving ? 'Saving...' : 'Save'}
        </Button>
      </DialogFooter>
    </div>
  );
}

type VisibilityFieldKey = keyof Omit<TeamFieldVisibility, 'id' | 'team_id' | 'created_at' | 'updated_at'>;
type FieldVisibilityGroup = 'Classification' | 'Planning' | 'Other';

const FIELD_VISIBILITY_FIELDS: { key: VisibilityFieldKey; label: string; group: FieldVisibilityGroup }[] = [
  { key: 'priority', label: 'Priority', group: 'Classification' },
  { key: 'story_type', label: 'Type', group: 'Classification' },
  { key: 'severity', label: 'Severity', group: 'Classification' },
  { key: 'epic', label: 'Epic', group: 'Planning' },
  { key: 'sprint', label: 'Sprint', group: 'Planning' },
  { key: 'estimate', label: 'Estimate', group: 'Planning' },
  { key: 'labels', label: 'Labels', group: 'Other' },
  { key: 'due_date', label: 'Due Date', group: 'Other' },
  { key: 'blocked', label: 'Blocked', group: 'Other' },
];

function FieldVisibilityForm({ teamId, initial, saving, onSave }: {
  teamId: string;
  initial: TeamFieldVisibility | null;
  saving: boolean;
  onSave: (data: Partial<Omit<TeamFieldVisibility, 'id' | 'team_id' | 'created_at' | 'updated_at'>>) => void;
}) {
  const [fields, setFields] = useState<Record<VisibilityFieldKey, boolean>>(() => {
    const defaults = {} as Record<VisibilityFieldKey, boolean>;
    for (const f of FIELD_VISIBILITY_FIELDS) {
      defaults[f.key] = initial ? initial[f.key] : true;
    }
    return defaults;
  });

  useEffect(() => {
    const next = {} as Record<VisibilityFieldKey, boolean>;
    for (const f of FIELD_VISIBILITY_FIELDS) {
      next[f.key] = initial ? initial[f.key] : true;
    }
    setFields(next);
  }, [initial, teamId]);

  const groups = [...new Set(FIELD_VISIBILITY_FIELDS.map((f) => f.group))];

  return (
    <div className="space-y-5 py-2">
      <p className="text-sm text-muted-foreground">
        Toggle which metadata fields appear on stories for this team. State, Owner, Requester, and Team are always visible.
      </p>
      {groups.map((group) => (
        <div key={group} className="space-y-2">
          <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide">{group}</p>
          {FIELD_VISIBILITY_FIELDS.filter((f) => f.group === group).map((f) => (
            <div key={f.key} className="flex items-center justify-between">
              <p className="text-sm font-medium">{f.label}</p>
              <Switch
                checked={fields[f.key]}
                onCheckedChange={(checked) => setFields((prev) => ({ ...prev, [f.key]: checked }))}
              />
            </div>
          ))}
        </div>
      ))}
      <DialogFooter>
        <Button
          disabled={saving}
          onClick={() => onSave(fields)}
        >
          {saving ? 'Saving...' : 'Save'}
        </Button>
      </DialogFooter>
    </div>
  );
}

/* ============ Teams Tab ============ */

function TeamsTab({ workspaceId, teams, userMemberships, invitationPreassignments, teamEstimateSettings, teamFieldVisibility, editable, onRefresh }: {
  workspaceId: string;
  teams: WorkspaceTeam[];
  userMemberships: TeamUserMembership[];
  invitationPreassignments: InvitationTeamPreassignment[];
  teamEstimateSettings: TeamEstimateSettings[];
  teamFieldVisibility: TeamFieldVisibility[];
  editable: boolean;
  onRefresh: () => void | Promise<void>;
}) {
  const navigate = useNavigate();
  const { currentWorkspace } = useWorkspaceStore();
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editTeam, setEditTeam] = useState<WorkspaceTeam | null>(null);
  const [name, setName] = useState('');
  const [handle, setHandle] = useState('');
  const [description, setDescription] = useState('');
  const [saving, setSaving] = useState(false);
  const [query, setQuery] = useState('');
  const [selectedTeamId, setSelectedTeamId] = useState<string | null>(null);

  const [estimateDialogOpen, setEstimateDialogOpen] = useState(false);
  const [estimateSaving, setEstimateSaving] = useState(false);

  const [fieldVisDialogOpen, setFieldVisDialogOpen] = useState(false);
  const [fieldVisSaving, setFieldVisSaving] = useState(false);

  const [memberDialogOpen, setMemberDialogOpen] = useState(false);
  const [workspaceMembers, setWorkspaceMembers] = useState<MemberWithUser[]>([]);
  const [invitations, setInvitations] = useState<Invitation[]>([]);
  const [membersLoading, setMembersLoading] = useState(false);
  const [selectedUserId, setSelectedUserId] = useState('');
  const [selectedRole, setSelectedRole] = useState<'owner' | 'member'>('member');
  const [savingMember, setSavingMember] = useState(false);

  useEffect(() => {
    let mounted = true;
    const loadMembers = async () => {
      setMembersLoading(true);
      try {
        const [membersRes, invitesRes] = await Promise.all([
          workspacesService.listMembers(workspaceId),
          editable ? inviteService.list(workspaceId) : Promise.resolve({ data: null, error: null }),
        ]);
        if (!mounted) return;
        if (membersRes.error) {
          if (editable) toast.error(membersRes.error);
          setWorkspaceMembers([]);
        } else {
          setWorkspaceMembers(
            [...(membersRes.data ?? [])].sort((a, b) =>
              (a.full_name || a.email).localeCompare(b.full_name || b.email),
            ),
          );
        }
        setInvitations((invitesRes.data ?? []).filter((inv) => inv.status === 'pending'));
      } finally {
        if (mounted) setMembersLoading(false);
      }
    };
    loadMembers();
    return () => { mounted = false; };
  }, [workspaceId, editable]);

  const filteredTeams = [...teams]
    .filter((team) => {
      const search = query.trim().toLowerCase();
      if (!search) return true;
      return [team.name, team.handle, team.description]
        .filter(Boolean)
        .some((value) => value!.toLowerCase().includes(search));
    })
    .sort((a, b) => a.name.localeCompare(b.name));

  const selectedTeam = selectedTeamId ? teams.find((t) => t.id === selectedTeamId) ?? null : null;

  const getTeamMemberships = (teamId: string) =>
    userMemberships
      .filter((membership) => membership.team_id === teamId)
      .map((membership) => ({
        membership,
        user: workspaceMembers.find((member) => member.user_id === membership.user_id),
      }))
      .sort((a, b) => (a.user?.full_name || a.user?.email || '').localeCompare(b.user?.full_name || b.user?.email || ''));

  const teamMembers = selectedTeam
    ? getTeamMemberships(selectedTeam.id)
    : [];

  const availableMembers = selectedTeam
    ? workspaceMembers.filter((member) => !userMemberships.some((membership) => membership.team_id === selectedTeam.id && membership.user_id === member.user_id))
    : [];

  const openCreate = () => {
    setEditTeam(null);
    setName('');
    setHandle('');
    setDescription('');
    setDialogOpen(true);
  };

  const openEdit = (team: WorkspaceTeam) => {
    setEditTeam(team);
    setName(team.name);
    setHandle(team.handle ?? '');
    setDescription(team.description ?? '');
    setDialogOpen(true);
  };

  const openMembers = (team: WorkspaceTeam) => {
    setSelectedTeamId(team.id);
    setSelectedUserId('');
    setSelectedRole('member');
    setMemberDialogOpen(true);
  };

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setSaving(true);
    const payload = {
      name,
      handle: handle.trim() ? slugifyTeamHandle(handle) : undefined,
      description: description || undefined,
    };
    if (editTeam) {
      const { error } = await settingsService.updateTeam(editTeam.id, payload);
      if (error) toast.error(error);
      else {
        toast.success('Team updated');
        setDialogOpen(false);
        await onRefresh();
      }
    } else {
      const { data, error } = await settingsService.createTeam({ workspace_id: workspaceId, ...payload });
      if (error) toast.error(error);
      else {
        toast.success('Team created');
        setDialogOpen(false);
        setSelectedTeamId(data?.id ?? null);
        await onRefresh();
      }
    }
    setSaving(false);
  };

  const handleDelete = async (id: string) => {
    const { error } = await settingsService.deleteTeam(id);
    if (error) toast.error(error);
    else {
      toast.success('Team deleted');
      setSelectedTeamId(null);
      await onRefresh();
    }
  };

  const handleRoleChange = async (teamId: string, userId: string, role: 'owner' | 'member') => {
    const { error } = await settingsService.updateTeamMember(teamId, userId, { role });
    if (error) toast.error(error);
    else {
      toast.success('Team member updated');
      await onRefresh();
    }
  };

  const handleRemoveMember = async (teamId: string, userId: string) => {
    const { error } = await settingsService.removeTeamMember(teamId, userId);
    if (error) toast.error(error);
    else {
      toast.success('Team member removed');
      await onRefresh();
    }
  };

  const handleAddInvitation = async (teamId: string, invitationId: string) => {
    const { error } = await settingsService.addTeamInvitation(teamId, invitationId);
    if (error) toast.error(error);
    else {
      toast.success('Invited member pre-assigned to team');
      await onRefresh();
    }
  };

  const handleRemoveInvitation = async (teamId: string, invitationId: string) => {
    const { error } = await settingsService.removeTeamInvitation(teamId, invitationId);
    if (error) toast.error(error);
    else {
      toast.success('Invitation pre-assignment removed');
      await onRefresh();
    }
  };

  const openSettingsSection = (section: SettingsSection) => {
    const slug = currentWorkspace?.slug;
    if (!slug) return;
    navigate({
      to: '/w/$slug/settings/$section',
      params: { slug, section },
      search: { team: selectedTeamId ?? undefined },
    });
  };

  // ── Detail view (team selected) ──
  if (selectedTeam) {
    const invitedCount = invitationPreassignments.filter((pa) => pa.team_id === selectedTeam.id).length;
    const memberCount = teamMembers.length + invitedCount;
    const teamEstConfig = teamEstimateSettings.find((s) => s.team_id === selectedTeam.id);
    const estimateMeta = teamEstConfig?.enabled
      ? SCALE_LABELS[teamEstConfig.scale]
      : 'Disabled';
    const teamVisConfig = teamFieldVisibility.find((s) => s.team_id === selectedTeam.id);
    const visibleCount = teamVisConfig
      ? FIELD_VISIBILITY_FIELDS.filter((f) => teamVisConfig[f.key]).length
      : FIELD_VISIBILITY_FIELDS.length;
    const fieldVisMeta = `${visibleCount} of ${FIELD_VISIBILITY_FIELDS.length} visible`;
    const settingsGroups: {
      label?: string;
      rows: { key: string; icon: LucideIcon; title: string; description: string; meta: string; action: () => void; disabled: boolean }[];
    }[] = [
      {
        rows: [
          {
            key: 'general',
            icon: Settings2,
            title: 'General',
            description: 'Name, identifier, and broader settings',
            meta: selectedTeam.handle ? `@${selectedTeam.handle}` : '',
            action: () => openEdit(selectedTeam),
            disabled: !editable,
          },
          {
            key: 'members',
            icon: Users,
            title: 'Members',
            description: 'Manage team members',
            meta: `${memberCount} member${memberCount === 1 ? '' : 's'}`,
            action: () => openMembers(selectedTeam),
            disabled: !editable,
          },
        ],
      },
      {
        label: 'Issues, projects, and docs',
        rows: [
          {
            key: 'estimates',
            icon: LayoutGrid,
            title: 'Estimates',
            description: 'Configure estimate scale and options',
            meta: estimateMeta,
            action: () => setEstimateDialogOpen(true),
            disabled: !editable,
          },
          {
            key: 'field-visibility',
            icon: Eye,
            title: 'Field visibility',
            description: 'Show or hide metadata fields for this team',
            meta: fieldVisMeta,
            action: () => setFieldVisDialogOpen(true),
            disabled: !editable,
          },
          {
            key: 'labels',
            icon: Tag,
            title: 'Issue labels',
            description: "Labels available to this team's issues",
            meta: '',
            action: () => openSettingsSection('labels'),
            disabled: false,
          },
        ],
      },
      {
        label: 'Workflow',
        rows: [
          {
            key: 'workflow',
            icon: GitBranch,
            title: 'Issue statuses & automations',
            description: 'Customize issue statuses and automations',
            meta: '',
            action: () => openSettingsSection('workflows'),
            disabled: false,
          },
          {
            key: 'automations',
            icon: RefreshCw,
            title: 'Automations',
            description: 'Sprint and epic automations for this team',
            meta: '',
            action: () => openSettingsSection('automations'),
            disabled: false,
          },
        ],
      },
    ];

    return (
      <>
        <div className="space-y-8">
          <button
            type="button"
            onClick={() => setSelectedTeamId(null)}
            className="inline-flex items-center gap-1.5 text-sm text-muted-foreground transition-colors hover:text-foreground"
          >
            <ChevronRight className="h-4 w-4 rotate-180" />
            Teams
          </button>

          <div className="flex items-center gap-4">
            <div className="flex h-12 w-12 shrink-0 items-center justify-center rounded-xl border border-primary/20 bg-primary/10 text-base font-semibold text-primary">
              {getInitials(selectedTeam.name)}
            </div>
            <div className="flex min-w-0 flex-1 items-center justify-between">
              <h2 className="text-xl font-semibold tracking-tight">{selectedTeam.name}</h2>
              {editable && (
                <div className="flex items-center gap-2">
                  <Button variant="outline" size="sm" onClick={() => openMembers(selectedTeam)}>
                    <Users className="h-4 w-4 mr-1" />
                    Add member
                  </Button>
                  <Button variant="outline" size="sm" onClick={() => handleDelete(selectedTeam.id)}>
                    <Trash2 className="h-4 w-4 mr-1" />
                    Delete
                  </Button>
                </div>
              )}
            </div>
          </div>

          {settingsGroups.map((group) => (
            <div key={group.label ?? '_default'} className="space-y-3">
              {group.label && (
                <h3 className="text-sm font-medium text-muted-foreground">{group.label}</h3>
              )}
              <div className="overflow-hidden rounded-lg border border-border bg-background">
                {group.rows.map((row, idx) => (
                  <button
                    key={row.key}
                    type="button"
                    disabled={row.disabled}
                    onClick={row.action}
                    className={cn(
                      'flex w-full items-center gap-4 px-4 py-4 text-left transition-colors',
                      row.disabled ? 'cursor-not-allowed opacity-60' : 'hover:bg-muted/40',
                      idx > 0 && 'border-t border-border',
                    )}
                  >
                    <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground">
                      <row.icon className="h-4 w-4" />
                    </div>
                    <div className="min-w-0 flex-1">
                      <p className="text-sm font-medium">{row.title}</p>
                      <p className="text-sm text-muted-foreground">{row.description}</p>
                    </div>
                    {row.meta && (
                      <span className="hidden text-sm text-muted-foreground md:block">{row.meta}</span>
                    )}
                    <ChevronRight className="h-4 w-4 shrink-0 text-muted-foreground" />
                  </button>
                ))}
              </div>
            </div>
          ))}
        </div>

        <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
          <DialogContent>
            <form onSubmit={handleSubmit}>
              <DialogHeader>
                <DialogTitle>{editTeam ? 'Edit Team' : 'Create Team'}</DialogTitle>
              </DialogHeader>
              <div className="space-y-4 py-4">
                <div className="space-y-2">
                  <Label>Name</Label>
                  <Input value={name} onChange={e => setName(e.target.value)} required />
                </div>
                <div className="space-y-2">
                  <Label>Handle</Label>
                  <Input
                    value={handle}
                    onChange={e => setHandle(e.target.value)}
                    placeholder={slugifyTeamHandle(name) || 'growth'}
                  />
                  <p className="text-xs text-muted-foreground">Used for mentions like @{slugifyTeamHandle(handle || name) || 'team'}.</p>
                </div>
                <div className="space-y-2">
                  <Label>Description</Label>
                  <Textarea
                    value={description}
                    onChange={e => setDescription(e.target.value)}
                    rows={4}
                    placeholder="Describe what this team owns."
                  />
                </div>
              </div>
              <DialogFooter>
                <Button type="button" variant="outline" onClick={() => setDialogOpen(false)}>Cancel</Button>
                <Button type="submit" disabled={saving}>{saving ? 'Saving...' : 'Save'}</Button>
              </DialogFooter>
            </form>
          </DialogContent>
        </Dialog>

        <Dialog open={memberDialogOpen} onOpenChange={setMemberDialogOpen}>
          <DialogContent className="max-w-lg gap-0 p-0">
            <DialogHeader className="border-b px-5 py-4">
              <DialogTitle className="text-base">{selectedTeam ? `${selectedTeam.name} members` : 'Team Members'}</DialogTitle>
            </DialogHeader>

            {editable && (availableMembers.length > 0 || (selectedTeam && invitations.filter((inv) => !invitationPreassignments.some((pa) => pa.invitation_id === inv.id && pa.team_id === selectedTeam.id)).length > 0)) && (
              <div className="flex items-center gap-2 border-b px-5 py-3">
                <Select value={selectedUserId} onValueChange={setSelectedUserId}>
                  <SelectTrigger className="h-9 flex-1">
                    <SelectValue placeholder={membersLoading ? 'Loading...' : 'Add a member...'} />
                  </SelectTrigger>
                  <SelectContent>
                    {availableMembers.length > 0 && availableMembers.map((member) => (
                      <SelectItem key={member.user_id} value={`user:${member.user_id}`}>
                        <div className="flex items-center gap-2">
                          <UserAvatar name={member.full_name || member.email} className="h-5 w-5" fallbackClassName="text-[9px]" />
                          {member.full_name || member.email}
                        </div>
                      </SelectItem>
                    ))}
                    {selectedTeam && invitations.filter((inv) => !invitationPreassignments.some((pa) => pa.invitation_id === inv.id && pa.team_id === selectedTeam.id)).length > 0 && (
                      <>
                        {availableMembers.length > 0 && (
                          <div className="px-2 py-1.5 text-xs font-medium text-muted-foreground">Pending invitations</div>
                        )}
                        {invitations
                          .filter((inv) => !invitationPreassignments.some((pa) => pa.invitation_id === inv.id && pa.team_id === selectedTeam.id))
                          .map((inv) => (
                            <SelectItem key={inv.id} value={`inv:${inv.id}`}>
                              <div className="flex items-center gap-2">
                                <UserAvatar name={inv.email} className="h-5 w-5" fallbackClassName="text-[9px]" />
                                <span>{inv.email}</span>
                                <Badge variant="secondary" className="text-[10px] px-1.5 py-0">Invited</Badge>
                              </div>
                            </SelectItem>
                          ))}
                      </>
                    )}
                  </SelectContent>
                </Select>
                {selectedUserId.startsWith('user:') && (
                  <Select value={selectedRole} onValueChange={(value) => setSelectedRole(value as 'owner' | 'member')}>
                    <SelectTrigger className="h-9 w-[110px]">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="member">Member</SelectItem>
                      <SelectItem value="owner">Owner</SelectItem>
                    </SelectContent>
                  </Select>
                )}
                <Button
                  size="sm"
                  className="h-9"
                  disabled={!selectedUserId || savingMember}
                  onClick={async () => {
                    if (!selectedTeam) return;
                    if (selectedUserId.startsWith('user:')) {
                      const uid = selectedUserId.replace('user:', '');
                      setSavingMember(true);
                      const { error } = await settingsService.addTeamMember(selectedTeam.id, { user_id: uid, role: selectedRole });
                      setSavingMember(false);
                      if (error) { toast.error(error); return; }
                      toast.success('Team member added');
                      setSelectedUserId('');
                      setSelectedRole('member');
                      await onRefresh();
                    } else if (selectedUserId.startsWith('inv:')) {
                      const invId = selectedUserId.replace('inv:', '');
                      await handleAddInvitation(selectedTeam.id, invId);
                      setSelectedUserId('');
                    }
                  }}
                >
                  Add
                </Button>
              </div>
            )}

            <div className="max-h-[400px] overflow-y-auto">
              {selectedTeam && (teamMembers.length > 0 || invitationPreassignments.filter((pa) => pa.team_id === selectedTeam.id).length > 0) ? (
                <div className="divide-y">
                  {teamMembers.map(({ membership, user }) => (
                    <div key={membership.id} className="flex items-center gap-3 px-5 py-3">
                      <UserAvatar name={user?.full_name || user?.email || membership.user_id} className="h-8 w-8" fallbackClassName="text-xs" />
                      <div className="min-w-0 flex-1">
                        <p className="truncate text-sm font-medium">{user?.full_name || 'Unknown user'}</p>
                        <p className="truncate text-xs text-muted-foreground">{user?.email || membership.user_id}</p>
                      </div>
                      {editable ? (
                        <div className="flex items-center gap-1.5">
                          <Select
                            value={membership.role}
                            onValueChange={(value) => handleRoleChange(membership.team_id, membership.user_id, value as 'owner' | 'member')}
                          >
                            <SelectTrigger className="h-7 w-[100px] text-xs">
                              <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                              <SelectItem value="member">Member</SelectItem>
                              <SelectItem value="owner">Owner</SelectItem>
                            </SelectContent>
                          </Select>
                          <Button size="icon" variant="ghost" className="h-7 w-7" onClick={() => handleRemoveMember(membership.team_id, membership.user_id)}>
                            <X className="h-3.5 w-3.5" />
                          </Button>
                        </div>
                      ) : (
                        <Badge variant="outline" className="text-xs capitalize">{membership.role}</Badge>
                      )}
                    </div>
                  ))}
                  {invitationPreassignments
                    .filter((pa) => pa.team_id === selectedTeam.id)
                    .map((pa) => {
                      const inv = invitations.find((i) => i.id === pa.invitation_id);
                      return (
                        <div key={pa.id} className="flex items-center gap-3 px-5 py-3 bg-muted/30">
                          <UserAvatar name={inv?.email || pa.invitation_id} className="h-8 w-8" fallbackClassName="text-xs" />
                          <div className="min-w-0 flex-1">
                            <p className="truncate text-sm font-medium">{inv?.email || 'Pending invitation'}</p>
                            <p className="truncate text-xs text-muted-foreground">Will join as member when accepted</p>
                          </div>
                          <Badge variant="secondary" className="text-xs">Invited</Badge>
                          {editable && (
                            <Button size="icon" variant="ghost" className="h-7 w-7" onClick={() => handleRemoveInvitation(selectedTeam.id, pa.invitation_id)}>
                              <X className="h-3.5 w-3.5" />
                            </Button>
                          )}
                        </div>
                      );
                    })}
                </div>
              ) : (
                <div className="px-5 py-8 text-center text-sm text-muted-foreground">
                  No team members yet. Add members above.
                </div>
              )}
            </div>
          </DialogContent>
        </Dialog>

        {/* Estimate Settings Dialog */}
        <Dialog open={estimateDialogOpen} onOpenChange={setEstimateDialogOpen}>
          <DialogContent className="max-w-md">
            <DialogHeader>
              <DialogTitle>Estimate Settings</DialogTitle>
            </DialogHeader>
            <EstimateSettingsForm
              teamId={selectedTeam.id}
              initial={teamEstConfig ?? null}
              saving={estimateSaving}
              onSave={async (data) => {
                setEstimateSaving(true);
                const { error } = await settingsService.updateTeamEstimateSettings(selectedTeam.id, data);
                setEstimateSaving(false);
                if (error) {
                  toast.error(error);
                } else {
                  toast.success('Estimate settings updated');
                  setEstimateDialogOpen(false);
                  await onRefresh();
                }
              }}
            />
          </DialogContent>
        </Dialog>

        {/* Field Visibility Dialog */}
        <Dialog open={fieldVisDialogOpen} onOpenChange={setFieldVisDialogOpen}>
          <DialogContent className="max-w-md">
            <DialogHeader>
              <DialogTitle>Field Visibility</DialogTitle>
            </DialogHeader>
            <FieldVisibilityForm
              teamId={selectedTeam.id}
              initial={teamVisConfig ?? null}
              saving={fieldVisSaving}
              onSave={async (data) => {
                setFieldVisSaving(true);
                const { error } = await settingsService.updateTeamFieldVisibility(selectedTeam.id, data);
                setFieldVisSaving(false);
                if (error) {
                  toast.error(error);
                } else {
                  toast.success('Field visibility updated');
                  setFieldVisDialogOpen(false);
                  await onRefresh();
                }
              }}
            />
          </DialogContent>
        </Dialog>
      </>
    );
  }

  // ── List view (no team selected) ──
  return (
    <>
      <div className="space-y-6">
        <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
          <div>
            <h2 className="text-xl font-semibold">Teams</h2>
            <p className="text-sm text-muted-foreground">
              Create teams, assign members, and manage team-level PM settings.
            </p>
          </div>
          <div className="flex items-center gap-3">
            {teams.length > 0 && (
              <div className="relative">
                <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  value={query}
                  onChange={(event) => setQuery(event.target.value)}
                  className="h-9 w-60 pl-9"
                  placeholder="Search teams..."
                />
              </div>
            )}
            {editable && (
              <Button size="sm" onClick={openCreate}>
                <Plus className="h-4 w-4 mr-1" />
                New Team
              </Button>
            )}
          </div>
        </div>

        {teams.length === 0 ? (
          <div className="flex min-h-[320px] flex-col items-center justify-center gap-3 rounded-lg border border-dashed border-border text-center">
            <div className="flex h-12 w-12 items-center justify-center rounded-xl border border-border bg-muted/60">
              <Users className="h-5 w-5 text-muted-foreground" />
            </div>
            <div className="space-y-1">
              <p className="font-medium">No teams yet</p>
              <p className="text-sm text-muted-foreground">
                Create your first team to manage memberships, labels, and workflows.
              </p>
            </div>
            {editable && (
              <Button onClick={openCreate}>
                <Plus className="h-4 w-4 mr-1" />
                Create Team
              </Button>
            )}
          </div>
        ) : (
          <div className="overflow-hidden rounded-lg border border-border">
            <Table>
              <TableHeader>
                <TableRow className="bg-muted/40 hover:bg-muted/40">
                  <TableHead className="w-[280px]">Name</TableHead>
                  <TableHead className="w-[140px]">Handle</TableHead>
                  <TableHead className="w-[100px]">Members</TableHead>
                  <TableHead>Description</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {filteredTeams.map((team) => {
                  const memberCount = getTeamMemberships(team.id).length;
                  return (
                    <TableRow
                      key={team.id}
                      className="cursor-pointer"
                      onClick={() => setSelectedTeamId(team.id)}
                    >
                      <TableCell>
                        <div className="flex items-center gap-3">
                          <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg border border-primary/20 bg-primary/10 text-xs font-semibold text-primary">
                            {getInitials(team.name)}
                          </div>
                          <span className="font-medium">{team.name}</span>
                        </div>
                      </TableCell>
                      <TableCell>
                        {team.handle ? (
                          <span className="font-mono text-xs text-muted-foreground">@{team.handle}</span>
                        ) : (
                          <span className="text-xs text-muted-foreground/50">&mdash;</span>
                        )}
                      </TableCell>
                      <TableCell>
                        <span className="text-sm text-muted-foreground">{memberCount}</span>
                      </TableCell>
                      <TableCell>
                        <span className="line-clamp-1 text-sm text-muted-foreground">
                          {team.description || '\u2014'}
                        </span>
                      </TableCell>
                    </TableRow>
                  );
                })}
                {filteredTeams.length === 0 && (
                  <TableRow>
                    <TableCell colSpan={4} className="py-8 text-center text-sm text-muted-foreground">
                      No teams match your search.
                    </TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          </div>
        )}
      </div>

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent>
          <form onSubmit={handleSubmit}>
            <DialogHeader>
              <DialogTitle>Create Team</DialogTitle>
            </DialogHeader>
            <div className="space-y-4 py-4">
              <div className="space-y-2">
                <Label>Name</Label>
                <Input value={name} onChange={e => setName(e.target.value)} required />
              </div>
              <div className="space-y-2">
                <Label>Handle</Label>
                <Input
                  value={handle}
                  onChange={e => setHandle(e.target.value)}
                  placeholder={slugifyTeamHandle(name) || 'growth'}
                />
                <p className="text-xs text-muted-foreground">Used for mentions like @{slugifyTeamHandle(handle || name) || 'team'}.</p>
              </div>
              <div className="space-y-2">
                <Label>Description</Label>
                <Textarea
                  value={description}
                  onChange={e => setDescription(e.target.value)}
                  rows={4}
                  placeholder="Describe what this team owns."
                />
              </div>
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setDialogOpen(false)}>Cancel</Button>
              <Button type="submit" disabled={saving}>{saving ? 'Saving...' : 'Save'}</Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </>
  );
}

/* ============ People Tab ============ */

function PeopleTab({ people, editable, onRefresh }: {
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

function WorkflowsTab({ workspaceId, teams, editable, initialTeamId }: {
  workspaceId: string;
  teams: WorkspaceTeam[];
  editable: boolean;
  initialTeamId?: string;
}) {
  const navigate = useNavigate();
  const { currentWorkspace } = useWorkspaceStore();
  const [workflows, setWorkflows] = useState<WorkflowWithStates[]>([]);
  const [loading, setLoading] = useState(true);
  const [teamFilter, setTeamFilter] = useState<string>(initialTeamId || '__all__');
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

  const filteredWorkflows = teamFilter === '__all__'
    ? workflows
    : teamFilter === '__default__'
      ? workflows.filter((w) => !w.workflow.team_id)
      : workflows.filter((w) => w.workflow.team_id === teamFilter);

  return (
    <Card className={LINEAR_CARD_CLASS}>
      <CardHeader>
        <div className="flex items-center justify-between">
          <div>
            <CardTitle className="text-base">Workflows</CardTitle>
            <CardDescription>{filteredWorkflows.length} workflow{filteredWorkflows.length !== 1 ? 's' : ''}</CardDescription>
          </div>
          <div className="flex items-center gap-2">
            <Select value={teamFilter} onValueChange={setTeamFilter}>
              <SelectTrigger className="h-8 w-[220px] text-sm">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="__all__">All workflows</SelectItem>
                <SelectItem value="__default__">Workspace default</SelectItem>
                {teams.map((team) => (
                  <SelectItem key={team.id} value={team.id}>
                    {team.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            {editable && (
              <Button size="sm" onClick={openCreate}>
                <Plus className="h-4 w-4 mr-1" /> Create Workflow
              </Button>
            )}
          </div>
        </div>
      </CardHeader>
      <CardContent>
        {loading ? (
          <p className="text-sm text-muted-foreground text-center py-6">Loading workflows...</p>
        ) : filteredWorkflows.length === 0 ? (
          <p className="text-sm text-muted-foreground text-center py-6">No workflows found.</p>
        ) : (
          <div className="space-y-2">
            {filteredWorkflows.map((workflow) => (
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
                    <Button
                      size="sm"
                      variant="ghost"
                      className="h-7 gap-1 text-xs"
                      onClick={() => {
                        const slug = currentWorkspace?.slug;
                        if (slug) {
                          navigate({
                            to: '/w/$slug/settings/$section',
                            params: { slug, section: 'workflowstates' },
                            search: { workflow: workflow.workflow.id },
                          });
                        }
                      }}
                    >
                      <ListTree className="h-3.5 w-3.5" />
                      Modify States
                    </Button>
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

function WorkflowStatesTab({ workspaceId, editable, initialWorkflowId }: {
  workspaceId: string;
  editable: boolean;
  initialWorkflowId?: string;
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
      // Prefer URL param, then keep existing selection, then fall back to first
      if (initialWorkflowId && next.some((workflow) => workflow.workflow.id === initialWorkflowId)) return initialWorkflowId;
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

/* ============ Automations Tab ============ */

function AutomationsTab({ workspaceId, teams }: {
  workspaceId: string;
  teams: WorkspaceTeam[];
}) {
  const [automations, setAutomations] = useState<PMAutomation[]>([]);
  const [epicStates, setEpicStates] = useState<EpicWorkflowState[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      setLoading(true);
      try {
        const [autoRes, statesRes] = await Promise.all([
          pmAutomationService.list(workspaceId),
          pmWorkflowService.listEpicStates(workspaceId),
        ]);
        if (cancelled) return;
        if (autoRes.data) setAutomations(autoRes.data);
        if (statesRes.data) setEpicStates(statesRes.data);
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => { cancelled = true; };
  }, [workspaceId]);

  const getAuto = (type: AutomationType, teamId?: string): PMAutomation | undefined =>
    automations.find((a) => a.automation_type === type && (teamId ? a.team_id === teamId : !a.team_id));

  const upsert = async (type: AutomationType, enabled: boolean, opts?: { teamId?: string; configStateId?: string; configInt?: number; configInt2?: number; configInt3?: number }) => {
    const payload = {
      workspace_id: workspaceId,
      automation_type: type,
      enabled,
      team_id: opts?.teamId,
      config_state_id: opts?.configStateId,
      config_int: opts?.configInt,
      config_int2: opts?.configInt2,
      config_int3: opts?.configInt3,
    };

    // Snapshot for rollback
    const snapshot = automations;

    // Optimistic update
    setAutomations((prev) => {
      const idx = prev.findIndex((a) => a.automation_type === type && (opts?.teamId ? a.team_id === opts.teamId : !a.team_id));
      if (idx >= 0) {
        const updated = [...prev];
        updated[idx] = { ...updated[idx], enabled, config_state_id: opts?.configStateId, config_int: opts?.configInt, config_int2: opts?.configInt2, config_int3: opts?.configInt3 };
        return updated;
      }
      return [...prev, { id: 'temp-' + Date.now(), workspace_id: workspaceId, automation_type: type, enabled, team_id: opts?.teamId, config_state_id: opts?.configStateId, config_int: opts?.configInt, config_int2: opts?.configInt2, config_int3: opts?.configInt3, created_at: '', updated_at: '' }];
    });

    try {
      const res = await pmAutomationService.upsert(workspaceId, payload);
      if (res.error) {
        setAutomations(snapshot);
        toast.error(res.error);
      } else if (res.data) {
        // Replace temp/stale entry with server response
        setAutomations((prev) => {
          const idx = prev.findIndex((a) => a.automation_type === type && (opts?.teamId ? a.team_id === opts.teamId : !a.team_id));
          if (idx >= 0) {
            const updated = [...prev];
            updated[idx] = res.data!;
            return updated;
          }
          return prev;
        });
      }
    } catch {
      setAutomations(snapshot);
      toast.error('Failed to save automation');
    }
  };

  const removeAuto = async (type: AutomationType, teamId?: string) => {
    const snapshot = automations;
    setAutomations((prev) => prev.filter((a) => !(a.automation_type === type && (teamId ? a.team_id === teamId : !a.team_id))));
    const res = await pmAutomationService.remove(workspaceId, type, teamId);
    if (res.error) {
      setAutomations(snapshot);
      toast.error(res.error);
    }
  };

  const updateSprintConfig = (cfg: PMAutomation, patch: { enabled?: boolean; configInt?: number; configInt2?: number; configInt3?: number }) =>
    upsert('sprint_auto_create', patch.enabled ?? cfg.enabled, {
      teamId: cfg.team_id!,
      configInt: patch.configInt ?? cfg.config_int ?? 2,
      configInt2: patch.configInt2 ?? cfg.config_int2 ?? 1,
      configInt3: patch.configInt3 ?? cfg.config_int3 ?? 1,
    });

  const startedStates = epicStates.filter((s) => s.state_type === 'started');
  const doneStates = epicStates.filter((s) => s.state_type === 'done');

  const autoStart = getAuto('epic_auto_start');
  const autoComplete = getAuto('epic_auto_complete');

  const sprintAutoCreateConfigs = automations.filter((a) => a.automation_type === 'sprint_auto_create' && a.team_id);
  const sprintMoveConfigs = automations.filter((a) => a.automation_type === 'sprint_move_unfinished' && a.team_id);

  const sprintAutoCreateTeamIds = new Set(sprintAutoCreateConfigs.map((a) => a.team_id!));
  const sprintMoveTeamIds = new Set(sprintMoveConfigs.map((a) => a.team_id!));

  if (loading) {
    return (
      <div className="space-y-4">
        <Skeleton className="h-48" />
        <Skeleton className="h-48" />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* ── Epic Automations ── */}
      <Card className={LINEAR_CARD_CLASS}>
        <CardHeader>
          <CardTitle className="text-base">Epic Automations</CardTitle>
          <CardDescription>Automatically transition epics based on story progress.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex items-center gap-2 rounded-md bg-blue-50 p-3 text-sm text-blue-800">
            <Info className="h-4 w-4 shrink-0" />
            Changes to Epic Automations affect the entire workspace.
          </div>

          {/* Auto Start Epic */}
          <div className="flex items-center justify-between gap-4 rounded-md border p-4">
            <div className="flex-1">
              <p className="text-sm font-medium">Auto Start Epic</p>
              <p className="text-xs text-muted-foreground">
                When any story moves to a started state, auto-transition its parent epic.
              </p>
            </div>
            <div className="flex items-center gap-3">
              {startedStates.length > 1 ? (
                <Select
                  value={autoStart?.config_state_id ?? ''}
                  onValueChange={(val) => upsert('epic_auto_start', autoStart?.enabled ?? true, { configStateId: val })}
                >
                  <SelectTrigger className="w-[180px] h-8 text-xs">
                    <SelectValue placeholder="Target state..." />
                  </SelectTrigger>
                  <SelectContent>
                    {startedStates.map((st) => (
                      <SelectItem key={st.id} value={st.id}>{st.name}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              ) : startedStates.length === 1 ? (
                <Badge variant="secondary" className="text-xs">{startedStates[0].name}</Badge>
              ) : null}
              <Switch
                checked={autoStart?.enabled ?? false}
                onCheckedChange={(checked) => {
                  const stateId = autoStart?.config_state_id ?? startedStates[0]?.id;
                  if (!stateId) { toast.error('No started epic state available. Please check your epic workflow states.'); return; }
                  upsert('epic_auto_start', checked, { configStateId: stateId });
                }}
              />
            </div>
          </div>

          {/* Auto Complete Epic */}
          <div className="flex items-center justify-between gap-4 rounded-md border p-4">
            <div className="flex-1">
              <p className="text-sm font-medium">Auto Complete Epic</p>
              <p className="text-xs text-muted-foreground">
                When all stories in an epic reach a done state, auto-transition the epic.
              </p>
            </div>
            <div className="flex items-center gap-3">
              {doneStates.length > 1 ? (
                <Select
                  value={autoComplete?.config_state_id ?? ''}
                  onValueChange={(val) => upsert('epic_auto_complete', autoComplete?.enabled ?? true, { configStateId: val })}
                >
                  <SelectTrigger className="w-[180px] h-8 text-xs">
                    <SelectValue placeholder="Target state..." />
                  </SelectTrigger>
                  <SelectContent>
                    {doneStates.map((st) => (
                      <SelectItem key={st.id} value={st.id}>{st.name}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              ) : doneStates.length === 1 ? (
                <Badge variant="secondary" className="text-xs">{doneStates[0].name}</Badge>
              ) : null}
              <Switch
                checked={autoComplete?.enabled ?? false}
                onCheckedChange={(checked) => {
                  const stateId = autoComplete?.config_state_id ?? doneStates[0]?.id;
                  if (!stateId) { toast.error('No done epic state available. Please check your epic workflow states.'); return; }
                  upsert('epic_auto_complete', checked, { configStateId: stateId });
                }}
              />
            </div>
          </div>
        </CardContent>
      </Card>

      {/* ── Sprint Automations ── */}
      <Card className={LINEAR_CARD_CLASS}>
        <CardHeader>
          <CardTitle className="text-base">Sprint Automations</CardTitle>
          <CardDescription>Automate sprint creation and story rollover per team.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-6">
          <div className="flex items-center gap-2 rounded-md bg-blue-50 p-3 text-sm text-blue-800">
            <Info className="h-4 w-4 shrink-0" />
            Changes to Sprint Automations are specific to each Team.
          </div>

          {/* Auto-Create Future Sprints */}
          <div className="space-y-3">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm font-medium">Auto-Create Future Sprints</p>
                <p className="text-xs text-muted-foreground">
                  Automatically create future sprints when a sprint completes.
                </p>
              </div>
              <Select
                value=""
                onValueChange={(teamId) => upsert('sprint_auto_create', true, { teamId, configInt: 2, configInt2: 1, configInt3: 1 })}
              >
                <SelectTrigger className="w-[140px] h-8 text-xs">
                  <SelectValue placeholder="Add Team..." />
                </SelectTrigger>
                <SelectContent>
                  {teams.filter((t) => !sprintAutoCreateTeamIds.has(t.id)).map((t) => (
                    <SelectItem key={t.id} value={t.id}>{t.name}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            {sprintAutoCreateConfigs.map((cfg) => {
              const team = teams.find((t) => t.id === cfg.team_id);
              return (
                <div key={cfg.id} className="flex items-center gap-3 rounded-md border p-3">
                  <span className="text-sm font-medium min-w-[100px]">{team?.name ?? 'Unknown'}</span>
                  <div className="flex items-center gap-2 text-xs flex-wrap">
                    <Label className="text-xs text-muted-foreground">Sprints:</Label>
                    <Input
                      type="number"
                      min={1}
                      max={10}
                      value={cfg.config_int ?? 2}
                      onChange={(e) => updateSprintConfig(cfg, { configInt: Number(e.target.value) })}
                      className="w-16 h-7 text-xs"
                    />
                    <Label className="text-xs text-muted-foreground">Weeks:</Label>
                    <Input
                      type="number"
                      min={1}
                      max={8}
                      value={cfg.config_int2 ?? 1}
                      onChange={(e) => updateSprintConfig(cfg, { configInt2: Number(e.target.value) })}
                      className="w-16 h-7 text-xs"
                    />
                    <Label className="text-xs text-muted-foreground">Start day:</Label>
                    <Select
                      value={String(cfg.config_int3 ?? 1)}
                      onValueChange={(val) => updateSprintConfig(cfg, { configInt3: Number(val) })}
                    >
                      <SelectTrigger className="w-[100px] h-7 text-xs">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        {['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday'].map((day, i) => (
                          <SelectItem key={i} value={String(i)}>{day}</SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                  <Switch
                    checked={cfg.enabled}
                    onCheckedChange={(checked) => updateSprintConfig(cfg, { enabled: checked })}
                  />
                  <button type="button" onClick={() => removeAuto('sprint_auto_create', cfg.team_id!)} className="text-muted-foreground hover:text-destructive">
                    <X className="h-4 w-4" />
                  </button>
                </div>
              );
            })}
          </div>

          {/* Move Unfinished Stories */}
          <div className="space-y-3">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm font-medium">Move Unfinished Stories to Next Sprint</p>
                <p className="text-xs text-muted-foreground">
                  When a sprint ends, move incomplete stories to the next sprint.
                </p>
              </div>
              <Select
                value=""
                onValueChange={(teamId) => upsert('sprint_move_unfinished', true, { teamId })}
              >
                <SelectTrigger className="w-[140px] h-8 text-xs">
                  <SelectValue placeholder="Add Team..." />
                </SelectTrigger>
                <SelectContent>
                  {teams.filter((t) => !sprintMoveTeamIds.has(t.id)).map((t) => (
                    <SelectItem key={t.id} value={t.id}>{t.name}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            {sprintMoveConfigs.map((cfg) => {
              const team = teams.find((t) => t.id === cfg.team_id);
              return (
                <div key={cfg.id} className="flex items-center gap-3 rounded-md border p-3">
                  <span className="text-sm font-medium flex-1">{team?.name ?? 'Unknown'}</span>
                  <Switch
                    checked={cfg.enabled}
                    onCheckedChange={(checked) => upsert('sprint_move_unfinished', checked, { teamId: cfg.team_id! })}
                  />
                  <button type="button" onClick={() => removeAuto('sprint_move_unfinished', cfg.team_id!)} className="text-muted-foreground hover:text-destructive">
                    <X className="h-4 w-4" />
                  </button>
                </div>
              );
            })}
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
