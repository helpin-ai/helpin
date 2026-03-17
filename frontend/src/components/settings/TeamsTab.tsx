import { useEffect, useState, type FormEvent } from 'react';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { settingsService } from '@/lib/services/settingsService';
import { gitService } from '@/lib/services/gitService';
import { workspacesService } from '@/lib/services/workspacesService';
import { inviteService } from '@/lib/services/inviteService';
import { pmWorkflowService } from '@/lib/services/pmWorkflowService';
import { automationRuleService } from '@/lib/services/automationRuleService';
import { agentService } from '@/lib/services/agentService';
import { PipelineBuilder } from './PipelineBuilder';
import { SCALE_LABELS, SCALE_DESCRIPTIONS, getEstimateOptions } from '@/lib/estimateScales';
import type { WorkspaceTeam, MemberWithUser, Invitation, TeamUserMembership, InvitationTeamPreassignment, TeamEstimateSettings, TeamFieldVisibility, EstimateScale, TeamRepoDefault } from '@/lib/types';
import type { Agent, AutomationRule, GitRepository, StateType, WorkflowState, WorkflowWithStates } from '@/lib/pmTypes';
import { ColorPicker, PRESET_COLORS } from '@/components/pm/ColorPicker';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import type { SettingsSection } from '@/pages/Settings';
import { UserAvatar, getAvatarColor } from '@/components/pm/UserAvatar';
import { ConfirmDialog } from '@/components/pm/ConfirmDialog';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { Textarea } from '@/components/ui/textarea';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { cn, getInitials } from '@/lib/utils';
import { Checkbox } from '@/components/ui/checkbox';
import { Bot, Check, ChevronRight, Eye, GitBranch, GitPullRequest, LayoutGrid, Pencil, Plus, RefreshCw, Settings2, Tag, Trash2, Users, X, type LucideIcon } from 'lucide-react';
import { useNavigate } from '@tanstack/react-router';
import { toast } from 'sonner';
import {
  buildPresetFieldVisibility,
  normalizeTeamType,
  slugifyTeamHandle,
  TEAM_TYPE_PRESETS,
  type DefaultStoryType,
  type TeamType,
  type VisibilityFieldKey,
} from '@/lib/teamPresets';

const STORY_TYPE_LABELS: Record<DefaultStoryType, string> = {
  feature: 'Feature',
  bug: 'Bug',
  chore: 'Chore',
};

/* ── Helper Forms ── */

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
          <p className="text-xs text-muted-foreground">Show effort estimates on stories</p>
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
              <p className="text-xs text-muted-foreground">Allow stories to be estimated as zero effort</p>
            </div>
            <Switch checked={allowZero} onCheckedChange={setAllowZero} />
          </div>

          {/* Count unestimated as one */}
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium">Count unestimated as 1 point</p>
              <p className="text-xs text-muted-foreground">Unestimated stories count as 1 point in calculations</p>
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

type FieldVisibilityGroup = 'Classification' | 'Planning' | 'Other' | 'Panels';

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
  { key: 'delivery', label: 'Delivery', group: 'Panels' },
  { key: 'dev_history', label: 'Development History', group: 'Panels' },
];

const FIELD_VISIBILITY_HELP: Record<VisibilityFieldKey, string> = {
  priority: 'Shows the urgency level for a story so the team can quickly sort what matters most.',
  story_type: 'Shows whether the story is a feature, bug, or chore.',
  severity: 'Shows impact level, usually for bugs or operational issues. This starts off for new teams by default.',
  epic: 'Lets stories roll up into larger initiatives.',
  sprint: 'Lets stories be assigned to sprint cycles.',
  estimate: 'Shows effort sizing on stories for planning and forecasting.',
  labels: 'Adds lightweight tags for categorization and filtering.',
  due_date: 'Shows target due dates directly on stories.',
  blocked: 'Lets the team mark a story as blocked when it cannot move forward.',
  delivery: 'Shows delivery-related metadata such as repo and branch context.',
  dev_history: 'Shows linked pull requests, commits, and related development activity.',
};

function FieldVisibilityForm({ teamId, initial, saving, onSave }: {
  teamId: string;
  initial: TeamFieldVisibility | null;
  saving: boolean;
  onSave: (data: Partial<Omit<TeamFieldVisibility, 'id' | 'team_id' | 'created_at' | 'updated_at'>>) => void;
}) {
  const [fields, setFields] = useState<Record<VisibilityFieldKey, boolean>>(() => {
    const defaults = {} as Record<VisibilityFieldKey, boolean>;
    for (const f of FIELD_VISIBILITY_FIELDS) {
      defaults[f.key] = initial ? initial[f.key] : f.key === 'severity' || f.key === 'blocked' ? false : true;
    }
    return defaults;
  });

  useEffect(() => {
    const next = {} as Record<VisibilityFieldKey, boolean>;
    for (const f of FIELD_VISIBILITY_FIELDS) {
      next[f.key] = initial ? initial[f.key] : f.key === 'severity' || f.key === 'blocked' ? false : true;
    }
    setFields(next);
  }, [initial, teamId]);

  const groups = [...new Set(FIELD_VISIBILITY_FIELDS.map((f) => f.group))];

  return (
    <div className="space-y-5 py-2">
      <p className="text-sm text-muted-foreground">
        Configure which fields and panels appear on stories for this team. State, Owner, Requester, and Team are always visible.
      </p>
      {groups.map((group) => (
        <div key={group} className="space-y-2">
          <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide">{group}</p>
          {FIELD_VISIBILITY_FIELDS.filter((f) => f.group === group).map((f) => (
            <div key={f.key} className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <p className="text-sm font-medium">{f.label}</p>
                <Tooltip>
                  <TooltipTrigger asChild>
                    <button
                      type="button"
                      className="inline-flex h-4 w-4 items-center justify-center rounded-full border border-border/70 text-[10px] font-semibold text-muted-foreground transition-colors hover:border-border hover:text-foreground"
                      aria-label={`Help for ${f.label}`}
                    >
                      ?
                    </button>
                  </TooltipTrigger>
                  <TooltipContent side="top" className="w-56 text-pretty leading-relaxed">
                    {FIELD_VISIBILITY_HELP[f.key]}
                  </TooltipContent>
                </Tooltip>
              </div>
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

function TeamRepoDefaultForm({
  initial,
  repositories,
  workflowStates,
  loading,
  saving,
  onSave,
}: {
  initial: TeamRepoDefault | null;
  repositories: GitRepository[];
  workflowStates: WorkflowState[];
  loading: boolean;
  saving: boolean;
  onSave: (payload: {
    repository_id: string;
    base_branch?: string;
    branch_template?: string;
    auto_sync_states?: boolean;
    review_state_id?: string;
    done_state_id?: string;
  }) => void | Promise<void>;
}) {
  const [repositoryId, setRepositoryId] = useState(initial?.repository_id ?? '');
  const [baseBranch, setBaseBranch] = useState(initial?.base_branch ?? '');
  const [branchTemplate, setBranchTemplate] = useState(initial?.branch_template ?? 'tp-{display_id}-{slug}');
  const [autoSyncStates, setAutoSyncStates] = useState(initial?.auto_sync_states ?? true);
  const [reviewStateId, setReviewStateId] = useState(initial?.review_state_id ?? 'none');
  const [doneStateId, setDoneStateId] = useState(initial?.done_state_id ?? 'none');

  useEffect(() => {
    setRepositoryId(initial?.repository_id ?? '');
    setBaseBranch(initial?.base_branch ?? '');
    setBranchTemplate(initial?.branch_template ?? 'tp-{display_id}-{slug}');
    setAutoSyncStates(initial?.auto_sync_states ?? true);
    setReviewStateId(initial?.review_state_id ?? 'none');
    setDoneStateId(initial?.done_state_id ?? 'none');
  }, [initial]);

  const selectedRepository = repositories.find((repository) => repository.id === repositoryId) ?? null;

  return (
    <div className="space-y-5 py-2">
      <p className="text-sm text-muted-foreground">
        Stories on this team inherit these delivery defaults. Story detail can still override the repository or base branch.
      </p>
      <div className="space-y-2">
        <Label>Repository</Label>
        <Select value={repositoryId || undefined} onValueChange={setRepositoryId}>
          <SelectTrigger>
            <SelectValue placeholder={loading ? 'Loading repositories...' : 'Select repository'} />
          </SelectTrigger>
          <SelectContent>
            {repositories.map((repository) => (
              <SelectItem key={repository.id} value={repository.id}>
                {repository.full_name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        {selectedRepository && (
          <p className="text-xs text-muted-foreground">
            Default branch: {selectedRepository.default_branch}
          </p>
        )}
      </div>
      <div className="space-y-2">
        <Label>Base branch</Label>
        <Input
          value={baseBranch}
          onChange={(event) => setBaseBranch(event.target.value)}
          placeholder={selectedRepository?.default_branch || 'main'}
        />
      </div>
      <div className="space-y-2">
        <Label>Branch template</Label>
        <Input
          value={branchTemplate}
          onChange={(event) => setBranchTemplate(event.target.value)}
          placeholder="tp-{display_id}-{slug}"
        />
        <p className="text-xs text-muted-foreground">
          Available tokens: {'{display_id}'} and {'{slug}'}.
        </p>
      </div>
      <div className="flex items-center justify-between rounded-lg border border-border/60 px-3 py-2">
        <div>
          <p className="text-sm font-medium">Auto-sync workflow state from PR events</p>
          <p className="text-xs text-muted-foreground">
            When enabled, pull request webhooks can move stories forward automatically.
          </p>
        </div>
        <Switch checked={autoSyncStates} onCheckedChange={setAutoSyncStates} />
      </div>
      <div className="grid gap-4 sm:grid-cols-2">
        <div className="space-y-2">
          <Label>PR opened state</Label>
          <Select value={reviewStateId} onValueChange={setReviewStateId}>
            <SelectTrigger>
              <SelectValue placeholder="Choose review state" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="none">Do not map</SelectItem>
              {workflowStates.map((state) => (
                <SelectItem key={state.id} value={state.id}>
                  {state.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div className="space-y-2">
          <Label>PR merged state</Label>
          <Select value={doneStateId} onValueChange={setDoneStateId}>
            <SelectTrigger>
              <SelectValue placeholder="Choose done state" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="none">Do not map</SelectItem>
              {workflowStates.map((state) => (
                <SelectItem key={state.id} value={state.id}>
                  {state.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </div>
      {repositories.length === 0 && !loading && (
        <p className="text-xs text-muted-foreground">
          No repositories are synced yet. Connect GitHub and sync repositories before setting a team delivery default.
        </p>
      )}
      <DialogFooter>
        <Button
          disabled={saving || !repositoryId}
          onClick={() => onSave({
            repository_id: repositoryId,
            base_branch: baseBranch || selectedRepository?.default_branch || 'main',
            branch_template: branchTemplate || 'tp-{display_id}-{slug}',
            auto_sync_states: autoSyncStates,
            review_state_id: reviewStateId !== 'none' ? reviewStateId : undefined,
            done_state_id: doneStateId !== 'none' ? doneStateId : undefined,
          })}
        >
          {saving ? 'Saving...' : 'Save'}
        </Button>
      </DialogFooter>
    </div>
  );
}

/* ── Teams Tab ── */

export function TeamsTab({ workspaceId, teams, userMemberships, invitationPreassignments, teamEstimateSettings, teamFieldVisibility, teamRepoDefaults, editable, onRefresh, initialTeamId }: {
  workspaceId: string;
  teams: WorkspaceTeam[];
  userMemberships: TeamUserMembership[];
  invitationPreassignments: InvitationTeamPreassignment[];
  teamEstimateSettings: TeamEstimateSettings[];
  teamFieldVisibility: TeamFieldVisibility[];
  teamRepoDefaults: TeamRepoDefault[];
  editable: boolean;
  onRefresh: (silent?: boolean) => void | Promise<void>;
  initialTeamId?: string;
}) {
  const navigate = useNavigate();
  const { currentWorkspace } = useWorkspaceStore();
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editTeam, setEditTeam] = useState<WorkspaceTeam | null>(null);
  const [name, setName] = useState('');
  const [handle, setHandle] = useState('');
  const [description, setDescription] = useState('');
  const [teamType, setTeamType] = useState<TeamType>('engineering');
  const [defaultStoryType, setDefaultStoryType] = useState<DefaultStoryType>('feature');
  const [storyTypeTouched, setStoryTypeTouched] = useState(false);
  const [saving, setSaving] = useState(false);
  const [selectedTeamId, setSelectedTeamId] = useState<string | null>(initialTeamId ?? null);

  useEffect(() => {
    setSelectedTeamId(initialTeamId ?? null);
  }, [initialTeamId]);

  const [estimateDialogOpen, setEstimateDialogOpen] = useState(false);
  const [estimateSaving, setEstimateSaving] = useState(false);

  const [fieldVisDialogOpen, setFieldVisDialogOpen] = useState(false);
  const [fieldVisSaving, setFieldVisSaving] = useState(false);

  const [repoDialogOpen, setRepoDialogOpen] = useState(false);
  const [repoSaving, setRepoSaving] = useState(false);
  const [repositories, setRepositories] = useState<GitRepository[]>([]);
  const [repositoriesLoading, setRepositoriesLoading] = useState(false);
  const [workflows, setWorkflows] = useState<WorkflowWithStates[]>([]);
  const [workflowDialogOpen, setWorkflowDialogOpen] = useState(false);
  const [pipelineRules, setPipelineRules] = useState<AutomationRule[]>([]);
  const [pipelineAgents, setPipelineAgents] = useState<Agent[]>([]);

  const [memberDialogOpen, setMemberDialogOpen] = useState(false);
  const [workspaceMembers, setWorkspaceMembers] = useState<MemberWithUser[]>([]);
  const [invitations, setInvitations] = useState<Invitation[]>([]);
  const [, setMembersLoading] = useState(false);
  const [savingMember, setSavingMember] = useState(false);
  const [memberSearch, setMemberSearch] = useState('');
  const [deleteTeamConfirm, setDeleteTeamConfirm] = useState<string | null>(null);

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

  useEffect(() => {
    let mounted = true;
    const loadRepositories = async () => {
      setRepositoriesLoading(true);
      const { data, error } = await gitService.listRepositories(workspaceId);
      if (!mounted) return;
      if (error) {
        toast.error(error);
        setRepositories([]);
      } else {
        setRepositories(data ?? []);
      }
      setRepositoriesLoading(false);
    };
    loadRepositories();
    return () => { mounted = false; };
  }, [workspaceId]);

  useEffect(() => {
    let mounted = true;
    const loadWorkflows = async () => {
      const { data, error } = await pmWorkflowService.list(workspaceId);
      if (!mounted) return;
      if (error) {
        toast.error(error);
        setWorkflows([]);
      } else {
        setWorkflows(data ?? []);
      }
    };
    loadWorkflows();
    return () => { mounted = false; };
  }, [workspaceId]);

  // Load agents for pipeline builder
  useEffect(() => {
    agentService.list(workspaceId).then((res) => { if (res.data) setPipelineAgents(res.data.filter((a) => a.agent_kind === 'llm')); });
  }, [workspaceId]);

  const filteredTeams = [...teams].sort((a, b) => a.name.localeCompare(b.name));

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

  useEffect(() => {
    if (!storyTypeTouched) {
      setDefaultStoryType(TEAM_TYPE_PRESETS[teamType].defaultStoryType);
    }
  }, [teamType, storyTypeTouched]);

  const openCreate = () => {
    setEditTeam(null);
    setName('');
    setHandle('');
    setDescription('');
    setTeamType('engineering');
    setDefaultStoryType(TEAM_TYPE_PRESETS.engineering.defaultStoryType);
    setStoryTypeTouched(false);
    setDialogOpen(true);
  };

  const openEdit = (team: WorkspaceTeam) => {
    setEditTeam(team);
    setName(team.name);
    setHandle(team.handle ?? '');
    setDescription(team.description ?? '');
    setTeamType(normalizeTeamType(team.team_type));
    setDefaultStoryType(team.default_story_type ?? 'feature');
    setStoryTypeTouched(false);
    setDialogOpen(true);
  };

  const openMembers = (team: WorkspaceTeam) => {
    setSelectedTeamId(team.id);
    setMemberSearch('');
    setMemberDialogOpen(true);
  };

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setSaving(true);
    const payload = {
      name,
      handle: handle.trim() ? slugifyTeamHandle(handle) : undefined,
      description: description || undefined,
      team_type: teamType,
      default_story_type: defaultStoryType,
    };
    if (editTeam) {
      const { error } = await settingsService.updateTeam(workspaceId, editTeam.id, payload);
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
        const createdTeamId = data?.id;
        if (createdTeamId) {
          const preset = TEAM_TYPE_PRESETS[teamType];
          const [estimateRes, fieldVisRes] = await Promise.all([
            settingsService.updateTeamEstimateSettings(workspaceId, createdTeamId, preset.estimate),
            settingsService.updateTeamFieldVisibility(workspaceId, createdTeamId, buildPresetFieldVisibility(teamType)),
          ]);
          if (estimateRes.error) toast.error(estimateRes.error);
          if (fieldVisRes.error) toast.error(fieldVisRes.error);
        }
        toast.success('Team created');
        setDialogOpen(false);
        setSelectedTeamId(data?.id ?? null);
        await onRefresh();
      }
    }
    setSaving(false);
  };

  const handleDelete = async (id: string) => {
    const { error } = await settingsService.deleteTeam(workspaceId, id);
    if (error) toast.error(error);
    else {
      toast.success('Team deleted');
      setSelectedTeamId(null);
      await onRefresh();
    }
  };

  const handleRoleChange = async (teamId: string, userId: string, role: 'owner' | 'member') => {
    const { error } = await settingsService.updateTeamMember(workspaceId, teamId, userId, { role });
    if (error) toast.error(error);
    else {
      toast.success('Team member updated');
      await onRefresh(true);
    }
  };

  const handleRemoveMember = async (teamId: string, userId: string) => {
    const { error } = await settingsService.removeTeamMember(workspaceId, teamId, userId);
    if (error) toast.error(error);
    else {
      toast.success('Team member removed');
      await onRefresh(true);
    }
  };

  const handleAddInvitation = async (teamId: string, invitationId: string) => {
    const { error } = await settingsService.addTeamInvitation(workspaceId, teamId, invitationId);
    if (error) toast.error(error);
    else {
      toast.success('Invited member pre-assigned to team');
      await onRefresh(true);
    }
  };

  const handleRemoveInvitation = async (teamId: string, invitationId: string) => {
    const { error } = await settingsService.removeTeamInvitation(workspaceId, teamId, invitationId);
    if (error) toast.error(error);
    else {
      toast.success('Invitation pre-assignment removed');
      await onRefresh(true);
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
    const teamRepoDefault = teamRepoDefaults.find((config) => config.team_id === selectedTeam.id);
    const repoRecord = repositories.find((repository) => repository.id === teamRepoDefault?.repository_id);
    const teamWorkflowStates = workflows
      .filter(({ workflow }) => !workflow.team_id || workflow.team_id === selectedTeam.id)
      .flatMap(({ states }) => states);
    const deliveryMeta = teamRepoDefault
      ? `${repoRecord?.full_name ?? 'Repo selected'} · ${teamRepoDefault.base_branch}`
      : 'Not configured';
    const teamOwnWorkflow = workflows.find((w) => w.workflow.team_id === selectedTeam.id);
    const activeTeamWorkflow = teamOwnWorkflow ?? workflows.find((w) => !w.workflow.team_id);
    const workflowMeta = activeTeamWorkflow
      ? (
        <span className="flex flex-wrap items-center gap-1">
          {activeTeamWorkflow.states.slice().sort((a, b) => a.position - b.position).map((s, i) => (
            <span key={s.id} className="inline-flex items-center gap-1">
              {i > 0 && <span className="text-muted-foreground/50">→</span>}
              <span className="h-2 w-2 rounded-full shrink-0" style={{ backgroundColor: s.color ?? '#9ca3af' }} />
              <span>{s.name}</span>
            </span>
          ))}
        </span>
      )
      : 'Not configured';
    const settingsGroups: {
      label?: string;
      rows: { key: string; icon: LucideIcon; title: string; description: string; meta: React.ReactNode; action: () => void; disabled: boolean }[];
    }[] = [
      {
        rows: [
          {
            key: 'general',
            icon: Settings2,
            title: 'General',
            description: 'Name, identifier, team type, and story defaults',
            meta: [selectedTeam.handle ? `@${selectedTeam.handle}` : '', normalizeTeamType(selectedTeam.team_type) === 'engineering' ? 'Engineering' : 'Other', STORY_TYPE_LABELS[selectedTeam.default_story_type ?? 'feature']]
              .filter(Boolean)
              .join(' · '),
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
        label: 'Workflow',
        rows: [
          {
            key: 'workflow',
            icon: GitBranch,
            title: 'Workflow states',
            description: 'Manage workflow states for this team',
            meta: workflowMeta,
            action: () => {
              setWorkflowDialogOpen(true);
              if (activeTeamWorkflow) {
                automationRuleService.listByWorkflow(workspaceId, activeTeamWorkflow.workflow.id).then((res) => {
                  if (res.data) setPipelineRules(res.data);
                });
              }
            },
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
      {
        label: 'Story options',
        rows: [
          {
            key: 'field-visibility',
            icon: Eye,
            title: 'Story fields',
            description: 'Configure which fields and panels appear on stories',
            meta: fieldVisMeta,
            action: () => setFieldVisDialogOpen(true),
            disabled: !editable,
          },
          {
            key: 'labels',
            icon: Tag,
            title: 'Story labels',
            description: "Labels available to this team's stories",
            meta: '',
            action: () => openSettingsSection('labels'),
            disabled: false,
          },
          {
            key: 'estimates',
            icon: LayoutGrid,
            title: 'Story estimates',
            description: 'Configure estimate scale and options',
            meta: estimateMeta,
            action: () => setEstimateDialogOpen(true),
            disabled: !editable,
          },
        ],
      },
      {
        label: 'Development',
        rows: [
          {
            key: 'delivery-defaults',
            icon: GitPullRequest,
            title: 'Delivery defaults',
            description: repositories.length > 0 || teamRepoDefault
              ? 'Choose the team repository, base branch, and branch template'
              : 'Connect GitHub in Delivery settings to configure repository defaults',
            meta: repositories.length > 0 || teamRepoDefault ? deliveryMeta : 'Not connected',
            action: () => {
              if (repositories.length > 0 || teamRepoDefault) {
                setRepoDialogOpen(true);
              } else {
                openSettingsSection('delivery');
              }
            },
            disabled: !editable,
          },
        ],
      },
    ];

    return (
      <>
        <div className="space-y-8">
          <button
            type="button"
            onClick={() => {
              setSelectedTeamId(null);
              if (currentWorkspace?.slug) {
                navigate({
                  to: '/w/$slug/settings/$section',
                  params: { slug: currentWorkspace.slug, section: 'teams' },
                  search: {},
                });
              }
            }}
            className="inline-flex items-center gap-1.5 text-sm text-muted-foreground transition-colors hover:text-foreground"
          >
            <ChevronRight className="h-4 w-4 rotate-180" />
            Teams
          </button>

          <div className="flex items-center gap-4">
            {(() => {
              const color = getAvatarColor(selectedTeam.name);
              return (
                <div className={cn('flex h-12 w-12 shrink-0 items-center justify-center rounded-xl text-base font-semibold', color.bg, color.text)}>
                  {getInitials(selectedTeam.name)}
                </div>
              );
            })()}
            <div className="flex min-w-0 flex-1 items-center justify-between">
              <h2 className="text-xl font-semibold tracking-tight">{selectedTeam.name}</h2>
              {editable && (
                <div className="flex items-center gap-2">
                  <Button variant="outline" size="sm" onClick={() => openMembers(selectedTeam)}>
                    <Users className="h-4 w-4 mr-1" />
                    Add member
                  </Button>
                  <Button variant="outline" size="sm" className="text-destructive hover:text-destructive" onClick={() => setDeleteTeamConfirm(selectedTeam.id)}>
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
                  <Label>Description <span className="text-muted-foreground font-normal">(optional)</span></Label>
                  <Textarea
                    value={description}
                    onChange={e => setDescription(e.target.value)}
                    rows={3}
                    placeholder="Briefly describe what this team owns."
                  />
                </div>
                <div className="flex items-start gap-3 rounded-md border border-border/60 p-3">
                  <Checkbox
                    id="engineering-team"
                    checked={teamType === 'engineering'}
                    onCheckedChange={(checked) => setTeamType(checked ? 'engineering' : 'custom')}
                    className="mt-0.5"
                  />
                  <div className="space-y-1">
                    <Label htmlFor="engineering-team" className="cursor-pointer leading-tight">This is an engineering / dev team</Label>
                    <p className="text-xs text-muted-foreground">
                      Engineering teams get fibonacci estimates, sprints, epics, delivery tracking, and GitHub integration enabled by default. Non-engineering teams start with a simpler setup.
                    </p>
                  </div>
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
          <DialogContent className="max-w-xl gap-0 p-0">
            <DialogHeader className="border-b px-5 py-4">
              <DialogTitle className="text-base">{selectedTeam ? `${selectedTeam.name} members` : 'Team Members'}</DialogTitle>
              <p className="text-sm text-muted-foreground">Add or remove members who belong to this team.</p>
            </DialogHeader>

            {editable && (availableMembers.length > 0 || (selectedTeam && invitations.filter((inv) => !invitationPreassignments.some((pa) => pa.invitation_id === inv.id && pa.team_id === selectedTeam.id)).length > 0)) && (
              <div className="border-b">
                <div className="px-5 py-3">
                  <Input
                    placeholder="Search members to add..."
                    value={memberSearch}
                    onChange={(e) => setMemberSearch(e.target.value)}
                    className="h-8 text-sm"
                  />
                </div>
                <div className="max-h-[160px] overflow-y-auto border-t border-border/40">
                  {(() => {
                    const query = memberSearch.toLowerCase();
                    const filteredMembers = availableMembers.filter((m) =>
                      (m.full_name || '').toLowerCase().includes(query) || m.email.toLowerCase().includes(query)
                    );
                    const filteredInvitations = selectedTeam
                      ? invitations
                          .filter((inv) => !invitationPreassignments.some((pa) => pa.invitation_id === inv.id && pa.team_id === selectedTeam.id))
                          .filter((inv) => inv.email.toLowerCase().includes(query))
                      : [];

                    if (filteredMembers.length === 0 && filteredInvitations.length === 0) {
                      return <p className="px-5 py-3 text-xs text-muted-foreground text-center">No members to add</p>;
                    }

                    return (
                      <>
                        {filteredMembers.map((member) => (
                          <button
                            key={member.user_id}
                            type="button"
                            disabled={savingMember}
                            className="flex w-full items-center gap-3 px-5 py-2 text-left transition-colors hover:bg-accent cursor-pointer disabled:opacity-50"
                            onClick={async () => {
                              if (!selectedTeam) return;
                              setSavingMember(true);
                              const { error } = await settingsService.addTeamMember(workspaceId, selectedTeam.id, { user_id: member.user_id, role: 'member' });
                              setSavingMember(false);
                              if (error) { toast.error(error); return; }
                              toast.success(`${member.full_name || member.email} added`);
                              await onRefresh(true);
                            }}
                          >
                            <UserAvatar name={member.full_name || member.email} className="h-6 w-6" fallbackClassName="text-[9px]" />
                            <div className="min-w-0 flex-1">
                              <p className="truncate text-sm">{member.full_name || member.email}</p>
                              {member.full_name && <p className="truncate text-xs text-muted-foreground">{member.email}</p>}
                            </div>
                            <Plus className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                          </button>
                        ))}
                        {filteredInvitations.length > 0 && (
                          <>
                            {filteredMembers.length > 0 && (
                              <div className="px-5 py-1.5 text-[10px] font-medium uppercase tracking-wider text-muted-foreground/70">Pending invitations</div>
                            )}
                            {filteredInvitations.map((inv) => (
                              <button
                                key={inv.id}
                                type="button"
                                disabled={savingMember}
                                className="flex w-full items-center gap-3 px-5 py-2 text-left transition-colors hover:bg-accent cursor-pointer disabled:opacity-50"
                                onClick={async () => {
                                  if (!selectedTeam) return;
                                  await handleAddInvitation(selectedTeam.id, inv.id);
                                }}
                              >
                                <UserAvatar name={inv.email} className="h-6 w-6" fallbackClassName="text-[9px]" />
                                <div className="min-w-0 flex-1">
                                  <p className="truncate text-sm">{inv.email}</p>
                                </div>
                                <Badge variant="secondary" className="text-[10px] px-1.5 py-0">Invited</Badge>
                                <Plus className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                              </button>
                            ))}
                          </>
                        )}
                      </>
                    );
                  })()}
                </div>
              </div>
            )}

            <div className="max-h-[400px] overflow-y-auto">
              {selectedTeam && (teamMembers.length > 0 || invitationPreassignments.filter((pa) => pa.team_id === selectedTeam.id).length > 0) && (
                <div className="px-5 pt-3 pb-1">
                  <p className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground/70">Current members</p>
                </div>
              )}
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
              <DialogTitle>Story Estimates</DialogTitle>
            </DialogHeader>
            <EstimateSettingsForm
              teamId={selectedTeam.id}
              initial={teamEstConfig ?? null}
              saving={estimateSaving}
              onSave={async (data) => {
                setEstimateSaving(true);
                const { error } = await settingsService.updateTeamEstimateSettings(workspaceId, selectedTeam.id, data);
                if (data.enabled !== undefined) {
                  await settingsService.updateTeamFieldVisibility(workspaceId, selectedTeam.id, { estimate: data.enabled });
                }
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

        {/* Story Display Dialog */}
        <Dialog open={fieldVisDialogOpen} onOpenChange={setFieldVisDialogOpen}>
          <DialogContent className="max-w-md">
            <DialogHeader>
              <DialogTitle>Story Fields</DialogTitle>
            </DialogHeader>
            <FieldVisibilityForm
              teamId={selectedTeam.id}
              initial={teamVisConfig ?? null}
              saving={fieldVisSaving}
              onSave={async (data) => {
                setFieldVisSaving(true);
                const { error } = await settingsService.updateTeamFieldVisibility(workspaceId, selectedTeam.id, data);
                if ('estimate' in data) {
                  await settingsService.updateTeamEstimateSettings(workspaceId, selectedTeam.id, { enabled: data.estimate });
                }
                setFieldVisSaving(false);
                if (error) {
                  toast.error(error);
                } else {
                  toast.success('Story fields updated');
                  setFieldVisDialogOpen(false);
                  await onRefresh();
                }
              }}
            />
          </DialogContent>
        </Dialog>

        <Dialog open={repoDialogOpen} onOpenChange={setRepoDialogOpen}>
          <DialogContent className="max-w-md">
            <DialogHeader>
              <DialogTitle>Delivery Defaults</DialogTitle>
            </DialogHeader>
            <TeamRepoDefaultForm
              initial={teamRepoDefault ?? null}
              repositories={repositories}
              workflowStates={teamWorkflowStates}
              loading={repositoriesLoading}
              saving={repoSaving}
              onSave={async (data) => {
                setRepoSaving(true);
                const { error } = await settingsService.updateTeamRepoDefault(workspaceId, selectedTeam.id, data);
                setRepoSaving(false);
                if (error) {
                  toast.error(error);
                } else {
                  toast.success('Delivery defaults updated');
                  setRepoDialogOpen(false);
                  await onRefresh();
                }
              }}
            />
          </DialogContent>
        </Dialog>

        {/* Workflow States Editor Dialog */}
        <Dialog open={workflowDialogOpen} onOpenChange={setWorkflowDialogOpen}>
          <DialogContent className="sm:max-w-5xl">
            <DialogHeader>
              <DialogTitle>Workflow States</DialogTitle>
            </DialogHeader>
            {activeTeamWorkflow ? (
              <>
                <PipelineBuilder
                  workspaceId={workspaceId}
                  workflowId={activeTeamWorkflow.workflow.id}
                  states={activeTeamWorkflow.states.slice().sort((a, b) => a.position - b.position)}
                  agents={pipelineAgents}
                  rules={pipelineRules}
                  editable={editable}
                  onChanged={() => {
                    automationRuleService.listByWorkflow(workspaceId, activeTeamWorkflow.workflow.id).then((res) => {
                      if (res.data) setPipelineRules(res.data);
                    });
                  }}
                />
                <TeamWorkflowStateEditor
                  workspaceId={workspaceId}
                  workflow={activeTeamWorkflow}
                  editable={editable}
                  onUpdate={(updated) => {
                    setWorkflows((prev) => prev.map((w) => w.workflow.id === updated.workflow.id ? updated : w));
                  }}
                />
              </>
            ) : (
              <p className="text-sm text-muted-foreground py-2">No workflow configured for this team.</p>
            )}
            <DialogFooter>
              <Button onClick={() => setWorkflowDialogOpen(false)}>Done</Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      <ConfirmDialog
        open={deleteTeamConfirm !== null}
        onOpenChange={(open) => { if (!open) setDeleteTeamConfirm(null); }}
        title="Delete team"
        description="This will permanently delete the team and remove all member assignments. This action cannot be undone."
        confirmLabel="Delete"
        variant="destructive"
        onConfirm={() => { if (deleteTeamConfirm) handleDelete(deleteTeamConfirm); setDeleteTeamConfirm(null); }}
      />
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
              Create teams, assign members, and manage team-level settings.
            </p>
          </div>
          {editable && (
            <Button size="sm" onClick={openCreate}>
              <Plus className="h-4 w-4 mr-1" />
              New Team
            </Button>
          )}
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
                  <TableHead className="w-[100px]">Workflow</TableHead>
                  <TableHead>Description</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {filteredTeams.map((team) => {
                  const teamMembers = getTeamMemberships(team.id);
                  const memberCount = teamMembers.length;
                  return (
                    <TableRow
                      key={team.id}
                      className="cursor-pointer"
                      onClick={() => {
                        setSelectedTeamId(team.id);
                        if (currentWorkspace?.slug) {
                          navigate({
                            to: '/w/$slug/settings/$section',
                            params: { slug: currentWorkspace.slug, section: 'teams' },
                            search: { team: team.id },
                          });
                        }
                      }}
                    >
                      <TableCell>
                        <div className="flex items-center gap-3">
                          {(() => {
                            const color = getAvatarColor(team.name);
                            return (
                              <div className={cn('flex h-8 w-8 shrink-0 items-center justify-center rounded-lg text-xs font-semibold', color.bg, color.text)}>
                                {getInitials(team.name)}
                              </div>
                            );
                          })()}
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
                        <div className="flex items-center gap-1.5">
                          <div className="flex -space-x-1.5">
                            {teamMembers.slice(0, 4).map(({ membership, user }) => (
                              <UserAvatar
                                key={membership.id}
                                name={user?.full_name ?? user?.email ?? '?'}
                                className="h-5 w-5 ring-1 ring-background"
                              />
                            ))}
                          </div>
                          {memberCount > 4 && (
                            <span className="text-xs text-muted-foreground">+{memberCount - 4}</span>
                          )}
                          {memberCount === 0 && (
                            <span className="text-xs text-muted-foreground/50">&mdash;</span>
                          )}
                        </div>
                      </TableCell>
                      <TableCell>
                        <span className="text-sm text-muted-foreground">
                          {(() => {
                            const wf = workflows.find((w) => w.workflow.team_id === team.id);
                            return wf ? `${wf.states.length} states` : '\u2014';
                          })()}
                        </span>
                      </TableCell>
                      <TableCell>
                        <span className="line-clamp-1 text-sm text-muted-foreground">
                          {team.description || '\u2014'}
                        </span>
                      </TableCell>
                    </TableRow>
                  );
                })}
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
                <Label>Description <span className="text-muted-foreground font-normal">(optional)</span></Label>
                <Textarea
                  value={description}
                  onChange={e => setDescription(e.target.value)}
                  rows={3}
                  placeholder="Briefly describe what this team owns."
                />
              </div>
              <div className="flex items-start gap-3 rounded-md border border-border/60 p-3">
                <Checkbox
                  id="create-engineering-team"
                  checked={teamType === 'engineering'}
                  onCheckedChange={(checked) => setTeamType(checked ? 'engineering' : 'custom')}
                  className="mt-0.5"
                />
                <div className="space-y-1">
                  <Label htmlFor="create-engineering-team" className="cursor-pointer leading-tight">This is an engineering / dev team</Label>
                  <p className="text-xs text-muted-foreground">
                    Engineering teams get fibonacci estimates, sprints, epics, delivery tracking, and GitHub integration enabled by default. Non-engineering teams start with a simpler setup.
                  </p>
                </div>
              </div>
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setDialogOpen(false)}>Cancel</Button>
              <Button type="submit" disabled={saving}>{saving ? 'Saving...' : 'Save'}</Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      <ConfirmDialog
        open={deleteTeamConfirm !== null}
        onOpenChange={(open) => { if (!open) setDeleteTeamConfirm(null); }}
        title="Delete team"
        description="This will permanently delete the team and remove all member assignments. This action cannot be undone."
        confirmLabel="Delete"
        variant="destructive"
        onConfirm={() => { if (deleteTeamConfirm) handleDelete(deleteTeamConfirm); setDeleteTeamConfirm(null); }}
      />
    </>
  );
}

// ── Inline Workflow State Editor ──────────────────────────────────────

const STATE_TYPE_ORDER: StateType[] = ['backlog', 'unstarted', 'started', 'done'];
const STATE_TYPE_LABEL: Record<StateType, string> = {
  backlog: 'Backlog',
  unstarted: 'Not started',
  started: 'Started',
  done: 'Done',
};

function TeamWorkflowStateEditor({
  workspaceId,
  workflow,
  editable,
  onUpdate,
}: {
  workspaceId: string;
  workflow: WorkflowWithStates;
  editable: boolean;
  onUpdate: (updated: WorkflowWithStates) => void;
}) {
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editName, setEditName] = useState('');
  const [saving, setSaving] = useState(false);
  const [addingType, setAddingType] = useState<StateType | null>(null);
  const [newName, setNewName] = useState('');
  const [newColor, setNewColor] = useState(PRESET_COLORS[0]);
  const [deleteConfirm, setDeleteConfirm] = useState<string | null>(null);

  const sorted = workflow.states.slice().sort((a, b) => a.position - b.position);
  const grouped = STATE_TYPE_ORDER.map((type) => ({
    type,
    label: STATE_TYPE_LABEL[type],
    states: sorted.filter((s) => s.state_type === type),
  })).filter((g) => g.states.length > 0 || addingType === g.type);

  const handleRename = async (state: WorkflowState) => {
    const trimmed = editName.trim();
    if (!trimmed || trimmed === state.name) {
      setEditingId(null);
      return;
    }
    setSaving(true);
    const { data, error } = await pmWorkflowService.updateState(workspaceId, workflow.workflow.id, state.id, { name: trimmed });
    setSaving(false);
    if (error) { toast.error(error); return; }
    if (data) {
      onUpdate({
        ...workflow,
        states: workflow.states.map((s) => s.id === state.id ? { ...s, name: trimmed } : s),
      });
    }
    setEditingId(null);
  };

  const handleColorChange = async (state: WorkflowState, color: string) => {
    const { error } = await pmWorkflowService.updateState(workspaceId, workflow.workflow.id, state.id, { color });
    if (error) { toast.error(error); return; }
    onUpdate({
      ...workflow,
      states: workflow.states.map((s) => s.id === state.id ? { ...s, color } : s),
    });
  };

  const handleAdd = async () => {
    const trimmed = newName.trim();
    if (!trimmed || !addingType) return;
    setSaving(true);
    const statesOfType = sorted.filter((s) => s.state_type === addingType);
    const position = statesOfType.length > 0 ? statesOfType[statesOfType.length - 1].position + 1 : sorted.length;
    const { data, error } = await pmWorkflowService.createState(workspaceId, workflow.workflow.id, {
      name: trimmed,
      state_type: addingType,
      position,
      color: newColor,
    });
    setSaving(false);
    if (error) { toast.error(error); return; }
    if (data && typeof data === 'object' && 'id' in data && 'workflow_id' in data) {
      const createdState = data as WorkflowState;
      onUpdate({ ...workflow, states: [...workflow.states, createdState] });
    }
    setNewName('');
    setNewColor(PRESET_COLORS[0]);
    setAddingType(null);
  };

  const handleDelete = async (stateId: string) => {
    setSaving(true);
    const { error } = await pmWorkflowService.removeState(workspaceId, workflow.workflow.id, stateId);
    setSaving(false);
    if (error) { toast.error(error); return; }
    onUpdate({ ...workflow, states: workflow.states.filter((s) => s.id !== stateId) });
    setDeleteConfirm(null);
  };

  return (
    <div className="space-y-4 py-2">
      {grouped.map((group) => (
        <div key={group.type}>
          <p className="text-[11px] font-medium uppercase tracking-wider text-muted-foreground mb-1.5">
            {group.label}
          </p>
          <div className="space-y-1">
            {group.states.map((state) => (
              <div
                key={state.id}
                className="group flex items-center gap-2 rounded-md border border-border bg-background px-2.5 py-1.5"
              >
                <Popover>
                  <PopoverTrigger asChild>
                    <button
                      type="button"
                      className="h-3.5 w-3.5 shrink-0 rounded-full border border-border/60 transition-transform hover:scale-110"
                      style={{ backgroundColor: state.color ?? '#9ca3af' }}
                      disabled={!editable}
                    />
                  </PopoverTrigger>
                  {editable && (
                    <PopoverContent className="w-auto p-2" align="start">
                      <ColorPicker value={state.color ?? '#9ca3af'} onChange={(c) => handleColorChange(state, c)} />
                    </PopoverContent>
                  )}
                </Popover>

                {editingId === state.id ? (
                  <Input
                    autoFocus
                    value={editName}
                    onChange={(e) => setEditName(e.target.value)}
                    onBlur={() => handleRename(state)}
                    onKeyDown={(e) => { if (e.key === 'Enter') handleRename(state); if (e.key === 'Escape') setEditingId(null); }}
                    className="h-6 flex-1 text-xs px-1 py-0"
                    disabled={saving}
                  />
                ) : (
                  <span className="flex-1 text-sm truncate">{state.name}</span>
                )}

                {editable && editingId !== state.id && (
                  <div className="flex items-center gap-0.5 opacity-0 group-hover:opacity-100 transition-opacity">
                    <button
                      type="button"
                      className="h-5 w-5 flex items-center justify-center rounded text-muted-foreground hover:text-foreground hover:bg-muted"
                      onClick={() => { setEditingId(state.id); setEditName(state.name); }}
                    >
                      <Pencil className="h-3 w-3" />
                    </button>
                    {workflow.states.length > 1 && (
                      <button
                        type="button"
                        className="h-5 w-5 flex items-center justify-center rounded text-muted-foreground hover:text-destructive hover:bg-muted"
                        onClick={() => setDeleteConfirm(state.id)}
                      >
                        <Trash2 className="h-3 w-3" />
                      </button>
                    )}
                  </div>
                )}
              </div>
            ))}

            {addingType === group.type && (
              <div className="flex items-center gap-2 rounded-md border border-primary/40 bg-background px-2.5 py-1.5">
                <Popover>
                  <PopoverTrigger asChild>
                    <button
                      type="button"
                      className="h-3.5 w-3.5 shrink-0 rounded-full border border-border/60 transition-transform hover:scale-110"
                      style={{ backgroundColor: newColor }}
                    />
                  </PopoverTrigger>
                  <PopoverContent className="w-auto p-2" align="start">
                    <ColorPicker value={newColor} onChange={setNewColor} />
                  </PopoverContent>
                </Popover>
                <Input
                  autoFocus
                  value={newName}
                  onChange={(e) => setNewName(e.target.value)}
                  onKeyDown={(e) => { if (e.key === 'Enter') handleAdd(); if (e.key === 'Escape') { setAddingType(null); setNewName(''); } }}
                  placeholder="State name..."
                  className="h-6 flex-1 text-xs px-1 py-0 border-0 shadow-none focus-visible:ring-0"
                  disabled={saving}
                />
                <button
                  type="button"
                  className="h-5 w-5 flex items-center justify-center rounded text-primary hover:bg-muted"
                  onClick={handleAdd}
                  disabled={saving || !newName.trim()}
                >
                  <Check className="h-3.5 w-3.5" />
                </button>
                <button
                  type="button"
                  className="h-5 w-5 flex items-center justify-center rounded text-muted-foreground hover:bg-muted"
                  onClick={() => { setAddingType(null); setNewName(''); }}
                >
                  <X className="h-3.5 w-3.5" />
                </button>
              </div>
            )}
          </div>

          {editable && addingType !== group.type && (
            <button
              type="button"
              className="mt-1 flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground transition-colors"
              onClick={() => { setAddingType(group.type); setNewName(''); setNewColor(PRESET_COLORS[Math.floor(Math.random() * PRESET_COLORS.length)]); }}
            >
              <Plus className="h-3 w-3" /> Add state
            </button>
          )}
        </div>
      ))}

      {!editable && (
        <p className="text-xs text-muted-foreground">You don't have permission to edit workflow states.</p>
      )}

      <ConfirmDialog
        open={deleteConfirm !== null}
        onOpenChange={(open) => { if (!open) setDeleteConfirm(null); }}
        title="Delete state"
        description="Stories in this state will need to be moved to another state. This cannot be undone."
        confirmLabel="Delete"
        variant="destructive"
        onConfirm={() => { if (deleteConfirm) handleDelete(deleteConfirm); }}
      />
    </div>
  );
}
