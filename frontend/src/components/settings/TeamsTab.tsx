import { useEffect, useRef, useState, type FormEvent } from 'react';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { settingsService } from '@/lib/services/settingsService';
import { gitService } from '@/lib/services/gitService';
import { workspacesService } from '@/lib/services/workspacesService';
import { inviteService } from '@/lib/services/inviteService';
import { pmWorkflowService } from '@/lib/services/pmWorkflowService';
import { automationRuleService } from '@/lib/services/automationRuleService';
import { agentService } from '@/lib/services/agentService';
import { PipelineBuilder, type PipelineBuilderHandle } from './PipelineBuilder';
import { SCALE_LABELS } from '@/lib/estimateScales';
import type { WorkspaceTeam, MemberWithUser, Invitation, TeamUserMembership, InvitationTeamPreassignment, TeamEstimateSettings, TeamFieldVisibility, TeamRepoDefault } from '@/lib/types';
import type { Agent, AutomationRule, GitRepository, WorkflowWithStates } from '@/lib/pmTypes';
import type { SettingsSection } from '@/lib/settingsSections';
import { UserAvatar, getAvatarColor } from '@/components/pm/UserAvatar';
import { ConfirmDialog } from '@/components/pm/ConfirmDialog';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { cn, getInitials } from '@/lib/utils';
import { Checkbox } from '@/components/ui/checkbox';
import { ArrowRight01Icon, ViewIcon, File01Icon, GitBranchIcon, GitPullRequestIcon, LayoutGridIcon, PlusSignIcon, ArrowReloadHorizontalIcon, Settings02Icon, Tag01Icon, Delete01Icon, UserGroupIcon, Cancel01Icon, type IconComponent } from '@/lib/icons';
import { useDocsSpaces, useUpdateDocsSpace } from '@/hooks/queries';
import { useNavigate } from '@tanstack/react-router';
import { toast } from 'sonner';
import {
  buildPresetFieldVisibility,
  normalizeTeamType,
  slugifyTeamHandle,
  TEAM_TYPE_PRESETS,
  type DefaultTaskType,
  type TeamType,
} from '@/lib/teamPresets';
import { EstimateSettingsForm } from './teams/EstimateSettingsForm';
import { SprintSettingsForm } from '@/components/settings/teams/SprintSettingsForm';
import { pmAutomationService } from '@/lib/services/pmAutomationService';
import type { PMAutomation } from '@/lib/pmTypes';
import { FIELD_VISIBILITY_FIELDS, FieldVisibilityForm } from './teams/FieldVisibilityForm';
import { TeamRepoDefaultForm } from './teams/TeamRepoDefaultForm';
import { StoredIcon } from '@/components/ui/icon-picker';
import { QuietSearchInput, workspaceSidebarSafeInsetClassName } from '@/components/design-system/quiet';

/* ── Teams Tab ── */

export function TeamsTab({ workspaceId, teams, userMemberships, invitationPreassignments, teamEstimateSettings, teamFieldVisibility, teamRepoDefaults, editable, onRefresh, initialTeamId, initialSection, access }: {
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
  initialSection?: string;
  access?: import('@/lib/types').WorkspaceAccess | null;
}) {
  const navigate = useNavigate();
  const { currentWorkspace } = useWorkspaceStore();
  const myUserId = access?.membership?.user_id;
  const isWsAdmin = access?.membership?.role === 'owner' || access?.membership?.role === 'admin';
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editTeam, setEditTeam] = useState<WorkspaceTeam | null>(null);
  const [name, setName] = useState('');
  const [handle, setHandle] = useState('');
  const [description, setDescription] = useState('');
  const [teamType, setTeamType] = useState<TeamType>('engineering');
  const [defaultTaskType, setDefaultTaskType] = useState<DefaultTaskType>('feature');
  const [taskTypeTouched, setTaskTypeTouched] = useState(false);
  const [saving, setSaving] = useState(false);
  const [selectedTeamId, setSelectedTeamId] = useState<string | null>(initialTeamId ?? null);
  const isSelectedTeamManager = selectedTeamId
    ? (access?.team_memberships ?? []).some(tm => tm.team_id === selectedTeamId && tm.role === 'owner')
    : false;
  const teamEditable = editable || isSelectedTeamManager;

  useEffect(() => {
    setSelectedTeamId(initialTeamId ?? null);
  }, [initialTeamId]);

  const [estimateDialogOpen, setEstimateDialogOpen] = useState(false);
  const [estimateSaving, setEstimateSaving] = useState(false);

  const [sprintDialogOpen, setSprintDialogOpen] = useState(false);
  const [sprintSaving, setSprintSaving] = useState(false);
  const [automations, setAutomations] = useState<PMAutomation[]>([]);

  // Auto-open sprint dialog when navigated with ?section=sprints
  useEffect(() => {
    if (initialSection === 'sprints' && selectedTeamId) {
      setSprintDialogOpen(true);
    }
  }, [initialSection, selectedTeamId]);

  const [fieldVisDialogOpen, setFieldVisDialogOpen] = useState(false);
  const [fieldVisSaving, setFieldVisSaving] = useState(false);

  const [repoDialogOpen, setRepoDialogOpen] = useState(false);
  const [repoSaving, setRepoSaving] = useState(false);

  // Auto-open delivery defaults dialog when navigated with ?section=delivery
  useEffect(() => {
    if (initialSection === 'delivery' && selectedTeamId) {
      setRepoDialogOpen(true);
    }
  }, [initialSection, selectedTeamId]);
  const [repositories, setRepositories] = useState<GitRepository[]>([]);
  const [repositoriesLoading, setRepositoriesLoading] = useState(false);
  const [workflows, setWorkflows] = useState<WorkflowWithStates[]>([]);
  const [workflowDialogOpen, setWorkflowDialogOpen] = useState(false);
  const [workflowHasUnsavedChanges, setWorkflowHasUnsavedChanges] = useState(false);
  const [workflowJustSaved, setWorkflowJustSaved] = useState(false);
  const [workflowCloseConfirmOpen, setWorkflowCloseConfirmOpen] = useState(false);
  const workflowBuilderRef = useRef<PipelineBuilderHandle | null>(null);
  const workflowSavedTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const [pipelineRules, setPipelineRules] = useState<AutomationRule[]>([]);
  const [pipelineAgents, setPipelineAgents] = useState<Agent[]>([]);

  const [memberDialogOpen, setMemberDialogOpen] = useState(false);
  const [workspaceMembers, setWorkspaceMembers] = useState<MemberWithUser[]>([]);
  const [invitations, setInvitations] = useState<Invitation[]>([]);
  const [, setMembersLoading] = useState(false);
  const [savingMember, setSavingMember] = useState(false);
  const [memberSearch, setMemberSearch] = useState('');
  const [deleteTeamConfirm, setDeleteTeamConfirm] = useState<string | null>(null);
  const [spacesDialogOpen, setSpacesDialogOpen] = useState(false);
  const [spaceSaving, setSpaceSaving] = useState<string | null>(null);
  const { data: allSpaces } = useDocsSpaces(workspaceId);
  const updateSpace = useUpdateDocsSpace(workspaceId);

  useEffect(() => {
    let mounted = true;
    const loadMembers = async () => {
      setMembersLoading(true);
      try {
        const [membersRes, invitesRes] = await Promise.all([
          workspacesService.listMembers(workspaceId),
          (editable || isSelectedTeamManager) ? inviteService.list(workspaceId) : Promise.resolve({ data: null, error: null }),
        ]);
        if (!mounted) return;
        if (membersRes.error) {
          if (editable || isSelectedTeamManager) toast.error(membersRes.error);
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
  }, [workspaceId, editable, isSelectedTeamManager]);

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
    agentService.list(workspaceId).then((res) => { if (res.data) setPipelineAgents(res.data); });
  }, [workspaceId]);

  // Load automations for sprint settings
  useEffect(() => {
    pmAutomationService.list(workspaceId).then((res) => { if (res.data) setAutomations(res.data); });
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
    if (!taskTypeTouched) {
      setDefaultTaskType(TEAM_TYPE_PRESETS[teamType].defaultTaskType);
    }
  }, [teamType, taskTypeTouched]);

  const openCreate = () => {
    setEditTeam(null);
    setName('');
    setHandle('');
    setDescription('');
    setTeamType('engineering');
    setDefaultTaskType(TEAM_TYPE_PRESETS.engineering.defaultTaskType);
    setTaskTypeTouched(false);
    setDialogOpen(true);
  };

  const openEdit = (team: WorkspaceTeam) => {
    setEditTeam(team);
    setName(team.name);
    setHandle(team.handle ?? '');
    setDescription(team.description ?? '');
    setTeamType(normalizeTeamType(team.team_type));
    setDefaultTaskType(team.default_task_type ?? 'feature');
    setTaskTypeTouched(false);
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
      default_task_type: defaultTaskType,
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

  useEffect(() => {
    return () => {
      if (workflowSavedTimerRef.current) clearTimeout(workflowSavedTimerRef.current);
    };
  }, []);

  const markWorkflowSaved = () => {
    setWorkflowJustSaved(true);
    if (workflowSavedTimerRef.current) clearTimeout(workflowSavedTimerRef.current);
    workflowSavedTimerRef.current = setTimeout(() => {
      setWorkflowJustSaved(false);
      workflowSavedTimerRef.current = null;
    }, 2200);
  };

  const closeWorkflowDialog = () => {
    setWorkflowDialogOpen(false);
    setWorkflowCloseConfirmOpen(false);
    setWorkflowHasUnsavedChanges(false);
    setWorkflowJustSaved(false);
  };

  const handleWorkflowDialogOpenChange = (open: boolean) => {
    if (open) {
      setWorkflowDialogOpen(true);
      return;
    }
    if (workflowHasUnsavedChanges || workflowBuilderRef.current?.hasUnsavedChanges()) {
      setWorkflowCloseConfirmOpen(true);
      return;
    }
    closeWorkflowDialog();
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
      ? `${repoRecord?.full_name ?? 'Repo selected'} · default base ${teamRepoDefault.base_branch}`
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
      rows: { key: string; icon: IconComponent; title: string; description: string; meta: React.ReactNode; action: () => void; disabled: boolean }[];
    }[] = [
      {
        rows: [
          {
            key: 'general',
            icon: Settings02Icon,
            title: 'General',
            description: 'Name, identifier, team type, and task defaults',
            meta: [selectedTeam.handle ? `@${selectedTeam.handle}` : '', normalizeTeamType(selectedTeam.team_type) === 'engineering' ? 'Engineering / dev team' : 'Non-engineering team']
              .filter(Boolean)
              .join(' · '),
            action: () => openEdit(selectedTeam),
            disabled: !teamEditable,
          },
          {
            key: 'members',
            icon: UserGroupIcon,
            title: 'Members',
            description: 'Manage team members',
            meta: (() => {
              const managers = teamMembers.filter(({ membership }) => membership.role === 'owner');
              return (
                <span className="flex items-center gap-1">
                  {memberCount} member{memberCount === 1 ? '' : 's'}
                  {managers.length > 0 && (
                    <>
                      <span className="text-muted-foreground/50">·</span>
                      <Tooltip>
                        <TooltipTrigger asChild>
                          <span className="cursor-default">{managers.map(({ user }) => user?.full_name || user?.email || 'Unknown').join(', ')}</span>
                        </TooltipTrigger>
                        <TooltipContent side="top">Team Manager</TooltipContent>
                      </Tooltip>
                    </>
                  )}
                </span>
              );
            })(),
            action: () => openMembers(selectedTeam),
            disabled: !teamEditable,
          },
        ],
      },
      {
        label: 'Workflow',
        rows: [
          {
            key: 'workflow',
            icon: GitBranchIcon,
            title: 'Workflow states',
            description: 'Manage workflow states for this team',
            meta: workflowMeta,
            action: async () => {
              // If team is using the shared workspace default, resolve a team-specific copy first.
              if (!teamOwnWorkflow && selectedTeam) {
                const { data: resolved } = await pmWorkflowService.resolveTeamWorkflow(workspaceId, selectedTeam.id);
                if (resolved) {
                  setWorkflows((prev) => {
                    const exists = prev.some((w) => w.workflow.id === resolved.workflow.id);
                    return exists ? prev.map((w) => w.workflow.id === resolved.workflow.id ? resolved : w) : [...prev, resolved];
                  });
                }
              }
              setWorkflowHasUnsavedChanges(false);
              setWorkflowJustSaved(false);
              setWorkflowDialogOpen(true);
              const wf = teamOwnWorkflow ?? workflows.find((w) => !w.workflow.team_id);
              if (wf) {
                automationRuleService.listByWorkflow(workspaceId, wf.workflow.id).then((res) => {
                  if (res.data) setPipelineRules(res.data);
                });
              }
            },
            disabled: false,
          },
          {
            key: 'automations',
            icon: ArrowReloadHorizontalIcon,
            title: 'Automations',
            description: 'Sprint and epic automations for this team',
            meta: '',
            action: () => openSettingsSection('automations'),
            disabled: false,
          },
        ],
      },
      {
        label: 'Task options',
        rows: [
          {
            key: 'field-visibility',
            icon: ViewIcon,
            title: 'Task fields',
            description: 'Configure which fields and panels appear on tasks',
            meta: fieldVisMeta,
            action: () => setFieldVisDialogOpen(true),
            disabled: !teamEditable,
          },
          {
            key: 'labels',
            icon: Tag01Icon,
            title: 'Task labels',
            description: "Labels available to this team's tasks",
            meta: '',
            action: () => openSettingsSection('labels'),
            disabled: false,
          },
          {
            key: 'estimates',
            icon: LayoutGridIcon,
            title: 'Task estimates',
            description: 'Configure estimate scale and options',
            meta: estimateMeta,
            action: () => setEstimateDialogOpen(true),
            disabled: !teamEditable,
          },
          {
            key: 'sprints',
            icon: ArrowReloadHorizontalIcon,
            title: 'Sprints',
            description: 'Enable sprints and configure automation',
            meta: selectedTeam?.sprints_enabled !== false ? 'Enabled' : 'Disabled',
            action: () => setSprintDialogOpen(true),
            disabled: !teamEditable,
          },
        ],
      },
      {
        label: 'Development',
        rows: [
          {
            key: 'delivery-defaults',
            icon: GitPullRequestIcon,
            title: 'Delivery defaults',
            description: repositories.length > 0 || teamRepoDefault
              ? 'Choose the team repository, default base branch, and task branch template'
              : 'Add workspace repositories before configuring repository defaults',
            meta: repositories.length > 0 || teamRepoDefault ? deliveryMeta : 'Not connected',
            action: () => {
              if (repositories.length > 0 || teamRepoDefault) {
                setRepoDialogOpen(true);
              } else {
                openSettingsSection('repositories');
              }
            },
            disabled: !teamEditable,
          },
        ],
      },
      {
        label: 'Docs',
        rows: [
          {
            key: 'spaces',
            icon: File01Icon,
            title: 'Spaces',
            description: 'Docs spaces this team has access to',
            meta: (() => {
              const teamSpaces = (allSpaces ?? []).filter(sp => sp.visibility === 'workspace_wide' || sp.team_ids?.includes(selectedTeam.id));
              if (teamSpaces.length === 0) return 'None';
              return `${teamSpaces.length} space${teamSpaces.length === 1 ? '' : 's'}`;
            })(),
            action: () => setSpacesDialogOpen(true),
            disabled: !teamEditable,
          },
        ],
      },
    ];

    return (
      <>
        <div className={cn('space-y-8', workspaceSidebarSafeInsetClassName)}>
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
            <ArrowRight01Icon className="h-4 w-4 rotate-180" />
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
              {teamEditable && (
                <div className="flex items-center gap-2">
                  <Button variant="outline" size="sm" onClick={() => openMembers(selectedTeam)}>
                    <UserGroupIcon className="h-4 w-4 mr-1" />
                    Add member
                  </Button>
                  {editable && (
                    <Button variant="outline" size="sm" className="text-destructive hover:text-destructive" onClick={() => setDeleteTeamConfirm(selectedTeam.id)}>
                      <Delete01Icon className="h-4 w-4 mr-1" />
                      Delete
                    </Button>
                  )}
                </div>
              )}
            </div>
          </div>

          {settingsGroups.map((group) => (
            <div key={group.label ?? '_default'} className="space-y-3">
              {group.label && (
                <h3 className="text-sm font-medium text-muted-foreground">{group.label}</h3>
              )}
              <div className="overflow-hidden rounded-lg border border-border bg-card">
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
                    <ArrowRight01Icon className="h-4 w-4 shrink-0 text-muted-foreground" />
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
                {editTeam && (
                  <div className="space-y-2">
                    <Label>Handle</Label>
                    <Input
                      value={handle}
                      onChange={e => setHandle(e.target.value)}
                      placeholder={slugifyTeamHandle(name) || 'growth'}
                    />
                    <p className="text-xs text-muted-foreground">Used for mentions like @{slugifyTeamHandle(handle || name) || 'team'}.</p>
                  </div>
                )}
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
                      Engineering teams get development workflows, Git repository fields, and pre-defined settings. Non-engineering teams start with a simpler setup.
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
          <DialogContent className="sm:max-w-3xl gap-0 p-0">
            <DialogHeader className="border-b px-5 py-4">
              <DialogTitle className="text-base">{selectedTeam ? `${selectedTeam.name} members` : 'Team Members'}</DialogTitle>
              <p className="text-sm text-muted-foreground">Manage who belongs to this team.</p>
            </DialogHeader>

            <div className="grid grid-cols-1 sm:grid-cols-2 min-h-[300px]">
              {/* Left column — Current members */}
              <div className="flex flex-col border-r border-border/40">
                <div className="px-4 pt-3 pb-2">
                  <p className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground/70">
                    Members ({teamMembers.length}{selectedTeam && invitationPreassignments.filter((pa) => pa.team_id === selectedTeam.id).length > 0 ? ` + ${invitationPreassignments.filter((pa) => pa.team_id === selectedTeam.id).length} pending` : ''})
                  </p>
                </div>
                <div className="flex-1 overflow-y-auto max-h-[360px]">
                  {selectedTeam && (teamMembers.length > 0 || invitationPreassignments.some((pa) => pa.team_id === selectedTeam.id)) ? (
                    <div className="divide-y divide-border/40">
                      {[...teamMembers].sort((a, b) => {
                        if (a.membership.role === 'owner' && b.membership.role !== 'owner') return -1;
                        if (a.membership.role !== 'owner' && b.membership.role === 'owner') return 1;
                        return 0;
                      }).map(({ membership, user }) => {
                        const isSelf = membership.user_id === myUserId;
                        const isTeamManager = membership.role === 'owner';
                        const canEditMember = teamEditable && !isSelf && (isWsAdmin || !isTeamManager);

                        return (
                          <div key={membership.id} className="group/member flex items-center gap-2.5 px-4 py-2.5">
                            <UserAvatar
                              name={user?.full_name || user?.email || membership.user_id}
                              avatarUrl={user?.avatar_url ?? undefined}
                              avatarStyle={user?.avatar_style ?? undefined}
                              avatarSeed={user?.avatar_seed ?? undefined}
                              avatarBackgroundMode={user?.avatar_background_mode ?? undefined}
                              avatarBackgroundColor={user?.avatar_background_color ?? undefined}
                              className="h-7 w-7"
                              fallbackClassName="text-[10px]"
                            />
                            <div className="min-w-0 flex-1">
                              <div className="flex items-center gap-1.5">
                                <p className="truncate text-sm font-medium">{user?.full_name || 'Unknown user'}</p>
                                {isSelf && <Badge variant="outline" className="text-[10px] px-1 py-0">You</Badge>}
                              </div>
                              <p className="truncate text-xs text-muted-foreground">{user?.email || membership.user_id}</p>
                            </div>
                            <div className="relative flex items-center shrink-0">
                              {canEditMember ? (
                                <>
                                  {isTeamManager && (
                                    <Badge variant="secondary" className="text-[10px] px-1.5 py-0 group-hover/member:invisible">Team Manager</Badge>
                                  )}
                                  <div className="absolute right-0 flex items-center gap-0.5 opacity-0 group-hover/member:opacity-100 transition-opacity">
                                    <button
                                      type="button"
                                      className="rounded px-1.5 py-0.5 text-[10px] text-muted-foreground hover:bg-accent hover:text-foreground transition-colors cursor-pointer whitespace-nowrap"
                                      onClick={() => handleRoleChange(membership.team_id, membership.user_id, isTeamManager ? 'member' : 'owner')}
                                    >
                                      {isTeamManager ? 'Remove as manager' : 'Make manager'}
                                    </button>
                                    <Tooltip>
                                      <TooltipTrigger asChild>
                                        <Button size="icon" variant="ghost" className="h-6 w-6 text-muted-foreground hover:text-destructive" onClick={() => handleRemoveMember(membership.team_id, membership.user_id)}>
                                          <Cancel01Icon className="h-3 w-3" />
                                        </Button>
                                      </TooltipTrigger>
                                      <TooltipContent side="top">Remove from team</TooltipContent>
                                    </Tooltip>
                                  </div>
                                </>
                              ) : isTeamManager ? (
                                <Badge variant="secondary" className="text-[10px] px-1.5 py-0">Team Manager</Badge>
                              ) : null}
                            </div>
                          </div>
                        );
                      })}
                      {invitationPreassignments
                        .filter((pa) => pa.team_id === selectedTeam.id)
                        .filter((pa) => invitations.some((i) => i.id === pa.invitation_id))
                        .map((pa) => {
                          const inv = invitations.find((i) => i.id === pa.invitation_id);
                          return (
                            <div key={pa.id} className="group/member flex items-center gap-2.5 px-4 py-2.5 opacity-60">
                              <UserAvatar name={inv?.email || pa.invitation_id} className="h-7 w-7" fallbackClassName="text-[10px]" />
                              <div className="min-w-0 flex-1">
                                <p className="truncate text-sm font-medium">{inv?.email || 'Pending'}</p>
                              </div>
                              <div className="relative flex items-center shrink-0 gap-1.5">
                                <Badge variant="outline" className="text-[10px] px-1.5 py-0 group-hover/member:invisible">Pending</Badge>
                                {teamEditable && (
                                  <Tooltip>
                                    <TooltipTrigger asChild>
                                      <Button size="icon" variant="ghost" className="absolute right-0 h-6 w-6 text-muted-foreground hover:text-destructive opacity-0 group-hover/member:opacity-100 transition-opacity" onClick={() => handleRemoveInvitation(selectedTeam.id, pa.invitation_id)}>
                                        <Cancel01Icon className="h-3 w-3" />
                                      </Button>
                                    </TooltipTrigger>
                                    <TooltipContent side="top">Remove from team</TooltipContent>
                                  </Tooltip>
                                )}
                              </div>
                            </div>
                          );
                        })}
                    </div>
                  ) : (
                    <div className="flex items-center justify-center h-full px-4">
                      <p className="text-sm text-muted-foreground">No members yet</p>
                    </div>
                  )}
                </div>
                <div className="border-t border-border/40 px-4 py-2">
                  <p className="text-[10px] text-muted-foreground">Team managers can manage team settings and members.</p>
                </div>
              </div>

              {/* Right column — Add members */}
              <div className="flex flex-col">
                <div className="px-4 pt-3 pb-2">
                  <p className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground/70">Add members</p>
                </div>
                {teamEditable ? (
                  <>
                    <div className="px-4 pb-2">
                      <QuietSearchInput
                        placeholder="Search members..."
                        value={memberSearch}
                        onChange={(e) => setMemberSearch(e.target.value)}
                      />
                    </div>
                    <div className="flex-1 overflow-y-auto max-h-[320px]">
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
                          return <p className="px-4 py-6 text-xs text-muted-foreground text-center">No members to add</p>;
                        }

                        return (
                          <div className="divide-y divide-border/40">
                            {filteredMembers.map((member) => (
                              <button
                                key={member.user_id}
                                type="button"
                                disabled={savingMember}
                                className="flex w-full items-center gap-2.5 px-4 py-2 text-left transition-colors hover:bg-accent cursor-pointer disabled:opacity-50"
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
                                <UserAvatar
                                  name={member.full_name || member.email}
                                  avatarUrl={member.avatar_url}
                                  avatarStyle={member.avatar_style}
                                  avatarSeed={member.avatar_seed}
                                  avatarBackgroundMode={member.avatar_background_mode}
                                  avatarBackgroundColor={member.avatar_background_color}
                                  className="h-6 w-6"
                                  fallbackClassName="text-[9px]"
                                />
                                <div className="min-w-0 flex-1">
                                  <p className="truncate text-sm">{member.full_name || member.email}</p>
                                  {member.full_name && <p className="truncate text-xs text-muted-foreground">{member.email}</p>}
                                </div>
                                <PlusSignIcon className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                              </button>
                            ))}
                            {filteredInvitations.length > 0 && (
                              <>
                                {filteredMembers.length > 0 && (
                                  <div className="px-4 py-1.5 text-[10px] font-medium uppercase tracking-wider text-muted-foreground/70">Pending invitations</div>
                                )}
                                {filteredInvitations.map((inv) => (
                                  <button
                                    key={inv.id}
                                    type="button"
                                    disabled={savingMember}
                                    className="flex w-full items-center gap-2.5 px-4 py-2 text-left transition-colors hover:bg-accent cursor-pointer disabled:opacity-50"
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
                                    <PlusSignIcon className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                                  </button>
                                ))}
                              </>
                            )}
                          </div>
                        );
                      })()}
                    </div>
                  </>
                ) : (
                  <div className="flex items-center justify-center flex-1 px-4">
                    <p className="text-sm text-muted-foreground">You don't have permission to add members.</p>
                  </div>
                )}
              </div>
            </div>
          </DialogContent>
        </Dialog>

        {/* Estimate Settings Dialog */}
        <Dialog open={estimateDialogOpen} onOpenChange={setEstimateDialogOpen}>
          <DialogContent className="max-w-md">
            <DialogHeader>
              <DialogTitle>Task Estimates</DialogTitle>
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

        {/* Sprint Settings Dialog */}
        <Dialog open={sprintDialogOpen} onOpenChange={setSprintDialogOpen}>
          <DialogContent className="max-w-md">
            <DialogHeader>
              <DialogTitle>Sprint Settings</DialogTitle>
            </DialogHeader>
            <SprintSettingsForm
              teamId={selectedTeam?.id ?? ''}
              sprintsEnabled={selectedTeam?.sprints_enabled !== false}
              autoCreateConfig={automations.find((a) => a.automation_type === 'sprint_auto_create' && a.team_id === selectedTeam?.id) ?? null}
              moveUnfinishedConfig={automations.find((a) => a.automation_type === 'sprint_move_unfinished' && a.team_id === selectedTeam?.id) ?? null}
              saving={sprintSaving}
              onSave={async (data) => {
                if (!workspaceId || !selectedTeam) return;
                setSprintSaving(true);
                try {
                  // Update sprints_enabled on team
                  await settingsService.updateTeam(workspaceId, selectedTeam.id, { sprints_enabled: data.sprints_enabled });
                  // Upsert auto-create automation
                  if (data.sprints_enabled && data.auto_create_enabled) {
                    await pmAutomationService.upsert(workspaceId, {
                      automation_type: 'sprint_auto_create',
                      enabled: true,
                      workspace_id: workspaceId,
                      team_id: selectedTeam.id,
                      config_int: data.auto_create_upcoming_count,
                      config_int2: data.auto_create_duration_weeks,
                      config_int3: data.auto_create_start_day,
                    });
                  } else {
                    const existing = automations.find((a) => a.automation_type === 'sprint_auto_create' && a.team_id === selectedTeam.id);
                    if (existing) await pmAutomationService.upsert(workspaceId, { ...existing, enabled: false });
                  }
                  // Upsert move-unfinished automation
                  if (data.sprints_enabled && data.move_unfinished_enabled) {
                    await pmAutomationService.upsert(workspaceId, {
                      automation_type: 'sprint_move_unfinished',
                      enabled: true,
                      workspace_id: workspaceId,
                      team_id: selectedTeam.id,
                    });
                  } else {
                    const existing = automations.find((a) => a.automation_type === 'sprint_move_unfinished' && a.team_id === selectedTeam.id);
                    if (existing) await pmAutomationService.upsert(workspaceId, { ...existing, enabled: false });
                  }
                  // Refresh automations + parent data
                  const refreshed = await pmAutomationService.list(workspaceId);
                  if (refreshed.data) setAutomations(refreshed.data);
                  await onRefresh();
                } catch {
                  // Error handled silently, data refreshed
                }
                setSprintSaving(false);
                setSprintDialogOpen(false);
                // Clear section param so dialog doesn't re-open on next render
                if (initialSection && currentWorkspace?.slug) {
                  navigate({ to: '/w/$slug/settings/teams', params: { slug: currentWorkspace.slug }, search: { team: selectedTeamId ?? undefined }, replace: true });
                }
              }}
            />
          </DialogContent>
        </Dialog>

        {/* Task Display Dialog */}
        <Dialog open={fieldVisDialogOpen} onOpenChange={setFieldVisDialogOpen}>
          <DialogContent className="max-w-md">
            <DialogHeader>
              <DialogTitle>Task Fields</DialogTitle>
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
                  toast.success('Task fields updated');
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
              workspaceId={workspaceId}
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
        <Dialog open={workflowDialogOpen} onOpenChange={handleWorkflowDialogOpenChange}>
          <DialogContent className="flex max-h-[90vh] flex-col gap-0 overflow-hidden p-0 sm:max-w-6xl">
            <DialogHeader className="border-b border-border px-5 py-4">
              <div className="flex min-h-8 items-start justify-between gap-4 pr-14">
                <div className="min-w-0">
                  <DialogTitle>Workflow states</DialogTitle>
                  <p className="mt-1 text-xs text-muted-foreground">
                    Manage states, types, and the automation that runs when tasks enter each state.
                  </p>
                </div>
                {(workflowHasUnsavedChanges || workflowJustSaved) && (
                  <p
                    className={cn(
                      'mt-0.5 text-xs',
                      workflowHasUnsavedChanges && 'text-amber-600 dark:text-amber-400',
                      !workflowHasUnsavedChanges && workflowJustSaved && 'text-emerald-600 dark:text-emerald-400',
                    )}
                  >
                    {workflowHasUnsavedChanges ? 'Unsaved changes' : 'Saved'}
                  </p>
                )}
              </div>
            </DialogHeader>
            <div className="min-h-0 flex-1 space-y-6 overflow-y-auto px-5 py-4">
              {activeTeamWorkflow ? (
                <>
                  <PipelineBuilder
                    ref={workflowBuilderRef}
                    workspaceId={workspaceId}
                    workflow={activeTeamWorkflow}
                    agents={pipelineAgents}
                    rules={pipelineRules}
                    editable={teamEditable}
                    onChanged={() => {
                      automationRuleService.listByWorkflow(workspaceId, activeTeamWorkflow.workflow.id).then((res) => {
                        if (res.data) setPipelineRules(res.data);
                      });
                    }}
                    onWorkflowUpdate={(updated) => {
                      setWorkflows((prev) => prev.map((w) => w.workflow.id === updated.workflow.id ? updated : w));
                    }}
                    onUnsavedChange={setWorkflowHasUnsavedChanges}
                    onSaved={markWorkflowSaved}
                  />
                </>
              ) : (
                <p className="text-sm text-muted-foreground py-2">No workflow configured for this team.</p>
              )}
            </div>
          </DialogContent>
        </Dialog>
        <ConfirmDialog
          open={workflowCloseConfirmOpen}
          onOpenChange={(open) => {
            if (!open) setWorkflowCloseConfirmOpen(false);
          }}
          title="Discard unsaved changes?"
          description="You have a pending workflow edit that has not been saved yet."
          confirmLabel="Discard"
          cancelLabel="Keep editing"
          variant="destructive"
          onConfirm={closeWorkflowDialog}
        />

      <Dialog open={spacesDialogOpen} onOpenChange={setSpacesDialogOpen}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>Docs spaces</DialogTitle>
          </DialogHeader>
          <div className="space-y-3">
            <p className="text-sm text-muted-foreground">
              Toggle which docs spaces this team can access.
            </p>
            {!allSpaces || allSpaces.length === 0 ? (
              <p className="text-sm text-muted-foreground italic">No docs spaces created yet.</p>
            ) : (
              <>
              <div className="flex flex-wrap gap-1.5">
                {allSpaces.map((space) => {
                  const isWorkspaceWide = space.visibility === 'workspace_wide';
                  const selected = isWorkspaceWide || (space.team_ids?.includes(selectedTeamId ?? '') ?? false);
                  const isSaving = spaceSaving === space.id;
                  const pill = (
                    <button
                      type="button"
                      disabled={isSaving || isWorkspaceWide}
                      onClick={async () => {
                        if (!selectedTeamId || isWorkspaceWide) return;
                        setSpaceSaving(space.id);
                        const currentTeamIds = space.team_ids ?? [];
                        const newTeamIds = selected
                          ? currentTeamIds.filter(id => id !== selectedTeamId)
                          : [...currentTeamIds, selectedTeamId];
                        try {
                          await updateSpace.mutateAsync({ id: space.id, team_ids: newTeamIds, set_team_ids: true });
                        } catch {
                          toast.error('Failed to update space');
                        } finally {
                          setSpaceSaving(null);
                        }
                      }}
                      className={cn(
                        'rounded-md border px-2.5 py-1 text-xs transition-colors',
                        selected
                          ? 'border-primary bg-primary/10 text-primary font-medium'
                          : 'border-border text-muted-foreground hover:border-primary/50 hover:text-foreground',
                        isWorkspaceWide && 'opacity-60 cursor-default',
                        isSaving && 'opacity-50 cursor-wait',
                      )}
                    >
                      <span className="inline-flex items-center gap-1">
                        <StoredIcon name={space.icon} className="h-4 w-4 shrink-0" textClassName="" />
                        <span>{space.name}{isWorkspaceWide ? ' (all teams)' : ''}</span>
                      </span>
                    </button>
                  );
                  return <span key={space.id}>{pill}</span>;
                })}
              </div>
              <p className="text-[11px] text-muted-foreground mt-2">
                <span className="font-medium">Note:</span> Workspace-wide spaces can't be removed from here. Edit visibility from space settings.
              </p>
              </>
            )}
          </div>
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
            <p className="text-sm text-muted-foreground">
              Create teams, assign members, and manage team-level settings.
            </p>
          </div>
          {editable && (
            <Button size="sm" onClick={openCreate}>
              <PlusSignIcon className="h-4 w-4 mr-1" />
              New Team
            </Button>
          )}
        </div>

        {teams.length === 0 ? (
          <div className="flex min-h-[320px] flex-col items-center justify-center gap-3 rounded-lg border border-dashed border-border text-center">
            <div className="flex h-12 w-12 items-center justify-center rounded-xl border border-border bg-muted/60">
              <UserGroupIcon className="h-5 w-5 text-muted-foreground" />
            </div>
            <div className="space-y-1">
              <p className="font-medium">No teams yet</p>
              <p className="text-sm text-muted-foreground">
                Create your first team to manage memberships, labels, and workflows.
              </p>
            </div>
            {editable && (
              <Button onClick={openCreate}>
                <PlusSignIcon className="h-4 w-4 mr-1" />
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
                  <TableHead className="w-[120px]">Members</TableHead>
                  <TableHead className="w-[180px]">Team Manager</TableHead>
                  <TableHead className="w-[100px]">Workflow</TableHead>
                  <TableHead>Docs Spaces</TableHead>
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
                        {memberCount > 0 ? (
                          <Tooltip>
                            <TooltipTrigger asChild>
                              <div className="flex items-center gap-1.5 cursor-default">
                                <div className="flex -space-x-1.5">
                                  {teamMembers.slice(0, 4).map(({ membership, user }) => (
                                    <UserAvatar
                                      key={membership.id}
                                      name={user?.full_name ?? user?.email ?? '?'}
                                      avatarUrl={user?.avatar_url ?? undefined}
                                      avatarStyle={user?.avatar_style ?? undefined}
                                      avatarSeed={user?.avatar_seed ?? undefined}
                                      avatarBackgroundMode={user?.avatar_background_mode ?? undefined}
                                      avatarBackgroundColor={user?.avatar_background_color ?? undefined}
                                      className="h-5 w-5 ring-1 ring-background"
                                    />
                                  ))}
                                </div>
                                {memberCount > 4 && (
                                  <span className="text-xs text-muted-foreground">+{memberCount - 4}</span>
                                )}
                              </div>
                            </TooltipTrigger>
                            <TooltipContent side="bottom" className="p-2">
                              <div className="space-y-1.5">
                                {teamMembers.map(({ membership, user }) => (
                                  <div key={membership.id} className="flex items-center gap-2">
                                    <UserAvatar
                                      name={user?.full_name ?? user?.email ?? '?'}
                                      avatarUrl={user?.avatar_url ?? undefined}
                                      avatarStyle={user?.avatar_style ?? undefined}
                                      avatarSeed={user?.avatar_seed ?? undefined}
                                      avatarBackgroundMode={user?.avatar_background_mode ?? undefined}
                                      avatarBackgroundColor={user?.avatar_background_color ?? undefined}
                                      className="h-5 w-5"
                                    />
                                    <span className="text-xs">{user?.full_name ?? user?.email ?? '?'}</span>
                                  </div>
                                ))}
                              </div>
                            </TooltipContent>
                          </Tooltip>
                        ) : (
                          <span className="text-xs text-muted-foreground/50">&mdash;</span>
                        )}
                      </TableCell>
	                      <TableCell>
	                        {(() => {
	                          const managers = teamMembers.filter(({ membership }) => membership.role === 'owner');
	                          if (managers.length === 0) return <span className="text-xs text-muted-foreground/50">&mdash;</span>;
	                          const managerAvatars = (
	                            <div className="flex items-center gap-1.5">
	                              <div className="flex -space-x-1.5">
	                                {managers.slice(0, 4).map(({ membership, user }) => (
	                                  <UserAvatar
	                                    key={membership.id}
	                                    name={user?.full_name ?? user?.email ?? '?'}
                                    avatarUrl={user?.avatar_url ?? undefined}
                                    avatarStyle={user?.avatar_style ?? undefined}
                                    avatarSeed={user?.avatar_seed ?? undefined}
                                    avatarBackgroundMode={user?.avatar_background_mode ?? undefined}
                                    avatarBackgroundColor={user?.avatar_background_color ?? undefined}
	                                    className="h-5 w-5 ring-1 ring-background"
	                                  />
	                                ))}
	                              </div>
	                              {managers.length > 4 && (
	                                <span className="text-xs text-muted-foreground">+{managers.length - 4}</span>
	                              )}
	                              {managers.length === 1 && (
	                                <span className="truncate text-sm text-muted-foreground">
	                                  {managers[0].user?.full_name || managers[0].user?.email || 'Unknown'}
	                                </span>
	                              )}
	                            </div>
	                          );
	                          if (managers.length === 1) return managerAvatars;
	                          return (
	                            <Tooltip>
	                              <TooltipTrigger asChild>
	                                <div className="inline-flex cursor-default">{managerAvatars}</div>
	                              </TooltipTrigger>
	                              <TooltipContent side="bottom" className="p-2">
	                                <div className="space-y-1.5">
	                                  {managers.map(({ membership, user }) => (
	                                    <div key={membership.id} className="flex items-center gap-2">
	                                      <UserAvatar
	                                        name={user?.full_name ?? user?.email ?? '?'}
	                                        avatarUrl={user?.avatar_url ?? undefined}
	                                        avatarStyle={user?.avatar_style ?? undefined}
	                                        avatarSeed={user?.avatar_seed ?? undefined}
	                                        avatarBackgroundMode={user?.avatar_background_mode ?? undefined}
	                                        avatarBackgroundColor={user?.avatar_background_color ?? undefined}
	                                        className="h-5 w-5"
	                                      />
	                                      <span className="text-xs">{user?.full_name ?? user?.email ?? '?'}</span>
	                                    </div>
	                                  ))}
	                                </div>
	                              </TooltipContent>
	                            </Tooltip>
	                          );
	                        })()}
	                      </TableCell>
                      <TableCell>
                        {(() => {
                          const wf = workflows.find((w) => w.workflow.team_id === team.id);
                          if (!wf) return <span className="text-sm text-muted-foreground/50">&mdash;</span>;
                          return (
                            <Tooltip>
                              <TooltipTrigger asChild>
                                <span className="text-sm text-muted-foreground cursor-default">{wf.states.length} states</span>
                              </TooltipTrigger>
                              <TooltipContent side="top">
                                {wf.states.map((s) => s.name).join(' → ')}
                              </TooltipContent>
                            </Tooltip>
                          );
                        })()}
                      </TableCell>
                      <TableCell>
                        {(() => {
                          const teamSpaces = (allSpaces ?? []).filter(sp => sp.visibility === 'workspace_wide' || sp.team_ids?.includes(team.id));
                          if (teamSpaces.length === 0) return <span className="text-sm text-muted-foreground/50">&mdash;</span>;
                          if (teamSpaces.length === 1) return <span className="text-sm text-muted-foreground">{teamSpaces[0].name}</span>;
                          return (
                            <Tooltip>
                              <TooltipTrigger asChild>
                                <span className="text-sm text-muted-foreground cursor-default">{teamSpaces.length} spaces</span>
                              </TooltipTrigger>
                              <TooltipContent side="top">{teamSpaces.map(sp => sp.name).join(', ')}</TooltipContent>
                            </Tooltip>
                          );
                        })()}
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
                    Engineering teams get fibonacci estimates, sprints, epics, delivery tracking, and Git repository fields enabled by default. Non-engineering teams start with a simpler setup.
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
