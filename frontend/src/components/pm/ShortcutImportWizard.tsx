import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
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
  Bookmark01Icon,
  Target02Icon,
  StickyNote01Icon,
  Layers01Icon,
  ArrowReloadHorizontalIcon,
  type IconComponent,
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
import { useDocsCollections, useDocsSpaces } from '@/hooks/queries/useDocs';

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

const IMPORT_STEPS_API = ['API Enrichment', 'Teams', 'Workflows', 'Labels', 'Objectives', 'Epics', 'Sprints', 'Tasks', 'Media', 'Comments', 'Docs'];
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
    case 'docs':
      return 'Docs';
    case 'parse':
      return 'Preparing import';
    case 'completed':
      return 'Completed';
    default:
      return step || 'unknown';
  }
}

function formatImportStatusLabel(status: ShortcutImportStatusResponse['status']) {
  switch (status) {
    case 'pending':
      return 'Pending';
    case 'scanning':
      return 'Scanning';
    case 'ready':
      return 'Ready';
    case 'processing':
      return 'Processing';
    case 'completed':
      return 'Completed';
    case 'failed':
      return 'Failed';
    case 'canceled':
      return 'Canceled';
    default:
      return status;
  }
}

function formatImportDate(value?: string | null) {
  if (!value) return '—';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '—';
  return new Intl.DateTimeFormat(undefined, {
    month: 'short',
    day: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
  }).format(date);
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

// ─── Layout helpers ──────────────────────────────────────────────────

function SectionHeader({
  number,
  title,
  description,
  action,
}: {
  number: number;
  title: string;
  description?: string;
  action?: ReactNode;
}) {
  return (
    <div className="flex items-start gap-3">
      <div className="mt-0.5 flex h-6 w-6 shrink-0 items-center justify-center rounded-full border bg-background text-xs font-medium text-foreground">
        {number}
      </div>
      <div className="min-w-0 flex-1">
        <div className="flex items-center justify-between gap-2">
          <h3 className="text-sm font-semibold leading-6">{title}</h3>
          {action}
        </div>
        {description && (
          <p className="mt-0.5 text-xs text-muted-foreground">{description}</p>
        )}
      </div>
    </div>
  );
}

function SectionBody({ children, className }: { children: ReactNode; className?: string }) {
  return <div className={cn('pl-9', className)}>{children}</div>;
}

function FieldLabel({ children }: { children: ReactNode }) {
  return <Label className="text-[11px] font-medium uppercase tracking-wide text-muted-foreground">{children}</Label>;
}

function ToggleCard({
  icon: Icon,
  title,
  description,
  active,
  disabled,
  onClick,
}: {
  icon: IconComponent;
  title: string;
  description: string;
  active: boolean;
  disabled?: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      disabled={disabled}
      onClick={onClick}
      className={cn(
        'group relative flex flex-col items-start gap-2 rounded-md border bg-background p-3 text-left transition',
        active
          ? 'border-foreground ring-1 ring-foreground'
          : 'border-border hover:border-foreground/40',
        disabled && 'cursor-not-allowed opacity-60',
      )}
    >
      <div
        className={cn(
          'flex h-7 w-7 items-center justify-center rounded-md',
          active ? 'bg-foreground text-background' : 'bg-muted text-foreground',
        )}
      >
        <Icon className="h-4 w-4" />
      </div>
      <div className="min-w-0 space-y-0.5">
        <div className="text-sm font-medium leading-tight">{title}</div>
        <div className="text-xs text-muted-foreground leading-snug">{description}</div>
      </div>
      <div
        className={cn(
          'absolute right-2 top-2 flex h-4 w-4 items-center justify-center rounded-full border',
          active ? 'border-foreground bg-foreground text-background' : 'border-border bg-background',
        )}
      >
        {active && <Tick01Icon className="h-3 w-3" />}
      </div>
    </button>
  );
}

// ─── Main Component ──────────────────────────────────────────────────

interface ShortcutImportWizardProps {
  workspaceId: string;
  members: MemberWithUser[];
}

export function ShortcutImportWizard({ workspaceId, members }: ShortcutImportWizardProps) {
  const { teams: existingTeams } = useWorkspaceTeams(workspaceId);
  const { data: docsSpaces = [], isFetched: docsSpacesFetched } = useDocsSpaces(workspaceId);
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
  const [importDocs, setImportDocs] = useState(true);
  const [docsSpaceId, setDocsSpaceId] = useState('');
  const [docsCollectionId, setDocsCollectionId] = useState('');
  const [docsLookbackMonths, setDocsLookbackMonths] = useState('0');
  const [scanProgress, setScanProgress] = useState<ShortcutScanProgress | null>(null);
  const [apiToken, setApiToken] = useState('');
  const [importStatus, setImportStatus] = useState<ShortcutImportStatusResponse | null>(null);
  const [importHistory, setImportHistory] = useState<ShortcutImportStatusResponse[]>([]);
  const [historyLoading, setHistoryLoading] = useState(false);
  const [importing, setImporting] = useState(false);
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const scanIdRef = useRef<string | null>(null);
  const { data: docsCollections = [] } = useDocsCollections(workspaceId, docsSpaceId);

  // Cleanup polling on unmount
  useEffect(() => {
    return () => {
      if (pollRef.current) clearInterval(pollRef.current);
    };
  }, []);

  const loadImportHistory = useCallback(async () => {
    setHistoryLoading(true);
    const { data } = await pmImportService.listShortcutStatuses(workspaceId);
    setHistoryLoading(false);
    if (data) {
      setImportHistory(data);
    }
  }, [workspaceId]);

  useEffect(() => {
    void loadImportHistory();
  }, [loadImportHistory]);

  useEffect(() => {
    if (!docsSpaceId && docsSpaces.length > 0) {
      setDocsSpaceId(docsSpaces[0].id);
    }
    if (docsSpacesFetched && docsSpaces.length === 0 && importDocs) {
      setImportDocs(false);
    }
  }, [docsSpaceId, docsSpaces, docsSpacesFetched, importDocs]);

  useEffect(() => {
    if (docsCollectionId && !docsCollections.some((collection) => collection.id === docsCollectionId)) {
      setDocsCollectionId('');
    }
  }, [docsCollectionId, docsCollections]);

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
      import_docs: importDocs,
      docs_space_id: importDocs ? docsSpaceId : undefined,
      docs_collection_id: importDocs && docsCollectionId ? docsCollectionId : undefined,
      docs_lookback_months: importDocs ? parsePositiveInt(docsLookbackMonths) : undefined,
    };
  }, [
    importArchived,
    importCompleted,
    storyDateField,
    storyLookbackMonths,
    epicLookbackMonths,
    objectiveLookbackMonths,
    maxStories,
    importDocs,
    docsSpaceId,
    docsCollectionId,
    docsLookbackMonths,
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

  const startImportStatusPolling = useCallback((importId: string) => {
    if (pollRef.current) clearInterval(pollRef.current);
    const poll = async () => {
      const { data: status } = await pmImportService.getShortcutStatus(workspaceId, importId);
      if (status) {
        setImportStatus(status);
        if (status.status === 'completed' || status.status === 'failed' || status.status === 'canceled') {
          if (pollRef.current) clearInterval(pollRef.current);
          pollRef.current = null;
          setImporting(false);
          void loadImportHistory();
        }
      }
    };
    setImporting(true);
    void poll();
    pollRef.current = setInterval(poll, 1500);
  }, [loadImportHistory, workspaceId]);

  const handleStartImport = useCallback(async () => {
    if (!apiToken.trim()) return;
    if (importDocs && !docsSpaceId) {
      toast.error('Select a Helpin Docs space for Shortcut Docs');
      return;
    }
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
    await loadImportHistory();
    startImportStatusPolling(data.import_id);
  }, [userMappings, teamMappings, workflowMappings, workspaceId, apiToken, importOptions, importDocs, docsSpaceId, loadImportHistory, startImportStatusPolling]);

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

  const handleSelectHistory = (status: ShortcutImportStatusResponse) => {
    if (pollRef.current) clearInterval(pollRef.current);
    pollRef.current = null;
    setImporting(false);
    setImportStatus(status);
    setStep(4);
    if (status.status === 'pending' || status.status === 'scanning' || status.status === 'processing') {
      startImportStatusPolling(status.import_id);
    }
  };

  const showHistoryTable = step === 0 && !preview && !previewLoading;

  return (
    <div className="space-y-4">
    <Card className="overflow-hidden p-0">
      {/* Step indicator */}
      <div className="flex items-center gap-1.5 border-b px-5 py-3 text-xs">
        {STEP_LABELS.map((label, i) => {
          const isComplete = i < step;
          const isActive = i === step;
          return (
            <div key={label} className="flex items-center gap-1.5">
              {i > 0 && <div className={cn('h-px w-6', isComplete || isActive ? 'bg-foreground/40' : 'bg-border')} />}
              <div className="flex items-center gap-1.5">
                <div
                  className={cn(
                    'flex h-5 w-5 items-center justify-center rounded-full border text-[11px] font-medium',
                    isComplete && 'border-foreground bg-foreground text-background',
                    isActive && !isComplete && 'border-foreground bg-background text-foreground',
                    !isComplete && !isActive && 'border-border bg-background text-muted-foreground',
                  )}
                >
                  {isComplete ? <Tick01Icon className="h-3 w-3" /> : i + 1}
                </div>
                <span
                  className={cn(
                    isActive ? 'font-medium text-foreground' : 'text-muted-foreground',
                  )}
                >
                  {label}
                </span>
              </div>
            </div>
          );
        })}
      </div>

      {/* Step content */}
      <div className="px-5 py-5">
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
          importDocs={importDocs}
          docsSpaceId={docsSpaceId}
          docsCollectionId={docsCollectionId}
          docsLookbackMonths={docsLookbackMonths}
          docsSpaces={docsSpaces}
          docsCollections={docsCollections}
          onArchived={setImportArchived}
          onCompleted={setImportCompleted}
          onStoryDateField={setStoryDateField}
          onStoryLookbackMonths={setStoryLookbackMonths}
          onEpicLookbackMonths={setEpicLookbackMonths}
          onObjectiveLookbackMonths={setObjectiveLookbackMonths}
          onMaxStories={setMaxStories}
          onImportDocs={setImportDocs}
          onDocsSpaceId={(value) => {
            setDocsSpaceId(value);
            setDocsCollectionId('');
          }}
          onDocsCollectionId={setDocsCollectionId}
          onDocsLookbackMonths={setDocsLookbackMonths}
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
          importDocs={importDocs}
          docsSpaceName={docsSpaces.find((space) => space.id === docsSpaceId)?.name || ''}
          docsCollectionName={docsCollections.find((collection) => collection.id === docsCollectionId)?.name || ''}
          onArchived={setImportArchived}
          onCompleted={setImportCompleted}
          importing={importing}
          importStatus={importStatus}
          onStart={handleStartImport}
          hasApiToken={!!apiToken}
        />
      )}

      </div>

      {/* Navigation */}
      {!importing && importStatus?.status !== 'completed' && importStatus?.status !== 'failed' && (
        <div className="flex justify-end gap-2 border-t bg-muted/20 px-5 py-3">
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
    </Card>

    {showHistoryTable && (
      <ShortcutImportHistoryTable
        imports={importHistory}
        loading={historyLoading}
        onRefresh={loadImportHistory}
        onSelect={handleSelectHistory}
      />
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
  importDocs,
  docsSpaceId,
  docsCollectionId,
  docsLookbackMonths,
  docsSpaces,
  docsCollections,
  onArchived,
  onCompleted,
  onStoryDateField,
  onStoryLookbackMonths,
  onEpicLookbackMonths,
  onObjectiveLookbackMonths,
  onMaxStories,
  onImportDocs,
  onDocsSpaceId,
  onDocsCollectionId,
  onDocsLookbackMonths,
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
  importDocs: boolean;
  docsSpaceId: string;
  docsCollectionId: string;
  docsLookbackMonths: string;
  docsSpaces: { id: string; name: string }[];
  docsCollections: { id: string; name: string }[];
  onArchived: (v: boolean) => void;
  onCompleted: (v: boolean) => void;
  onStoryDateField: (v: 'updated_at' | 'created_at') => void;
  onStoryLookbackMonths: (v: string) => void;
  onEpicLookbackMonths: (v: string) => void;
  onObjectiveLookbackMonths: (v: string) => void;
  onMaxStories: (v: string) => void;
  onImportDocs: (v: boolean) => void;
  onDocsSpaceId: (v: string) => void;
  onDocsCollectionId: (v: string) => void;
  onDocsLookbackMonths: (v: string) => void;
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
    const docsAvailable = docsSpaces.length > 0;
    const resetFilters = () => {
      onStoryDateField('updated_at');
      onStoryLookbackMonths('6');
      onEpicLookbackMonths('12');
      onObjectiveLookbackMonths('12');
      onMaxStories('');
      onArchived(false);
      onCompleted(true);
    };

    return (
      <div className="space-y-8">
        {/* ─── Section 1: Connect ─────────────────────────────── */}
        <section className="space-y-3">
          <SectionHeader
            number={1}
            title="Connect to Shortcut"
            description="Enter your Shortcut API token to connect to your workspace."
          />
          <SectionBody className="space-y-3">
            <div className="flex max-w-xl gap-2">
              <div className="relative flex-1">
                <Key01Icon className="pointer-events-none absolute left-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-muted-foreground" />
                <Input
                  type="password"
                  placeholder="Shortcut API token"
                  value={apiToken}
                  onChange={(e) => onApiTokenChange(e.target.value)}
                  autoComplete="off"
                  data-1p-ignore
                  data-lpignore="true"
                  className="h-9 pl-8 font-mono text-xs"
                />
              </div>
              <Button type="button" onClick={onAPIPreview} disabled={!apiToken.trim()} className="h-9">
                Connect
              </Button>
            </div>
            <p className="text-xs text-muted-foreground">
              Find your token in Shortcut under{' '}
              <span className="font-medium text-foreground">Settings → API Tokens</span>.
            </p>
            <div className="flex items-center gap-2 rounded-md border bg-muted/40 px-3 py-2">
              <div className="flex h-5 w-5 items-center justify-center rounded-full bg-background">
                <div className="h-2 w-2 rounded-full bg-muted-foreground" />
              </div>
              <span className="text-xs text-muted-foreground">
                Not connected — enter a token and click Connect to get started.
              </span>
            </div>
          </SectionBody>
        </section>

        {/* ─── Section 2: What to import ──────────────────────── */}
        <section className="space-y-3">
          <SectionHeader
            number={2}
            title="What do you want to import?"
            description="Enter your token and click Connect to get started."
          />
          <SectionBody>
            <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
              <ToggleCard
                icon={Bookmark01Icon}
                title="Stories"
                description="Tasks are story items"
                active
                onClick={() => undefined}
              />
              <ToggleCard
                icon={Layers01Icon}
                title="Epics"
                description="Epics and larger initiatives"
                active
                onClick={() => undefined}
              />
              <ToggleCard
                icon={Target02Icon}
                title="Objectives"
                description="Objectives and key results"
                active
                onClick={() => undefined}
              />
              <ToggleCard
                icon={StickyNote01Icon}
                title="Docs"
                description="Documentation and notes"
                active={importDocs}
                disabled={!docsAvailable}
                onClick={() => onImportDocs(!importDocs)}
              />
            </div>
            <p className="mt-2 text-xs text-muted-foreground">
              You can import any combination of these.
            </p>
          </SectionBody>
        </section>

        {/* ─── Section 3: Filters ─────────────────────────────── */}
        <section className="space-y-3">
          <SectionHeader
            number={3}
            title="Filters"
            description="Refine the results to import."
            action={
              <Button
                type="button"
                variant="ghost"
                size="sm"
                onClick={resetFilters}
                className="h-7 text-xs text-muted-foreground"
              >
                <ArrowReloadHorizontalIcon className="mr-1 h-3 w-3" />
                Reset to defaults
              </Button>
            }
          />
          <SectionBody className="space-y-3">
            <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
              <div className="space-y-1.5">
                <FieldLabel>Date</FieldLabel>
                <Select value={storyDateField} onValueChange={(v) => onStoryDateField(v as 'updated_at' | 'created_at')}>
                  <SelectTrigger className="h-9">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="updated_at">Updated</SelectItem>
                    <SelectItem value="created_at">Created</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-1.5">
                <FieldLabel>Story window</FieldLabel>
                <Select value={storyLookbackMonths} onValueChange={onStoryLookbackMonths}>
                  <SelectTrigger className="h-9">
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
                <FieldLabel>Epic window</FieldLabel>
                <Select value={epicLookbackMonths} onValueChange={onEpicLookbackMonths}>
                  <SelectTrigger className="h-9">
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
                <FieldLabel>Objective window</FieldLabel>
                <Select value={objectiveLookbackMonths} onValueChange={onObjectiveLookbackMonths}>
                  <SelectTrigger className="h-9">
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
            <div className="grid gap-3 sm:grid-cols-3">
              <label className="flex items-center justify-between rounded-md border bg-background px-3 py-2">
                <span className="text-xs">Include archived tasks</span>
                <Switch checked={importArchived} onCheckedChange={onArchived} />
              </label>
              <label className="flex items-center justify-between rounded-md border bg-background px-3 py-2">
                <span className="text-xs">Include completed tasks</span>
                <Switch checked={importCompleted} onCheckedChange={onCompleted} />
              </label>
              <div className="space-y-1.5">
                <FieldLabel>Limit results</FieldLabel>
                <Input
                  inputMode="numeric"
                  min={1}
                  placeholder="No limit"
                  value={maxStories}
                  onChange={(e) => onMaxStories(e.target.value.replace(/\D/g, ''))}
                  className="h-9"
                />
              </div>
            </div>
          </SectionBody>
        </section>

        {/* ─── Section 4: Docs destination ──────────────────────── */}
        {importDocs && (
          <section className="space-y-3">
            <SectionHeader
              number={4}
              title="Docs destination"
              description="Choose where the imported docs will be created."
            />
            <SectionBody>
              <div className="grid gap-3 sm:grid-cols-3">
                <div className="space-y-1.5">
                  <FieldLabel>Docs space</FieldLabel>
                  <Select value={docsSpaceId || 'none'} onValueChange={onDocsSpaceId} disabled={!docsAvailable}>
                    <SelectTrigger className="h-9">
                      <SelectValue placeholder="Select space" />
                    </SelectTrigger>
                    <SelectContent>
                      {docsSpaces.length === 0 ? (
                        <SelectItem value="none" disabled>No spaces</SelectItem>
                      ) : (
                        docsSpaces.map((space) => (
                          <SelectItem key={space.id} value={space.id}>
                            {space.name}
                          </SelectItem>
                        ))
                      )}
                    </SelectContent>
                  </Select>
                </div>
                <div className="space-y-1.5">
                  <FieldLabel>Collection</FieldLabel>
                  <Select
                    value={docsCollectionId || 'root'}
                    onValueChange={(value) => onDocsCollectionId(value === 'root' ? '' : value)}
                    disabled={!docsSpaceId}
                  >
                    <SelectTrigger className="h-9">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="root">Uncategorized</SelectItem>
                      {docsCollections.map((collection) => (
                        <SelectItem key={collection.id} value={collection.id}>
                          {collection.name}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <div className="space-y-1.5">
                  <FieldLabel>Docs window</FieldLabel>
                  <Select value={docsLookbackMonths} onValueChange={onDocsLookbackMonths}>
                    <SelectTrigger className="h-9">
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
            </SectionBody>
          </section>
        )}
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
    { label: 'Docs', value: s.docs_count },
    { label: 'Teams', value: s.teams_count },
    { label: 'Workflows', value: s.workflows_count },
    { label: 'Checklists', value: s.checklist_items_count },
  ];

  const storyTypes = Object.entries(s.tasks_by_type).sort(([, a], [, b]) => b - a);
  const totalTasks = storyTypes.reduce((sum, [, v]) => sum + v, 0) || 1;

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between rounded-md border bg-emerald-50/60 px-3 py-2 dark:bg-emerald-950/20">
        <div className="flex items-center gap-2">
          <div className="flex h-5 w-5 items-center justify-center rounded-full bg-emerald-500/15">
            <Tick01Icon className="h-3.5 w-3.5 text-emerald-600 dark:text-emerald-400" />
          </div>
          <span className="text-sm font-medium">Shortcut API connected</span>
          <span className="text-xs text-muted-foreground">— preview ready</span>
        </div>
        <Button variant="ghost" size="sm" onClick={onClear} className="h-7 text-xs">
          Reset
        </Button>
      </div>

      <div className="grid grid-cols-3 gap-2 sm:grid-cols-5 lg:grid-cols-9">
        {statCards.map((c) => (
          <div
            key={c.label}
            className="rounded-md border bg-background px-3 py-2 text-center"
          >
            <p className="text-lg font-semibold leading-tight">{c.value.toLocaleString()}</p>
            <p className="mt-0.5 text-[11px] uppercase tracking-wide text-muted-foreground">{c.label}</p>
          </div>
        ))}
      </div>

      <div className="space-y-2">
        <p className="text-[11px] font-medium uppercase tracking-wide text-muted-foreground">Tasks by type</p>
        <div className="flex h-2 w-full max-w-md overflow-hidden rounded-full bg-muted">
          {storyTypes.map(([type, count]) => (
            <div
              key={type}
              className={cn('h-full', TASK_TYPE_CONFIG[type as TaskType]?.swatch || 'bg-muted-foreground')}
              style={{ width: `${(count / totalTasks) * 100}%` }}
              title={`${type}: ${count.toLocaleString()}`}
            />
          ))}
        </div>
        <div className="flex flex-wrap gap-x-4 gap-y-1.5">
          {storyTypes.map(([type, count]) => (
            <div key={type} className="flex items-center gap-1.5">
              {TASK_TYPE_CONFIG[type as TaskType] ? (
                <TaskTypeIcon taskType={type as TaskType} className="h-3.5 w-3.5 shrink-0" />
              ) : (
                <div className="h-2 w-2 rounded-full bg-muted-foreground" />
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
      <SectionHeader
        number={1}
        title="Choose destination teams"
        description="Each Shortcut team or group can create a new Helpin team or map into an existing one. New workflows will be created per destination team."
      />

      <Card className="ml-9">
        <CardContent className="px-4 py-3">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="text-xs">Shortcut team</TableHead>
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
      <SectionHeader
        number={1}
        title="Map workflows and states"
        description="Map each Shortcut workflow into an existing Helpin workflow, then map Shortcut states to Helpin states."
      />
      {existingWorkflows.length === 0 && (
        <Card className="ml-9">
          <CardContent className="px-4 py-3">
            <p className="text-sm text-muted-foreground">
              No Helpin workflows are available. Create the target workflow before starting the import.
            </p>
          </CardContent>
        </Card>
      )}

      <div className="ml-9 space-y-3">
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
      <SectionHeader
        number={1}
        title="Match users"
        description="Match Shortcut users to Helpin workspace members. You can invite unmatched users so their tasks are properly assigned."
        action={
          unmatchedCount > 0 ? (
            <Button variant="outline" size="sm" onClick={onInviteAll} className="h-7 text-xs">
              <Mail01Icon className="mr-1.5 h-3 w-3" />
              Invite all unmatched ({unmatchedCount})
            </Button>
          ) : undefined
        }
      />

      <div className="ml-9 space-y-4">
      <div className="rounded-md border bg-muted/40 px-3 py-2">
        <p className="text-xs">
          <span className="font-medium">{matchedCount}</span> of{' '}
          <span className="font-medium">{userMappings.length}</span> users auto-matched
        </p>
      </div>

      {matched.length > 0 && (
        <div className="space-y-2">
          <p className="text-[11px] font-medium uppercase tracking-wide text-muted-foreground">Matched members</p>
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
          <p className="text-[11px] font-medium uppercase tracking-wide text-muted-foreground">Unmatched users ({unmatched.length})</p>
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
    </div>
  );
}

function ShortcutImportHistoryTable({
  imports,
  loading,
  onRefresh,
  onSelect,
}: {
  imports: ShortcutImportStatusResponse[];
  loading: boolean;
  onRefresh: () => void;
  onSelect: (status: ShortcutImportStatusResponse) => void;
}) {
  const rows = imports.slice(0, 8);
  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between px-4 py-3">
        <CardTitle className="text-sm">Recent Shortcut imports</CardTitle>
        <Button type="button" variant="ghost" size="sm" onClick={onRefresh} disabled={loading}>
          {loading ? 'Refreshing' : 'Refresh'}
        </Button>
      </CardHeader>
      <CardContent className="px-4 pb-4 pt-0">
        {rows.length === 0 ? (
          <p className="text-sm text-muted-foreground">No Shortcut imports have been started yet.</p>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="text-xs">Started</TableHead>
                <TableHead className="text-xs">Status</TableHead>
                <TableHead className="text-xs text-right">Tasks</TableHead>
                <TableHead className="text-xs">Step</TableHead>
                <TableHead className="w-16 text-xs" />
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.map((row) => {
                const isFailed = row.status === 'failed';
                const isComplete = row.status === 'completed';
                const statusVariant = isFailed ? 'destructive' : isComplete ? 'secondary' : 'outline';
                return (
                  <TableRow key={row.import_id}>
                    <TableCell className="py-2 text-sm">{formatImportDate(row.created_at)}</TableCell>
                    <TableCell className="py-2">
                      <Badge variant={statusVariant}>{formatImportStatusLabel(row.status)}</Badge>
                    </TableCell>
                    <TableCell className="py-2 text-right text-sm">
                      {(row.result?.tasks_created ?? row.total_rows ?? row.progress.entities_total ?? 0).toLocaleString()}
                    </TableCell>
                    <TableCell className="py-2 text-sm text-muted-foreground">
                      {formatImportStepLabel(row.progress.current_step)}
                    </TableCell>
                    <TableCell className="py-2 text-right">
                      <Button type="button" variant="ghost" size="sm" onClick={() => onSelect(row)}>
                        View
                      </Button>
                    </TableCell>
                  </TableRow>
                );
              })}
            </TableBody>
          </Table>
        )}
      </CardContent>
    </Card>
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
  importDocs,
  docsSpaceName,
  docsCollectionName,
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
  importDocs: boolean;
  docsSpaceName: string;
  docsCollectionName: string;
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
  const isRunning = importing || importStatus?.status === 'pending' || importStatus?.status === 'scanning' || importStatus?.status === 'processing';

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
            You can leave this screen. The import status is saved and will remain available in history.
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
    <div className="space-y-6">
      <section className="space-y-3">
        <SectionHeader
          number={1}
          title="Options"
          description="Final overrides before kicking off the import."
        />
        <SectionBody>
          <div className="grid gap-2 sm:grid-cols-2">
            <label className="flex items-center justify-between rounded-md border bg-background px-3 py-2">
              <span className="text-xs">Import archived tasks</span>
              <Switch checked={importArchived} onCheckedChange={onArchived} disabled={hasApiToken} />
            </label>
            <label className="flex items-center justify-between rounded-md border bg-background px-3 py-2">
              <span className="text-xs">Import completed tasks</span>
              <Switch checked={importCompleted} onCheckedChange={onCompleted} disabled={hasApiToken} />
            </label>
          </div>
        </SectionBody>
      </section>

      {s && (
        <section className="space-y-3">
          <SectionHeader number={2} title="Summary" description="Review what will be imported." />
          <SectionBody>
            <div className="overflow-hidden rounded-md border">
              <Table>
                <TableBody>
                  <SummaryRow label="Teams" value={`${teamMappings.length} (${newTeams} new, ${existingTeams} existing)`} />
                  <SummaryRow
                    label="Workflows"
                    value={`${workflowMappings.length} existing workflows selected`}
                  />
                  <SummaryRow
                    label="Workflow states"
                    value={`${workflowMappings.reduce((sum, workflow) => sum + workflow.states.length, 0)} mapped`}
                  />
                  <SummaryRow label="Labels" value={`${s.labels_count}`} />
                  <SummaryRow label="Objectives" value={`${s.objectives_count}`} />
                  <SummaryRow label="Epics" value={`${s.epics_count}`} />
                  <SummaryRow label="Sprints" value={`${s.sprints_count}`} />
                  <SummaryRow label="Tasks" value={`${s.total_tasks.toLocaleString()}`} />
                  <SummaryRow
                    label="Docs"
                    value={
                      importDocs
                        ? `${s.docs_count.toLocaleString()} to ${docsCollectionName || docsSpaceName || 'selected space'}`
                        : 'Not selected'
                    }
                  />
                  <SummaryRow label="Checklist items" value={`${s.checklist_items_count}`} />
                  <SummaryRow
                    label="User mappings"
                    value={`${matchedUsers} matched${invitedUsers > 0 ? `, ${invitedUsers} invited` : ''}${skippedUsers > 0 ? `, ${skippedUsers} skipped` : ''}`}
                  />
                </TableBody>
              </Table>
            </div>
          </SectionBody>
        </section>
      )}

      {preview && preview.warnings && preview.warnings.length > 0 && (
        <section className="space-y-3">
          <SectionHeader number={3} title="Warnings" description="Resolve these before importing if possible." />
          <SectionBody>
            <div className="space-y-1.5 rounded-md border border-yellow-300/50 bg-yellow-50/60 px-3 py-2 dark:border-yellow-900/50 dark:bg-yellow-950/20">
              {preview.warnings.map((w, i) => (
                <div key={i} className="flex items-start gap-2 text-xs text-yellow-700 dark:text-yellow-400">
                  <Alert01Icon className="mt-0.5 h-3 w-3 shrink-0" />
                  <span>{w}</span>
                </div>
              ))}
            </div>
          </SectionBody>
        </section>
      )}

      <div className="flex items-center justify-between border-t pt-4">
        <p className="text-xs text-muted-foreground">
          The import runs in the background. You can leave this page and check progress in History.
        </p>
        <Button onClick={onStart}>Start import</Button>
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
    { label: 'Docs', created: result.docs_created, skipped: result.docs_skipped },
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
