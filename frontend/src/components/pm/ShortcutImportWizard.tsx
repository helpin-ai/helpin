import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Progress } from '@/components/ui/progress';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Badge } from '@/components/ui/badge';
import { cn } from '@/lib/utils';
import {
  Alert01Icon,
  Tick01Icon,
  ArrowDown01Icon,
  ArrowRight01Icon,
  Key01Icon,
  Loading01Icon,
  Mail01Icon,
  Cancel01Icon,
} from '@/lib/icons';
import { toast } from 'sonner';
import {
  pmImportService,
  type ShortcutImportOptionsPayload,
  type ShortcutImportPreviewResponse,
  type ShortcutImportStatusResponse,
  type WorkflowStateMappingPayload,
} from '@/lib/services/pmImportService';
import { TASK_TYPE_CONFIG, TaskTypeIcon } from '@/lib/pmConstants';
import type { TaskType, WorkflowWithStates } from '@/lib/pmTypes';
import type { MemberWithUser } from '@/lib/types';
import { pmWorkflowService } from '@/lib/services/pmWorkflowService';
import { inviteService } from '@/lib/services/inviteService';
import { useWorkspaceTeams } from '@/hooks/useWorkspaceTeams';

// ─── Types ───────────────────────────────────────────────────────────

type StateType = 'backlog' | 'unstarted' | 'started' | 'done';

interface WorkflowMapping {
  shortcutWorkflowId: string;
  shortcutWorkflowName: string;
  mode: 'create_new' | 'use_existing';
  newWorkflowName: string;
  states: {
    shortcutState: string;
    newStateName: string;
    stateType: StateType;
    position: number;
    storyCount: number;
    existingStateId: string;
  }[];
  existingWorkflowId: string;
  expanded: boolean;
}

type UserAction = 'matched' | 'invite' | 'skip';

interface UserMapping {
  email: string;
  storyCount: number;
  matchedUserId: string | null;
  matchedName: string | null;
  shortcutName: string | null;
  action: UserAction;
  manualUserId: string | null;
  invited: boolean;
}

interface ShortcutScanProgress {
  message: string;
  phase: string;
  processed: number;
  total: number;
}

interface TeamMapping {
  shortcutTeamName: string;
  taskCount: number;
  mode: 'create_new' | 'use_existing';
  newTeamName: string;
  existingTeamId: string;
}

type WizardStep = 0 | 1 | 2 | 3 | 4;

const STEP_LABELS = ['Connect', 'Teams', 'Workflows', 'Users', 'Import'];

const IMPORT_STEPS_API = ['API Enrichment', 'Teams', 'Workflows', 'Labels', 'Objectives', 'Epics', 'Sprints', 'Tasks', 'Media', 'Comments'];
const LOOKBACK_OPTIONS = [
  { value: '0', label: 'All time' },
  { value: '3', label: 'Last 3 months' },
  { value: '6', label: 'Last 6 months' },
  { value: '12', label: 'Last 12 months' },
  { value: '24', label: 'Last 24 months' },
];

function formatImportStepLabel(step?: string | null) {
  switch (step) {
    case 'api_enrichment':
      return 'API Enrichment';
    case 'teams':
      return 'Teams';
    case 'workflows':
      return 'Workflows';
    case 'labels':
      return 'Labels';
    case 'objectives':
      return 'Objectives';
    case 'epics':
      return 'Epics';
    case 'sprints':
      return 'Sprints';
    case 'stories':
      return 'Tasks';
    case 'story_media':
      return 'Media';
    case 'comments':
      return 'Comments';
    case 'parse':
      return 'Preparing import';
    case 'completed':
      return 'Completed';
    default:
      return step || 'unknown';
  }
}

function normalizeImportName(value: string) {
  return value.trim().toLowerCase();
}

function buildInitialTeamMappings(
  preview: ShortcutImportPreviewResponse,
  existingTeams: { id: string; name: string }[],
): TeamMapping[] {
  const existingByName = new Map(existingTeams.map((team) => [normalizeImportName(team.name), team]));
  return preview.teams.map((team) => {
    const existing = existingByName.get(normalizeImportName(team.name));
    return {
      shortcutTeamName: team.name,
      taskCount: team.task_count,
      mode: existing ? 'use_existing' : 'create_new',
      newTeamName: team.name,
      existingTeamId: existing?.id || '',
    };
  });
}

function normalizeWorkflowStateName(value: string) {
  return value.trim().toLowerCase().replace(/[^a-z0-9]+/g, ' ');
}

function autoMapExistingWorkflowStates(
  states: WorkflowMapping['states'],
  existingStates: WorkflowWithStates['states'],
) {
  const byName = new Map(existingStates.map((state) => [normalizeWorkflowStateName(state.name), state.id]));
  return states.map((state) => ({
    ...state,
    existingStateId: byName.get(normalizeWorkflowStateName(state.shortcutState)) || '',
  }));
}

function buildInitialWorkflowMappings(
  preview: ShortcutImportPreviewResponse,
  existingWorkflows: WorkflowWithStates[],
): WorkflowMapping[] {
  return preview.workflows.map((wf) => {
    const states = wf.states.map((s, i) => ({
      shortcutState: s.name,
      newStateName: s.name,
      stateType: (s.suggested_type || 'unstarted') as StateType,
      position: i,
      storyCount: s.task_count,
      existingStateId: '',
    }));
    const existing =
      existingWorkflows.find((candidate) => normalizeImportName(candidate.workflow.name) === normalizeImportName(wf.name)) ||
      existingWorkflows[0];
    return {
      shortcutWorkflowId: wf.id || '',
      shortcutWorkflowName: wf.name,
      mode: 'use_existing' as const,
      newWorkflowName: wf.name,
      states: existing ? autoMapExistingWorkflowStates(states, existing.states) : states,
      existingWorkflowId: existing?.workflow.id || '',
      expanded: true,
    };
  });
}

function workflowStateMappingStats(states: WorkflowMapping['states']) {
  const mapped = states.filter((state) => state.existingStateId).length;
  return { mapped, total: states.length };
}

// ─── Main Component ──────────────────────────────────────────────────

interface ShortcutImportWizardProps {
  workspaceId: string;
  members: MemberWithUser[];
}

export function ShortcutImportWizard({ workspaceId, members }: ShortcutImportWizardProps) {
  const { teams: existingTeams } = useWorkspaceTeams(workspaceId);
  const [step, setStep] = useState<WizardStep>(0);
  const [preview, setPreview] = useState<ShortcutImportPreviewResponse | null>(null);
  const [previewLoading, setPreviewLoading] = useState(false);
  const [workflowMappings, setWorkflowMappings] = useState<WorkflowMapping[]>([]);
  const [teamMappings, setTeamMappings] = useState<TeamMapping[]>([]);
  const [userMappings, setUserMappings] = useState<UserMapping[]>([]);
  const [existingWorkflows, setExistingWorkflows] = useState<WorkflowWithStates[]>([]);
  const [importArchived, setImportArchived] = useState(true);
  const [importCompleted, setImportCompleted] = useState(true);
  const [storyDateField, setStoryDateField] = useState<'updated_at' | 'created_at'>('updated_at');
  const [storyLookbackMonths, setStoryLookbackMonths] = useState('0');
  const [epicLookbackMonths, setEpicLookbackMonths] = useState('0');
  const [objectiveLookbackMonths, setObjectiveLookbackMonths] = useState('0');
  const [maxStories, setMaxStories] = useState('');
  const [scanProgress, setScanProgress] = useState<ShortcutScanProgress | null>(null);
  const [apiToken, setApiToken] = useState('');
  const [importStatus, setImportStatus] = useState<ShortcutImportStatusResponse | null>(null);
  const [importing, setImporting] = useState(false);
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const scanIdRef = useRef<string | null>(null);

  // Cleanup polling on unmount
  useEffect(() => {
    return () => {
      if (pollRef.current) clearInterval(pollRef.current);
    };
  }, []);

  useEffect(() => {
    const handleProgress = (event: Event) => {
      const detail = (event as CustomEvent).detail;
      const data = detail?.data as Record<string, unknown> | undefined;
      if (!data || typeof data.scan_id !== 'string' || data.scan_id !== scanIdRef.current) return;
      setScanProgress({
        message: typeof data.message === 'string' ? data.message : 'Scanning Shortcut workspace',
        phase: typeof data.phase === 'string' ? data.phase : 'scanning',
        processed: typeof data.processed === 'number' ? data.processed : 0,
        total: typeof data.total === 'number' ? data.total : 0,
      });
    };
    window.addEventListener('pm_import_preview-updated', handleProgress);
    return () => window.removeEventListener('pm_import_preview-updated', handleProgress);
  }, []);

  const importOptions = useMemo<ShortcutImportOptionsPayload>(() => {
    const parsePositiveInt = (value: string) => {
      const parsed = Number.parseInt(value, 10);
      return Number.isFinite(parsed) && parsed > 0 ? parsed : undefined;
    };
    return {
      import_archived: importArchived,
      import_completed: importCompleted,
      story_date_field: storyDateField,
      story_lookback_months: parsePositiveInt(storyLookbackMonths),
      epic_lookback_months: parsePositiveInt(epicLookbackMonths),
      objective_lookback_months: parsePositiveInt(objectiveLookbackMonths),
      max_stories: parsePositiveInt(maxStories),
    };
  }, [
    importArchived,
    importCompleted,
    storyDateField,
    storyLookbackMonths,
    epicLookbackMonths,
    objectiveLookbackMonths,
    maxStories,
  ]);

  // ─── Step 0: API Preview ─────────────────────────────────────────

  const handleAPIPreview = useCallback(async () => {
    if (!apiToken.trim()) {
      toast.error('Shortcut API token is required');
      return;
    }
    const scanId = typeof crypto !== 'undefined' && 'randomUUID' in crypto
      ? crypto.randomUUID()
      : `${Date.now()}-${Math.random().toString(36).slice(2)}`;
    scanIdRef.current = scanId;
    setScanProgress({ message: 'Starting Shortcut scan', phase: 'starting', processed: 0, total: 0 });
    setPreviewLoading(true);
    setPreview(null);
    const { data, error } = await pmImportService.previewShortcutAPI(workspaceId, apiToken.trim(), importOptions, scanId);
    setPreviewLoading(false);
    if (error || !data) {
      toast.error(error || 'Failed to preview Shortcut API import');
      setScanProgress(null);
      return;
    }
    setPreview(data);
    setTeamMappings(buildInitialTeamMappings(data, existingTeams));
    const wfRes = await pmWorkflowService.list(workspaceId);
    const helpinWorkflows = wfRes.data || [];
    setExistingWorkflows(helpinWorkflows);
    setWorkflowMappings(buildInitialWorkflowMappings(data, helpinWorkflows));
    setUserMappings(
      data.users.map((u) => ({
        email: u.email,
        storyCount: 0,
        matchedUserId: u.matched_user_id,
        matchedName: u.matched_name,
        shortcutName: u.shortcut_name || null,
        action: u.matched_user_id ? ('matched' as const) : ('skip' as const),
        manualUserId: null,
        invited: false,
      })),
    );
    setScanProgress(null);
  }, [apiToken, workspaceId, importOptions, existingTeams]);

  // ─── Step 1: Team helpers ────────────────────────────────────────

  const updateTeamMapping = useCallback((idx: number, updates: Partial<TeamMapping>) => {
    setTeamMappings((prev) => prev.map((m, i) => (i === idx ? { ...m, ...updates } : m)));
  }, []);

  const teamsValid = useMemo(() => {
    return teamMappings.every((team) => {
      if (team.mode === 'create_new') return team.newTeamName.trim() !== '';
      return team.existingTeamId.trim() !== '';
    });
  }, [teamMappings]);

  // ─── Step 2: Workflow helpers ────────────────────────────────────

  const updateWorkflowMapping = useCallback((idx: number, updates: Partial<WorkflowMapping>) => {
    setWorkflowMappings((prev) => prev.map((m, i) => (i === idx ? { ...m, ...updates } : m)));
  }, []);

  const updateWorkflowState = useCallback(
    (wfIdx: number, stateIdx: number, updates: Partial<WorkflowMapping['states'][0]>) => {
      setWorkflowMappings((prev) =>
        prev.map((m, i) =>
          i === wfIdx
            ? {
                ...m,
                states: m.states.map((s, j) => (j === stateIdx ? { ...s, ...updates } : s)),
              }
            : m,
        ),
      );
    },
    [],
  );

  const workflowsValid = useMemo(() => {
    return workflowMappings.every((wf) => {
      return wf.existingWorkflowId !== '' && wf.states.every((s) => s.existingStateId !== '');
    });
  }, [workflowMappings]);

  // ─── Step 2: User helpers ────────────────────────────────────────

  const updateUserMapping = useCallback((idx: number, updates: Partial<UserMapping>) => {
    setUserMappings((prev) => prev.map((m, i) => (i === idx ? { ...m, ...updates } : m)));
  }, []);

  const matchedCount = useMemo(
    () => userMappings.filter((u) => u.action === 'matched').length,
    [userMappings],
  );

  const unmatchedUsers = useMemo(
    () => userMappings.filter((u) => u.action !== 'matched' && !u.invited),
    [userMappings],
  );

  const handleInviteAll = useCallback(async () => {
    const toInvite = userMappings.filter((u) => u.action !== 'matched' && !u.invited);
    const results = await Promise.allSettled(
      toInvite.map((u) =>
        inviteService.send({ workspace_id: workspaceId, email: u.email, role: 'member' }),
      ),
    );
    let succeeded = 0;
    let autoMatched = 0;
    setUserMappings((prev) =>
      prev.map((u) => {
        const idx = toInvite.findIndex((t) => t.email === u.email);
        if (idx === -1) return u;
        const r = results[idx];
        if (r.status === 'fulfilled' && r.value.data) {
          succeeded++;
          return { ...u, action: 'invite' as const, invited: true };
        }
        // If invite failed, check if already a member and auto-match
        const existingMember = members.find(
          (m) => m.email.toLowerCase() === u.email.toLowerCase(),
        );
        if (existingMember) {
          autoMatched++;
          return {
            ...u,
            action: 'matched' as const,
            manualUserId: existingMember.user_id,
            matchedUserId: existingMember.user_id,
            matchedName: existingMember.full_name || null,
          };
        }
        // If pending invitation exists, mark as invited
        const errMsg = r.status === 'fulfilled' && r.value.error ? String(r.value.error) : '';
        if (errMsg.toLowerCase().includes('pending invitation')) {
          succeeded++;
          return { ...u, action: 'invite' as const, invited: true };
        }
        return u;
      }),
    );
    const parts = [];
    if (succeeded > 0) parts.push(`${succeeded} invited`);
    if (autoMatched > 0) parts.push(`${autoMatched} auto-matched`);
    if (parts.length > 0) toast.success(parts.join(', '));
  }, [userMappings, workspaceId, members]);

  const handleInviteSingle = useCallback(
    async (email: string) => {
      const { data, error } = await inviteService.send({
        workspace_id: workspaceId,
        email,
        role: 'member',
      });
      if (error || !data) {
        // If already a member, auto-match them
        const existingMember = members.find(
          (m) => m.email.toLowerCase() === email.toLowerCase(),
        );
        if (existingMember) {
          setUserMappings((prev) =>
            prev.map((u) =>
              u.email === email
                ? {
                    ...u,
                    action: 'matched' as const,
                    manualUserId: existingMember.user_id,
                    matchedUserId: existingMember.user_id,
                    matchedName: existingMember.full_name || null,
                  }
                : u,
            ),
          );
          toast.success(`${email} is already a member — auto-matched`);
          return;
        }
        // If a pending invitation already exists, mark as invited
        if (error && error.toLowerCase().includes('pending invitation')) {
          setUserMappings((prev) =>
            prev.map((u) =>
              u.email === email ? { ...u, action: 'invite' as const, invited: true } : u,
            ),
          );
          toast.success(`${email} already has a pending invitation — mapped`);
          return;
        }
        toast.error(error || 'Failed to send invitation');
        return;
      }
      setUserMappings((prev) =>
        prev.map((u) => (u.email === email ? { ...u, action: 'invite', invited: true } : u)),
      );
      toast.success(`Invitation sent to ${email}`);
    },
    [workspaceId, members],
  );

  // ─── Step 3: Execute ─────────────────────────────────────────────

  const handleStartImport = useCallback(async () => {
    if (!apiToken.trim()) return;
    setImporting(true);

    // Build user mappings: email → userId
    const userMap: Record<string, string> = {};
    for (const u of userMappings) {
      if (u.action === 'matched' && u.matchedUserId) userMap[u.email] = u.matchedUserId;
      else if (u.action === 'matched' && u.manualUserId) userMap[u.email] = u.manualUserId;
    }

    const teamMap: Record<string, string> = {};
    for (const team of teamMappings) {
      teamMap[team.shortcutTeamName] =
        team.mode === 'use_existing'
          ? `existing:${team.existingTeamId}`
          : `create:${team.newTeamName.trim() || team.shortcutTeamName}`;
    }

    // Build workflow state mappings
    const wfMappings: WorkflowStateMappingPayload[] = workflowMappings.map((wf) => ({
        shortcut_workflow_id: wf.shortcutWorkflowId || undefined,
        shortcut_workflow_name: wf.shortcutWorkflowName,
        mode: 'use_existing',
        existing_workflow_id: wf.existingWorkflowId,
        states: wf.states.map((s) => ({
          shortcut_state: s.shortcutState,
          existing_state_id: s.existingStateId,
        })),
      }));

    const { data, error } = await pmImportService.executeShortcutAPI(
      workspaceId,
      apiToken.trim(),
      userMap,
      teamMap,
      wfMappings,
      importOptions,
    );

    if (error || !data) {
      toast.error(error || 'Failed to start import');
      setImporting(false);
      return;
    }

    // Start polling
    pollRef.current = setInterval(async () => {
      const { data: status } = await pmImportService.getShortcutStatus(workspaceId, data.import_id);
      if (status) {
        setImportStatus(status);
        if (status.status === 'completed' || status.status === 'failed') {
          if (pollRef.current) clearInterval(pollRef.current);
          pollRef.current = null;
          setImporting(false);
        }
      }
    }, 1500);
  }, [userMappings, teamMappings, workflowMappings, workspaceId, apiToken, importOptions]);

  // ─── Navigation ──────────────────────────────────────────────────

  const canProceed = useMemo(() => {
    switch (step) {
      case 0: return !!preview;
      case 1: return teamsValid;
      case 2: return workflowsValid;
      case 3: return true;
      case 4: return false;
    }
  }, [step, preview, teamsValid, workflowsValid]);

  const goNext = () => setStep((s) => Math.min(s + 1, 4) as WizardStep);
  const goBack = () => setStep((s) => Math.max(s - 1, 0) as WizardStep);

  // ─── Render ──────────────────────────────────────────────────────

  return (
    <div className="space-y-6">
      {/* Step indicator */}
      <div className="flex items-center justify-center gap-2">
        {STEP_LABELS.map((label, i) => (
          <div key={label} className="flex items-center gap-2">
            {i > 0 && <div className="h-px w-8 bg-border" />}
            <div className="flex items-center gap-1.5">
              <div
                className={cn(
                  'flex h-6 w-6 items-center justify-center rounded-full text-xs font-medium',
                  i < step
                    ? 'bg-primary text-primary-foreground'
                    : i === step
                      ? 'bg-primary text-primary-foreground'
                      : 'bg-muted text-muted-foreground',
                )}
              >
                {i < step ? <Tick01Icon className="h-3.5 w-3.5" /> : i + 1}
              </div>
              <span
                className={cn(
                  'text-sm',
                  i === step ? 'font-medium' : 'text-muted-foreground',
                )}
              >
                {label}
              </span>
            </div>
          </div>
        ))}
      </div>

      {/* Step content */}
      {step === 0 && (
        <UploadStep
          preview={preview}
          loading={previewLoading}
          scanProgress={scanProgress}
          apiToken={apiToken}
          onApiTokenChange={(token) => {
            setApiToken(token);
          }}
          importArchived={importArchived}
          importCompleted={importCompleted}
          storyDateField={storyDateField}
          storyLookbackMonths={storyLookbackMonths}
          epicLookbackMonths={epicLookbackMonths}
          objectiveLookbackMonths={objectiveLookbackMonths}
          maxStories={maxStories}
          onArchived={setImportArchived}
          onCompleted={setImportCompleted}
          onStoryDateField={setStoryDateField}
          onStoryLookbackMonths={setStoryLookbackMonths}
          onEpicLookbackMonths={setEpicLookbackMonths}
          onObjectiveLookbackMonths={setObjectiveLookbackMonths}
          onMaxStories={setMaxStories}
          onAPIPreview={handleAPIPreview}
          onClear={() => {
            setPreview(null);
            setTeamMappings([]);
            setWorkflowMappings([]);
            setUserMappings([]);
            setImportStatus(null);
          }}
        />
      )}
      {step === 1 && (
        <TeamStep
          teamMappings={teamMappings}
          existingTeams={existingTeams}
          onUpdate={updateTeamMapping}
        />
      )}
      {step === 2 && (
        <WorkflowStep
          workflowMappings={workflowMappings}
          existingWorkflows={existingWorkflows}
          onUpdateMapping={updateWorkflowMapping}
          onUpdateState={updateWorkflowState}
        />
      )}
      {step === 3 && (
        <UserStep
          userMappings={userMappings}
          members={members}
          matchedCount={matchedCount}
          unmatchedCount={unmatchedUsers.length}
          onUpdate={updateUserMapping}
          onInviteAll={handleInviteAll}
          onInviteSingle={handleInviteSingle}
        />
      )}
      {step === 4 && (
        <ImportStep
          preview={preview}
          teamMappings={teamMappings}
          workflowMappings={workflowMappings}
          userMappings={userMappings}
          importArchived={importArchived}
          importCompleted={importCompleted}
          onArchived={setImportArchived}
          onCompleted={setImportCompleted}
          importing={importing}
          importStatus={importStatus}
          onStart={handleStartImport}
          hasApiToken={!!apiToken}
        />
      )}

      {/* Navigation */}
      {!importing && importStatus?.status !== 'completed' && importStatus?.status !== 'failed' && (
        <div className="flex justify-end gap-2 pt-2">
          {step > 0 && (
            <Button variant="outline" onClick={goBack}>
              Back
            </Button>
          )}
          {step < 4 && (
            <Button onClick={goNext} disabled={!canProceed}>
              Next: {STEP_LABELS[step + 1]}
            </Button>
          )}
        </div>
      )}
    </div>
  );
}

// ─── Step 0: API Preview ─────────────────────────────────────────────

function UploadStep({
  preview,
  loading,
  scanProgress,
  apiToken,
  onApiTokenChange,
  importArchived,
  importCompleted,
  storyDateField,
  storyLookbackMonths,
  epicLookbackMonths,
  objectiveLookbackMonths,
  maxStories,
  onArchived,
  onCompleted,
  onStoryDateField,
  onStoryLookbackMonths,
  onEpicLookbackMonths,
  onObjectiveLookbackMonths,
  onMaxStories,
  onAPIPreview,
  onClear,
}: {
  preview: ShortcutImportPreviewResponse | null;
  loading: boolean;
  scanProgress: ShortcutScanProgress | null;
  apiToken: string;
  onApiTokenChange: (token: string) => void;
  importArchived: boolean;
  importCompleted: boolean;
  storyDateField: 'updated_at' | 'created_at';
  storyLookbackMonths: string;
  epicLookbackMonths: string;
  objectiveLookbackMonths: string;
  maxStories: string;
  onArchived: (v: boolean) => void;
  onCompleted: (v: boolean) => void;
  onStoryDateField: (v: 'updated_at' | 'created_at') => void;
  onStoryLookbackMonths: (v: string) => void;
  onEpicLookbackMonths: (v: string) => void;
  onObjectiveLookbackMonths: (v: string) => void;
  onMaxStories: (v: string) => void;
  onAPIPreview: () => void;
  onClear: () => void;
}) {
  if (loading) {
    const hasCount = scanProgress && scanProgress.total > 0;
    return (
      <div className="flex flex-col items-center justify-center gap-3 py-16">
        <Loading01Icon className="h-8 w-8 animate-spin text-muted-foreground" />
        <div className="space-y-1 text-center">
          <p className="text-sm text-muted-foreground">
            {scanProgress?.message || 'Scanning Shortcut workspace...'}
          </p>
          {hasCount && (
            <p className="text-xs text-muted-foreground">
              {scanProgress.processed.toLocaleString()} of {scanProgress.total.toLocaleString()} stories
            </p>
          )}
        </div>
      </div>
    );
  }

  if (!preview) {
    return (
      <div className="space-y-4">
        <Card>
          <CardContent className="px-4 py-3">
            <div className="flex items-start gap-3">
              <Key01Icon className="h-4 w-4 text-muted-foreground mt-0.5 shrink-0" />
              <div className="flex-1 space-y-2">
                <div>
                  <Label className="text-sm font-medium">Shortcut API Token</Label>
                  <p className="text-xs text-muted-foreground mt-0.5">
                    Generate a token in Shortcut under Settings &rarr; API Tokens.
                  </p>
                </div>
                <div className="flex max-w-xl gap-2">
                  <Input
                    type="password"
                    placeholder="Shortcut API token"
                    value={apiToken}
                    onChange={(e) => onApiTokenChange(e.target.value)}
                    autoComplete="off"
                    data-1p-ignore
                    data-lpignore="true"
                    className="font-mono text-xs"
                  />
                  <Button type="button" onClick={onAPIPreview} disabled={!apiToken.trim()}>
                    Connect
                  </Button>
                </div>
                <div className="grid max-w-3xl gap-3 pt-2 sm:grid-cols-2 lg:grid-cols-4">
                  <div className="space-y-1.5">
                    <Label className="text-xs text-muted-foreground">Story date</Label>
                    <Select value={storyDateField} onValueChange={(v) => onStoryDateField(v as 'updated_at' | 'created_at')}>
                      <SelectTrigger className="h-8">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="updated_at">Updated</SelectItem>
                        <SelectItem value="created_at">Created</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>
                  <div className="space-y-1.5">
                    <Label className="text-xs text-muted-foreground">Story window</Label>
                    <Select value={storyLookbackMonths} onValueChange={onStoryLookbackMonths}>
                      <SelectTrigger className="h-8">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        {LOOKBACK_OPTIONS.map((option) => (
                          <SelectItem key={option.value} value={option.value}>
                            {option.label}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                  <div className="space-y-1.5">
                    <Label className="text-xs text-muted-foreground">Epic window</Label>
                    <Select value={epicLookbackMonths} onValueChange={onEpicLookbackMonths}>
                      <SelectTrigger className="h-8">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        {LOOKBACK_OPTIONS.map((option) => (
                          <SelectItem key={option.value} value={option.value}>
                            {option.label}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                  <div className="space-y-1.5">
                    <Label className="text-xs text-muted-foreground">Max stories</Label>
                    <Input
                      inputMode="numeric"
                      min={1}
                      placeholder="No limit"
                      value={maxStories}
                      onChange={(e) => onMaxStories(e.target.value.replace(/\D/g, ''))}
                      className="h-8"
                    />
                  </div>
                </div>
                <div className="grid max-w-xl gap-2 pt-1 sm:grid-cols-2">
                  <label className="flex items-center justify-between rounded-md border px-3 py-2">
                    <span className="text-xs">Archived tasks</span>
                    <Switch checked={importArchived} onCheckedChange={onArchived} />
                  </label>
                  <label className="flex items-center justify-between rounded-md border px-3 py-2">
                    <span className="text-xs">Completed tasks</span>
                    <Switch checked={importCompleted} onCheckedChange={onCompleted} />
                  </label>
                </div>
                <div className="max-w-[11rem] space-y-1.5">
                  <Label className="text-xs text-muted-foreground">Objective window</Label>
                  <Select value={objectiveLookbackMonths} onValueChange={onObjectiveLookbackMonths}>
                    <SelectTrigger className="h-8">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {LOOKBACK_OPTIONS.map((option) => (
                        <SelectItem key={option.value} value={option.value}>
                          {option.label}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>
    );
  }

  const s = preview.summary;
  const statCards = [
    { label: 'Tasks', value: s.total_tasks },
    { label: 'Epics', value: s.epics_count },
    { label: 'Objectives', value: s.objectives_count },
    { label: 'Sprints', value: s.sprints_count },
    { label: 'Labels', value: s.labels_count },
    { label: 'Teams', value: s.teams_count },
    { label: 'Workflows', value: s.workflows_count },
    { label: 'Checklists', value: s.checklist_items_count },
  ];

  const storyTypes = Object.entries(s.tasks_by_type).sort(([, a], [, b]) => b - a);
  const totalTasks = storyTypes.reduce((sum, [, v]) => sum + v, 0) || 1;

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Tick01Icon className="h-4 w-4 text-green-500" />
          <span className="text-sm font-medium truncate max-w-[300px]">
            Shortcut API connected
          </span>
        </div>
        <Button variant="ghost" size="sm" onClick={onClear}>
          Reset
        </Button>
      </div>

      <div className="grid grid-cols-4 gap-3">
        {statCards.map((c) => (
          <Card key={c.label} className="py-3">
            <CardContent className="px-4 py-0 text-center">
              <p className="text-2xl font-bold">{c.value.toLocaleString()}</p>
              <p className="text-xs text-muted-foreground">{c.label}</p>
            </CardContent>
          </Card>
        ))}
      </div>

      <div className="space-y-2">
        <p className="text-sm font-medium">Tasks by Type</p>
        <div className="flex h-3 w-1/2 overflow-hidden rounded-full">
          {storyTypes.map(([type, count]) => (
            <div
              key={type}
              className={cn('h-full', TASK_TYPE_CONFIG[type as TaskType]?.swatch || 'bg-muted-foreground')}
              style={{ width: `${(count / totalTasks) * 100}%` }}
              title={`${type}: ${count.toLocaleString()}`}
            />
          ))}
        </div>
        <div className="flex gap-4">
          {storyTypes.map(([type, count]) => (
            <div key={type} className="flex items-center gap-1.5">
              {TASK_TYPE_CONFIG[type as TaskType] ? (
                <TaskTypeIcon taskType={type as TaskType} className="h-4 w-4 shrink-0" />
              ) : (
                <div className="h-2.5 w-2.5 rounded-full bg-muted-foreground" />
              )}
              <span className="text-xs text-muted-foreground">
                {type} <span className="font-medium text-foreground">{count.toLocaleString()}</span>
              </span>
            </div>
          ))}
        </div>
      </div>

      {preview.warnings && preview.warnings.length > 0 && (
        <div className="space-y-1">
          {preview.warnings.map((w, i) => (
            <div key={i} className="flex items-start gap-2 text-xs text-yellow-600 dark:text-yellow-400">
              <Alert01Icon className="mt-0.5 h-3 w-3 shrink-0" />
              <span>{w}</span>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

// ─── Step 1: Team Mapping ────────────────────────────────────────────

function TeamStep({
  teamMappings,
  existingTeams,
  onUpdate,
}: {
  teamMappings: TeamMapping[];
  existingTeams: { id: string; name: string }[];
  onUpdate: (idx: number, updates: Partial<TeamMapping>) => void;
}) {
  if (teamMappings.length === 0) {
    return (
      <Card>
        <CardContent className="px-4 py-6">
          <p className="text-sm text-muted-foreground">
            No Shortcut teams or groups were detected. Imported stories will not be assigned to a team.
          </p>
        </CardContent>
      </Card>
    );
  }

  return (
    <div className="space-y-4">
      <div>
        <h3 className="text-base font-semibold">Choose Destination Teams</h3>
        <p className="text-sm text-muted-foreground">
          Each Shortcut team or group can create a new Helpin team or map into an existing one.
          New workflows will be created per destination team.
        </p>
      </div>

      <Card>
        <CardContent className="px-4 py-3">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="text-xs">Shortcut Team</TableHead>
                <TableHead className="text-xs">Destination</TableHead>
                <TableHead className="text-xs">Team</TableHead>
                <TableHead className="text-xs text-right">Stories</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {teamMappings.map((team, idx) => (
                <TableRow key={team.shortcutTeamName}>
                  <TableCell className="py-2 text-sm font-medium">{team.shortcutTeamName}</TableCell>
                  <TableCell className="py-2">
                    <Select
                      value={team.mode}
                      onValueChange={(value) => onUpdate(idx, { mode: value as TeamMapping['mode'] })}
                    >
                      <SelectTrigger className="h-8 w-36">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="create_new">Create new</SelectItem>
                        <SelectItem value="use_existing">Use existing</SelectItem>
                      </SelectContent>
                    </Select>
                  </TableCell>
                  <TableCell className="py-2">
                    {team.mode === 'create_new' ? (
                      <Input
                        value={team.newTeamName}
                        onChange={(e) => onUpdate(idx, { newTeamName: e.target.value })}
                        className="h-8"
                      />
                    ) : (
                      <Select
                        value={team.existingTeamId}
                        onValueChange={(value) => onUpdate(idx, { existingTeamId: value })}
                      >
                        <SelectTrigger className="h-8">
                          <SelectValue placeholder="Select team" />
                        </SelectTrigger>
                        <SelectContent>
                          {existingTeams.map((existing) => (
                            <SelectItem key={existing.id} value={existing.id}>
                              {existing.name}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    )}
                  </TableCell>
                  <TableCell className="py-2 text-right text-sm text-muted-foreground">
                    {team.taskCount.toLocaleString()}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </div>
  );
}

// ─── Step 2: Workflow Mapping ────────────────────────────────────────

function WorkflowStep({
  workflowMappings,
  existingWorkflows,
  onUpdateMapping,
  onUpdateState,
}: {
  workflowMappings: WorkflowMapping[];
  existingWorkflows: WorkflowWithStates[];
  onUpdateMapping: (idx: number, updates: Partial<WorkflowMapping>) => void;
  onUpdateState: (wfIdx: number, stateIdx: number, updates: Partial<WorkflowMapping['states'][0]>) => void;
}) {
  const allReady = workflowMappings.every((wf) => {
    return wf.existingWorkflowId !== '' && wf.states.every((s) => s.existingStateId !== '');
  });

  return (
    <div className="space-y-4">
      <div>
        <p className="text-sm text-muted-foreground">
          Map each Shortcut workflow into an existing Helpin workflow, then map Shortcut states to
          Helpin states.
        </p>
      </div>
      {existingWorkflows.length === 0 && (
        <Card>
          <CardContent className="px-4 py-3">
            <p className="text-sm text-muted-foreground">
              No Helpin workflows are available. Create the target workflow before starting the import.
            </p>
          </CardContent>
        </Card>
      )}

      {workflowMappings.map((wf, wfIdx) => {
        const isValid = wf.existingWorkflowId !== '' && wf.states.every((s) => s.existingStateId !== '');
        const totalTasks = wf.states.reduce((sum, s) => sum + s.storyCount, 0);
        const selectedWf = existingWorkflows.find((ew) => ew.workflow.id === wf.existingWorkflowId);
        const stateStats = workflowStateMappingStats(wf.states);

        return (
          <Card key={wf.shortcutWorkflowId || wf.shortcutWorkflowName}>
            <CardHeader
              className="cursor-pointer py-3 px-4"
              onClick={() => onUpdateMapping(wfIdx, { expanded: !wf.expanded })}
            >
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  {wf.expanded ? (
                    <ArrowDown01Icon className="h-4 w-4" />
                  ) : (
                    <ArrowRight01Icon className="h-4 w-4" />
                  )}
                  <CardTitle className="text-sm">{wf.shortcutWorkflowName}</CardTitle>
                  {isValid && <Badge variant="outline" className="text-green-600 border-green-300 text-xs">Ready</Badge>}
                </div>
                <span className="text-xs text-muted-foreground">
                  {totalTasks.toLocaleString()} tasks
                </span>
              </div>
            </CardHeader>

            {wf.expanded && (
              <CardContent className="space-y-4 px-4 pb-4 pt-0">
                <>
                  <div className="grid gap-3 md:grid-cols-[minmax(220px,320px)_1fr]">
                      <div className="space-y-1.5">
                        <Label className="text-xs">Existing workflow</Label>
                        <Select
                          value={wf.existingWorkflowId}
                          onValueChange={(v) => {
                            const nextWorkflow = existingWorkflows.find((ew) => ew.workflow.id === v);
                            onUpdateMapping(wfIdx, {
                              existingWorkflowId: v,
                              states: nextWorkflow
                                ? autoMapExistingWorkflowStates(wf.states, nextWorkflow.states)
                                : wf.states,
                            });
                          }}
                        >
                          <SelectTrigger className="h-8 text-sm">
                            <SelectValue placeholder="Select a workflow..." />
                          </SelectTrigger>
                          <SelectContent>
                            {existingWorkflows.map((ew) => (
                              <SelectItem key={ew.workflow.id} value={ew.workflow.id}>
                                {ew.workflow.name}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                      </div>
                      <div className="flex items-end justify-between gap-3">
                        <div className="text-xs text-muted-foreground">
                          {wf.existingWorkflowId
                            ? `${stateStats.mapped} of ${stateStats.total} states mapped`
                            : 'Select a workflow to map Shortcut states'}
                        </div>
                        {wf.existingWorkflowId && (
                          <div className="flex gap-2">
                            <Button
                              type="button"
                              variant="outline"
                              size="sm"
                              className="h-8 text-xs"
                              onClick={() => {
                                if (!selectedWf) return;
                                onUpdateMapping(wfIdx, {
                                  states: autoMapExistingWorkflowStates(wf.states, selectedWf.states),
                                });
                              }}
                            >
                              Auto-map
                            </Button>
                            <Button
                              type="button"
                              variant="ghost"
                              size="sm"
                              className="h-8 text-xs"
                              onClick={() =>
                                onUpdateMapping(wfIdx, {
                                  states: wf.states.map((state) => ({ ...state, existingStateId: '' })),
                                })
                              }
                            >
                          Clear
                        </Button>
                      </div>
                    )}
                  </div>
                  </div>

                  {wf.existingWorkflowId && (
                    <div className="rounded-md border">
                      <div className="hidden grid-cols-[minmax(0,1fr)_88px_minmax(220px,280px)] gap-3 border-b px-3 py-2 text-xs text-muted-foreground md:grid">
                        <div>Shortcut state</div>
                        <div className="text-right">Tasks</div>
                        <div>Helpin state</div>
                      </div>
                      <div className="divide-y">
                        {wf.states.map((s, sIdx) => (
                          <div
                            key={s.shortcutState}
                            className={cn(
                              'grid grid-cols-1 gap-2 px-3 py-2 md:grid-cols-[minmax(0,1fr)_88px_minmax(220px,280px)] md:items-center md:gap-3',
                              !s.existingStateId && 'bg-destructive/5',
                            )}
                          >
                            <div className="min-w-0">
                              <div className="truncate text-sm font-medium">{s.shortcutState}</div>
                              {!s.existingStateId && (
                                <div className="mt-0.5 text-xs text-destructive">Needs mapping</div>
                              )}
                            </div>
                            <div className="text-xs text-muted-foreground md:text-right">
                              {s.storyCount.toLocaleString()}
                            </div>
                            <Select
                              value={s.existingStateId}
                              onValueChange={(v) => onUpdateState(wfIdx, sIdx, { existingStateId: v })}
                            >
                              <SelectTrigger className="h-8 text-xs">
                                <SelectValue placeholder="Select state" />
                              </SelectTrigger>
                              <SelectContent>
                                {(selectedWf?.states || []).map((es) => (
                                  <SelectItem key={es.id} value={es.id}>
                                    {es.name}
                                  </SelectItem>
                                ))}
                              </SelectContent>
                            </Select>
                          </div>
                        ))}
                      </div>
                    </div>
                  )}
                </>
              </CardContent>
            )}
          </Card>
        );
      })}

      {allReady && (
        <p className="text-xs text-green-600 dark:text-green-400 flex items-center gap-1.5">
          <Tick01Icon className="h-3.5 w-3.5" />
          All workflows are ready. You can continue or expand to customize.
        </p>
      )}
    </div>
  );
}

// ─── Step 2: User Mapping ────────────────────────────────────────────

function UserStep({
  userMappings,
  members,
  matchedCount,
  unmatchedCount,
  onUpdate,
  onInviteAll,
  onInviteSingle,
}: {
  userMappings: UserMapping[];
  members: MemberWithUser[];
  matchedCount: number;
  unmatchedCount: number;
  onUpdate: (idx: number, updates: Partial<UserMapping>) => void;
  onInviteAll: () => void;
  onInviteSingle: (email: string) => void;
}) {
  const matched = userMappings.filter((u) => u.action === 'matched');
  const unmatched = userMappings.filter((u) => u.action !== 'matched');

  return (
    <div className="space-y-4">
      <div>
        <p className="text-sm text-muted-foreground">
          Match Shortcut users to Helpin workspace members. You can invite unmatched users so their
          tasks are properly assigned.
        </p>
      </div>

      <div className="flex items-center justify-between">
        <p className="text-sm">
          <span className="font-medium">{matchedCount}</span> of{' '}
          <span className="font-medium">{userMappings.length}</span> users auto-matched
        </p>
        {unmatchedCount > 0 && (
          <Button variant="outline" size="sm" onClick={onInviteAll}>
            <Mail01Icon className="mr-1.5 h-3.5 w-3.5" />
            Invite All Unmatched ({unmatchedCount})
          </Button>
        )}
      </div>

      {matched.length > 0 && (
        <div className="space-y-2">
          <p className="text-xs font-medium text-muted-foreground">Matched Members</p>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="text-xs">Shortcut User</TableHead>
                <TableHead className="text-xs">Helpin Member</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {matched.map((u) => (
                <TableRow key={u.email}>
                  <TableCell className="py-1.5">
                    <div className="flex items-center gap-1.5 text-sm">
                      <Tick01Icon className="h-3.5 w-3.5 text-green-500" />
                      <div>
                        {u.shortcutName && <span className="font-medium">{u.shortcutName} — </span>}
                        {u.email}
                      </div>
                    </div>
                  </TableCell>
                  <TableCell className="py-1.5 text-sm">{u.matchedName || '—'}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}

      {unmatched.length > 0 && (
        <div className="space-y-2">
          <p className="text-xs font-medium text-muted-foreground">Unmatched Users ({unmatched.length})</p>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="text-xs">Shortcut User</TableHead>
                <TableHead className="text-xs w-[220px]">Action</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {unmatched.map((u) => {
                const realIdx = userMappings.findIndex((m) => m.email === u.email);
                return (
                  <TableRow key={u.email}>
                    <TableCell className="py-1.5">
                      <div className="flex items-center gap-1.5 text-sm">
                        {u.invited ? (
                          <Mail01Icon className="h-3.5 w-3.5 text-blue-500" />
                        ) : (
                          <Alert01Icon className="h-3.5 w-3.5 text-yellow-500" />
                        )}
                        <div>
                          {u.shortcutName && <span className="font-medium">{u.shortcutName} — </span>}
                          {u.email}
                        </div>
                        {u.invited && (
                          <Badge variant="outline" className="text-xs text-blue-600 border-blue-300">
                            Invited
                          </Badge>
                        )}
                      </div>
                    </TableCell>
                    <TableCell className="py-1.5">
                      {u.invited ? (
                        <span className="text-xs text-muted-foreground">Pending acceptance</span>
                      ) : (
                        <div className="flex items-center gap-1">
                          <Select
                            value={u.manualUserId || '__action__' + u.action}
                            onValueChange={(v) => {
                              if (v === '__action__skip') {
                                onUpdate(realIdx, { action: 'skip', manualUserId: null });
                              } else if (v === '__action__invite') {
                                onInviteSingle(u.email);
                              } else {
                                const member = members.find((m) => m.user_id === v);
                                onUpdate(realIdx, {
                                  action: 'matched',
                                  manualUserId: v,
                                  matchedUserId: v,
                                  matchedName: member?.full_name || null,
                                });
                              }
                            }}
                          >
                            <SelectTrigger className="h-7 text-xs">
                              <SelectValue placeholder="Select..." />
                            </SelectTrigger>
                            <SelectContent>
                              {members.map((m) => (
                                <SelectItem key={m.user_id} value={m.user_id}>
                                  {m.full_name} ({m.email})
                                </SelectItem>
                              ))}
                              <SelectItem value="__action__invite">
                                <span className="flex items-center gap-1">
                                  <Mail01Icon className="h-3 w-3" /> Invite &amp; Map
                                </span>
                              </SelectItem>
                              <SelectItem value="__action__skip">
                                <span className="flex items-center gap-1">
                                  <Cancel01Icon className="h-3 w-3" /> Skip
                                </span>
                              </SelectItem>
                            </SelectContent>
                          </Select>
                        </div>
                      )}
                    </TableCell>
                  </TableRow>
                );
              })}
            </TableBody>
          </Table>

          {unmatched.some((u) => u.invited) && (
            <p className="text-xs text-muted-foreground">
              Invitations have been sent. Tasks will be assigned to these users. They'll see their
              work once they accept the invite and sign in.
            </p>
          )}
        </div>
      )}
    </div>
  );
}

// ─── Step 3: Configure & Import ──────────────────────────────────────

function ImportStep({
  preview,
  teamMappings,
  workflowMappings,
  userMappings,
  importArchived,
  importCompleted,
  onArchived,
  onCompleted,
  importing,
  importStatus,
  onStart,
  hasApiToken,
}: {
  preview: ShortcutImportPreviewResponse | null;
  teamMappings: TeamMapping[];
  workflowMappings: WorkflowMapping[];
  userMappings: UserMapping[];
  importArchived: boolean;
  importCompleted: boolean;
  onArchived: (v: boolean) => void;
  onCompleted: (v: boolean) => void;
  importing: boolean;
  importStatus: ShortcutImportStatusResponse | null;
  onStart: () => void;
  hasApiToken: boolean;
}) {
  const s = preview?.summary;
  const isDone = importStatus?.status === 'completed';
  const isFailed = importStatus?.status === 'failed';
  const isRunning = importing || importStatus?.status === 'processing';

  const matchedUsers = userMappings.filter((u) => u.action === 'matched').length;
  const invitedUsers = userMappings.filter((u) => u.invited).length;
  const skippedUsers = userMappings.filter((u) => u.action === 'skip' && !u.invited).length;
  const newTeams = teamMappings.filter((t) => t.mode === 'create_new').length;
  const existingTeams = teamMappings.filter((t) => t.mode === 'use_existing').length;

  // ─── Running / Done / Failed ────────────────────────────────────

  if (isRunning || isDone || isFailed) {
    const progress = importStatus?.progress;
    const pct = progress
      ? progress.steps_total > 0
        ? Math.round((progress.steps_completed / progress.steps_total) * 100)
        : 0
      : 0;

    return (
      <div className="space-y-4">
        <h3 className="text-base font-semibold">
          {isDone ? 'Import Complete' : isFailed ? 'Import Failed' : 'Importing...'}
        </h3>

        <div className="space-y-2">
          <Progress value={isDone ? 100 : pct} className="h-3" />
          <p className="text-xs text-muted-foreground">
            {isDone
              ? 'All steps completed successfully.'
              : isFailed
                ? `Failed at step — ${formatImportStepLabel(progress?.current_step)}`
                : `Step ${progress?.steps_completed || 0} of ${progress?.steps_total || 0} — ${formatImportStepLabel(progress?.current_step)}`}
          </p>
        </div>

        {isRunning && progress && (
          <div className="space-y-1">
            {IMPORT_STEPS_API.map((stepLabel, i) => {
              const done = i < (progress.steps_completed || 0);
              const active =
                i === (progress.steps_completed || 0) &&
                importStatus?.status === 'processing';
              return (
                <div
                  key={stepLabel}
                  className={cn(
                    'flex items-center gap-2 text-xs',
                    done ? 'text-green-600 dark:text-green-400' : active ? '' : 'text-muted-foreground',
                  )}
                >
                  {done ? (
                    <Tick01Icon className="h-3.5 w-3.5" />
                  ) : active ? (
                    <Loading01Icon className="h-3.5 w-3.5 animate-spin" />
                  ) : (
                    <div className="h-3.5 w-3.5 rounded-full border" />
                  )}
                  {stepLabel}
                </div>
              );
            })}
          </div>
        )}

        {isRunning && (
          <p className="text-xs text-muted-foreground">
            Do not close this window while the import is in progress.
          </p>
        )}

        {isDone && importStatus?.result && (
          <ResultTable result={importStatus.result} />
        )}

        {isFailed && importStatus?.error && (
          <Card className="border-destructive">
            <CardContent className="px-4 py-3">
              <p className="text-sm text-destructive">{importStatus.error}</p>
              <p className="mt-1 text-xs text-muted-foreground">
                All changes have been rolled back. No data was imported.
              </p>
            </CardContent>
          </Card>
        )}
      </div>
    );
  }

  // ─── Pre-import config ──────────────────────────────────────────

  return (
    <div className="space-y-4">
      <h3 className="text-base font-semibold">Review &amp; Import</h3>

      <Card>
        <CardHeader className="py-3 px-4">
          <CardTitle className="text-sm">Options</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3 px-4 pb-4 pt-0">
          <div className="flex items-center justify-between">
            <Label className="text-sm">Import archived tasks</Label>
            <Switch checked={importArchived} onCheckedChange={onArchived} disabled={hasApiToken} />
          </div>
          <div className="flex items-center justify-between">
            <Label className="text-sm">Import completed tasks</Label>
            <Switch checked={importCompleted} onCheckedChange={onCompleted} disabled={hasApiToken} />
          </div>
        </CardContent>
      </Card>

      {s && (
        <Card>
          <CardHeader className="py-3 px-4">
            <CardTitle className="text-sm">Summary</CardTitle>
          </CardHeader>
          <CardContent className="px-4 pb-4 pt-0">
            <Table>
              <TableBody>
                <SummaryRow label="Teams" value={`${teamMappings.length} (${newTeams} new, ${existingTeams} existing)`} />
                <SummaryRow
                  label="Workflows"
                  value={`${workflowMappings.length} existing workflows selected`}
                />
                <SummaryRow
                  label="Workflow States"
                  value={`${workflowMappings.reduce((sum, workflow) => sum + workflow.states.length, 0)} mapped`}
                />
                <SummaryRow label="Labels" value={`${s.labels_count}`} />
                <SummaryRow label="Objectives" value={`${s.objectives_count}`} />
                <SummaryRow label="Epics" value={`${s.epics_count}`} />
                <SummaryRow label="Sprints" value={`${s.sprints_count}`} />
                <SummaryRow label="Tasks" value={`${s.total_tasks.toLocaleString()}`} />
                <SummaryRow label="Checklist Items" value={`${s.checklist_items_count}`} />
                <SummaryRow
                  label="User Mappings"
                  value={`${matchedUsers} matched${invitedUsers > 0 ? `, ${invitedUsers} invited` : ''}${skippedUsers > 0 ? `, ${skippedUsers} skipped` : ''}`}
                />
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      )}

      {preview && preview.warnings && preview.warnings.length > 0 && (
        <Card>
          <CardHeader className="py-3 px-4">
            <CardTitle className="text-sm">Warnings</CardTitle>
          </CardHeader>
          <CardContent className="space-y-1.5 px-4 pb-4 pt-0">
            {preview.warnings.map((w, i) => (
              <div key={i} className="flex items-start gap-2 text-xs text-yellow-600 dark:text-yellow-400">
                <Alert01Icon className="mt-0.5 h-3 w-3 shrink-0" />
                <span>{w}</span>
              </div>
            ))}
          </CardContent>
        </Card>
      )}

      <div className="flex justify-end">
        <Button onClick={onStart}>Start Import</Button>
      </div>
    </div>
  );
}

function SummaryRow({ label, value }: { label: string; value: string }) {
  return (
    <TableRow>
      <TableCell className="py-1.5 text-sm">{label}</TableCell>
      <TableCell className="py-1.5 text-sm text-right">{value}</TableCell>
    </TableRow>
  );
}

function ResultTable({ result }: { result: ShortcutImportStatusResponse['result'] }) {
  if (!result) return null;
  const rows = [
    { label: 'Teams', created: result.teams_created },
    { label: 'Workflows', created: result.workflows_created },
    { label: 'Workflow States', created: result.workflow_states_created },
    { label: 'Labels', created: result.labels_created },
    { label: 'Objectives', created: result.objectives_created },
    { label: 'Epics', created: result.epics_created },
    { label: 'Sprints', created: result.sprints_created },
    { label: 'Tasks', created: result.tasks_created, skipped: result.tasks_skipped },
    { label: 'Checklist Items', created: result.checklist_items_created },
    { label: 'Owner Links', created: result.owner_links_created },
    { label: 'Label Links', created: result.label_links_created },
    { label: 'External Links', created: result.external_links_created },
    { label: 'Task Links', created: result.task_links_created },
    { label: 'Attachments', created: result.attachments_created },
    { label: 'Comments', created: result.comments_created },
  ];

  return (
    <div className="space-y-3">
      <Card>
        <CardHeader className="py-3 px-4">
          <CardTitle className="text-sm">Results</CardTitle>
        </CardHeader>
        <CardContent className="px-4 pb-4 pt-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="text-xs">Entity</TableHead>
                <TableHead className="text-xs text-right">Created</TableHead>
                <TableHead className="text-xs text-right">Skipped</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.map((r) => (
                <TableRow key={r.label}>
                  <TableCell className="py-1.5 text-sm">{r.label}</TableCell>
                  <TableCell className="py-1.5 text-sm text-right">{r.created}</TableCell>
                  <TableCell className="py-1.5 text-sm text-right text-muted-foreground">
                    {'skipped' in r && r.skipped ? r.skipped : '—'}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      {result.warnings && result.warnings.length > 0 && (
        <Card>
          <CardHeader className="py-3 px-4">
            <CardTitle className="text-sm">Warnings ({result.warnings.length})</CardTitle>
          </CardHeader>
          <CardContent className="space-y-1.5 px-4 pb-4 pt-0">
            {result.warnings.map((w, i) => (
              <div key={i} className="flex items-start gap-2 text-xs text-yellow-600 dark:text-yellow-400">
                <Alert01Icon className="mt-0.5 h-3 w-3 shrink-0" />
                <span>{w}</span>
              </div>
            ))}
          </CardContent>
        </Card>
      )}
    </div>
  );
}
