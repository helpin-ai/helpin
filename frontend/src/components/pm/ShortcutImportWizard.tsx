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
  AlertTriangle,
  Check,
  ChevronDown,
  ChevronRight,
  GripVertical,
  Key,
  Loader2,
  Mail,
  Upload,
  X,
} from 'lucide-react';
import {
  DndContext,
  closestCenter,
  KeyboardSensor,
  PointerSensor,
  useSensor,
  useSensors,
  type DragEndEvent,
} from '@dnd-kit/core';
import {
  SortableContext,
  sortableKeyboardCoordinates,
  useSortable,
  verticalListSortingStrategy,
} from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import { toast } from 'sonner';
import {
  pmImportService,
  type ShortcutImportPreviewResponse,
  type ShortcutImportStatusResponse,
  type WorkflowStateMappingPayload,
} from '@/lib/services/pmImportService';
import type { WorkflowWithStates } from '@/lib/pmTypes';
import type { MemberWithUser } from '@/lib/types';
import { pmWorkflowService } from '@/lib/services/pmWorkflowService';
import { inviteService } from '@/lib/services/inviteService';

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

type WizardStep = 0 | 1 | 2 | 3;

const STEP_LABELS = ['Upload', 'Workflows', 'Users', 'Import'];

const STATE_TYPE_OPTIONS: { value: StateType; label: string; color: string }[] = [
  { value: 'backlog', label: 'Backlog', color: 'bg-gray-400' },
  { value: 'unstarted', label: 'Unstarted', color: 'bg-blue-400' },
  { value: 'started', label: 'Started', color: 'bg-yellow-400' },
  { value: 'done', label: 'Done', color: 'bg-green-400' },
];

const IMPORT_STEPS_BASE = ['Teams', 'Workflows', 'Labels', 'Objectives', 'Epics', 'Sprints', 'Tasks', 'Links'];
const IMPORT_STEPS_API = ['API Enrichment', 'Teams', 'Workflows', 'Labels', 'Objectives', 'Epics', 'Sprints', 'Tasks', 'Links', 'Comments'];

// ─── Main Component ──────────────────────────────────────────────────

interface ShortcutImportWizardProps {
  workspaceId: string;
  members: MemberWithUser[];
}

export function ShortcutImportWizard({ workspaceId, members }: ShortcutImportWizardProps) {
  const [step, setStep] = useState<WizardStep>(0);
  const [file, setFile] = useState<File | null>(null);
  const [preview, setPreview] = useState<ShortcutImportPreviewResponse | null>(null);
  const [previewLoading, setPreviewLoading] = useState(false);
  const [workflowMappings, setWorkflowMappings] = useState<WorkflowMapping[]>([]);
  const [userMappings, setUserMappings] = useState<UserMapping[]>([]);
  const [existingWorkflows, setExistingWorkflows] = useState<WorkflowWithStates[]>([]);
  const [importArchived, setImportArchived] = useState(true);
  const [importCompleted, setImportCompleted] = useState(true);
  const [apiToken, setApiToken] = useState('');
  const [importStatus, setImportStatus] = useState<ShortcutImportStatusResponse | null>(null);
  const [importing, setImporting] = useState(false);
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);

  // Cleanup polling on unmount
  useEffect(() => {
    return () => {
      if (pollRef.current) clearInterval(pollRef.current);
    };
  }, []);

  // ─── Step 0: Upload ──────────────────────────────────────────────

  const handleFileSelect = useCallback(async (selected: File, token?: string) => {
    setFile(selected);
    setPreviewLoading(true);
    setPreview(null);
    const tokenToUse = token ?? apiToken;
    const { data, error } = await pmImportService.previewShortcut(workspaceId, selected, tokenToUse || undefined);
    setPreviewLoading(false);
    if (error || !data) {
      toast.error(error || 'Failed to preview CSV');
      setFile(null);
      return;
    }
    setPreview(data);

    // Initialize workflow mappings (backend returns states in logical order)
    setWorkflowMappings(
      data.workflows.map((wf) => ({
        shortcutWorkflowId: wf.id || '',
        shortcutWorkflowName: wf.name,
        mode: 'create_new' as const,
        newWorkflowName: wf.name,
        states: wf.states.map((s, i) => ({
          shortcutState: s.name,
          newStateName: s.name,
          stateType: (s.suggested_type || 'unstarted') as StateType,
          position: i,
          storyCount: s.task_count,
          existingStateId: '',
        })),
        existingWorkflowId: '',
        expanded: false,
      })),
    );

    // Build user story counts from preview
    const userStoryCount = new Map<string, number>();
    for (const u of data.users) {
      userStoryCount.set(u.email, (userStoryCount.get(u.email) || 0) + 1);
    }

    setUserMappings(
      data.users.map((u) => ({
        email: u.email,
        storyCount: 0, // preview doesn't give per-user counts yet
        matchedUserId: u.matched_user_id,
        matchedName: u.matched_name,
        shortcutName: u.shortcut_name || null,
        action: u.matched_user_id ? ('matched' as const) : ('skip' as const),
        manualUserId: null,
        invited: false,
      })),
    );

    // Load existing workflows for the "use existing" mode
    const wfRes = await pmWorkflowService.list(workspaceId);
    if (wfRes.data) setExistingWorkflows(wfRes.data);
  }, [workspaceId, apiToken]);

  const handleDrop = useCallback(
    (e: React.DragEvent) => {
      e.preventDefault();
      const f = e.dataTransfer.files[0];
      if (f && f.name.endsWith('.csv')) handleFileSelect(f);
      else toast.error('Please upload a .csv file');
    },
    [handleFileSelect],
  );

  const handleFileInput = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      const f = e.target.files?.[0];
      if (f) handleFileSelect(f);
    },
    [handleFileSelect],
  );

  // ─── Step 1: Workflow helpers ────────────────────────────────────

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
      if (wf.mode === 'create_new') {
        return wf.newWorkflowName.trim() !== '' && wf.states.some((s) => s.stateType === 'done');
      }
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
    if (!file) return;
    setImporting(true);

    // Build user mappings: email → userId
    const userMap: Record<string, string> = {};
    for (const u of userMappings) {
      if (u.action === 'matched' && u.matchedUserId) userMap[u.email] = u.matchedUserId;
      else if (u.action === 'matched' && u.manualUserId) userMap[u.email] = u.manualUserId;
    }

    // Build workflow state mappings
    const wfMappings: WorkflowStateMappingPayload[] = workflowMappings.map((wf) => {
      if (wf.mode === 'create_new') {
        return {
          shortcut_workflow_id: wf.shortcutWorkflowId || undefined,
          shortcut_workflow_name: wf.shortcutWorkflowName,
          mode: 'create_new',
          new_workflow_name: wf.newWorkflowName,
          states: wf.states.map((s) => ({
            shortcut_state: s.shortcutState,
            new_state_name: s.newStateName,
            state_type: s.stateType,
            position: s.position,
          })),
        };
      }
      return {
        shortcut_workflow_id: wf.shortcutWorkflowId || undefined,
        shortcut_workflow_name: wf.shortcutWorkflowName,
        mode: 'use_existing',
        existing_workflow_id: wf.existingWorkflowId,
        states: wf.states.map((s) => ({
          shortcut_state: s.shortcutState,
          existing_state_id: s.existingStateId,
        })),
      };
    });

    const { data, error } = await pmImportService.executeShortcut(
      workspaceId,
      file,
      userMap,
      wfMappings,
      { import_archived: importArchived, import_completed: importCompleted },
      apiToken || undefined,
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
  }, [file, userMappings, workflowMappings, importArchived, importCompleted, workspaceId, apiToken]);

  // ─── Navigation ──────────────────────────────────────────────────

  const canProceed = useMemo(() => {
    switch (step) {
      case 0: return !!preview;
      case 1: return workflowsValid;
      case 2: return true;
      case 3: return false;
    }
  }, [step, preview, workflowsValid]);

  const goNext = () => setStep((s) => Math.min(s + 1, 3) as WizardStep);
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
                {i < step ? <Check className="h-3.5 w-3.5" /> : i + 1}
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
          file={file}
          preview={preview}
          loading={previewLoading}
          apiToken={apiToken}
          onApiTokenChange={(token) => {
            setApiToken(token);
            // Re-preview if file already selected and token changes
            if (file && token !== apiToken) {
              handleFileSelect(file, token);
            }
          }}
          onDrop={handleDrop}
          onFileInput={handleFileInput}
          onClear={() => {
            setFile(null);
            setPreview(null);
          }}
        />
      )}
      {step === 1 && (
        <WorkflowStep
          workflowMappings={workflowMappings}
          existingWorkflows={existingWorkflows}
          onUpdateMapping={updateWorkflowMapping}
          onUpdateState={updateWorkflowState}
        />
      )}
      {step === 2 && (
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
      {step === 3 && (
        <ImportStep
          preview={preview}
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
          {step < 3 && (
            <Button onClick={goNext} disabled={!canProceed}>
              Next: {STEP_LABELS[step + 1]}
            </Button>
          )}
        </div>
      )}
    </div>
  );
}

// ─── Step 0: Upload & Preview ────────────────────────────────────────

function UploadStep({
  file,
  preview,
  loading,
  apiToken,
  onApiTokenChange,
  onDrop,
  onFileInput,
  onClear,
}: {
  file: File | null;
  preview: ShortcutImportPreviewResponse | null;
  loading: boolean;
  apiToken: string;
  onApiTokenChange: (token: string) => void;
  onDrop: (e: React.DragEvent) => void;
  onFileInput: (e: React.ChangeEvent<HTMLInputElement>) => void;
  onClear: () => void;
}) {
  const fileInputRef = useRef<HTMLInputElement>(null);

  if (loading) {
    return (
      <div className="flex flex-col items-center justify-center gap-3 py-16">
        <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
        <p className="text-sm text-muted-foreground">Analyzing CSV file...</p>
      </div>
    );
  }

  if (!file || !preview) {
    return (
      <div className="space-y-4">
        <p className="text-sm text-muted-foreground">
          In Shortcut, go to Settings &rarr; Export &rarr; CSV to download your workspace data.
        </p>

        {/* API Token (optional) */}
        <Card>
          <CardContent className="px-4 py-3">
            <div className="flex items-start gap-3">
              <Key className="h-4 w-4 text-muted-foreground mt-0.5 shrink-0" />
              <div className="flex-1 space-y-2">
                <div>
                  <Label className="text-sm font-medium">Shortcut API Token (optional)</Label>
                  <p className="text-xs text-muted-foreground mt-0.5">
                    Provides richer import: real label colors, sprint dates, epic/objective descriptions, and story comments.
                    Generate a token at Settings &rarr; API Tokens in Shortcut.
                  </p>
                </div>
                <Input
                  type="text"
                  placeholder="sc_..."
                  value={apiToken}
                  onChange={(e) => onApiTokenChange(e.target.value)}
                  autoComplete="off"
                  data-1p-ignore
                  data-lpignore="true"
                  className="max-w-sm font-mono text-xs"
                />
              </div>
            </div>
          </CardContent>
        </Card>

        <div
          onDragOver={(e) => e.preventDefault()}
          onDrop={onDrop}
          onClick={() => fileInputRef.current?.click()}
          className="flex cursor-pointer flex-col items-center justify-center gap-2 rounded-lg border-2 border-dashed border-border px-6 py-16 transition-colors hover:border-primary/50 hover:bg-muted/30"
        >
          <Upload className="h-8 w-8 text-muted-foreground" />
          <p className="text-sm font-medium">Drag and drop your CSV file here</p>
          <p className="text-xs text-muted-foreground">or click to browse — .csv up to 50 MB</p>
          <input
            ref={fileInputRef}
            type="file"
            accept=".csv"
            className="hidden"
            onChange={onFileInput}
          />
        </div>
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

  const STORY_TYPE_COLORS: Record<string, string> = {
    feature: 'bg-blue-500',
    bug: 'bg-red-400',
    chore: 'bg-amber-400',
  };

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Check className="h-4 w-4 text-green-500" />
          <span className="text-sm font-medium truncate max-w-[300px]">{file.name}</span>
        </div>
        <Button variant="ghost" size="sm" onClick={onClear}>
          Change File
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
              className={cn('h-full', STORY_TYPE_COLORS[type] || 'bg-gray-400')}
              style={{ width: `${(count / totalTasks) * 100}%` }}
              title={`${type}: ${count.toLocaleString()}`}
            />
          ))}
        </div>
        <div className="flex gap-4">
          {storyTypes.map(([type, count]) => (
            <div key={type} className="flex items-center gap-1.5">
              <div className={cn('h-2.5 w-2.5 rounded-full', STORY_TYPE_COLORS[type] || 'bg-gray-400')} />
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
              <AlertTriangle className="mt-0.5 h-3 w-3 shrink-0" />
              <span>{w}</span>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

// ─── Sortable State Row ──────────────────────────────────────────────

function SortableStateRow({
  state,
  onUpdateName,
  onUpdateType,
}: {
  state: WorkflowMapping['states'][0];
  onUpdateName: (name: string) => void;
  onUpdateType: (type: string) => void;
}) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({
    id: state.shortcutState,
  });

  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
  };

  return (
    <TableRow ref={setNodeRef} style={style} className={isDragging ? 'opacity-50 bg-accent/50' : ''}>
      <TableCell className="py-1.5 w-[40px]">
        <button
          type="button"
          className="h-6 w-6 flex items-center justify-center cursor-grab text-muted-foreground/50 hover:text-muted-foreground active:cursor-grabbing"
          {...attributes}
          {...listeners}
        >
          <GripVertical className="h-3.5 w-3.5" />
        </button>
      </TableCell>
      <TableCell className="py-1.5">
        <Input
          value={state.newStateName}
          onChange={(e) => onUpdateName(e.target.value)}
          className="h-7 text-sm"
        />
      </TableCell>
      <TableCell className="py-1.5 text-right text-xs text-muted-foreground">
        {state.storyCount.toLocaleString()}
      </TableCell>
      <TableCell className="py-1.5">
        <Select value={state.stateType} onValueChange={onUpdateType}>
          <SelectTrigger className="h-7 text-xs">
            <div className="flex items-center gap-1.5">
              <div
                className={cn(
                  'h-2 w-2 rounded-full',
                  STATE_TYPE_OPTIONS.find((o) => o.value === state.stateType)?.color,
                )}
              />
              <SelectValue />
            </div>
          </SelectTrigger>
          <SelectContent>
            {STATE_TYPE_OPTIONS.map((opt) => (
              <SelectItem key={opt.value} value={opt.value}>
                <div className="flex items-center gap-1.5">
                  <div className={cn('h-2 w-2 rounded-full', opt.color)} />
                  {opt.label}
                </div>
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </TableCell>
    </TableRow>
  );
}

// ─── Step 1: Workflow Mapping ────────────────────────────────────────

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
  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 5 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  );

  const handleStateDragEnd = (wfIdx: number) => (event: DragEndEvent) => {
    const { active, over } = event;
    if (!over || active.id === over.id) return;
    const wf = workflowMappings[wfIdx];
    const oldIndex = wf.states.findIndex((s) => s.shortcutState === active.id);
    const newIndex = wf.states.findIndex((s) => s.shortcutState === over.id);
    if (oldIndex === -1 || newIndex === -1) return;
    const reordered = [...wf.states];
    const [moved] = reordered.splice(oldIndex, 1);
    reordered.splice(newIndex, 0, moved);
    onUpdateMapping(wfIdx, { states: reordered.map((s, j) => ({ ...s, position: j })) });
  };

  const allReady = workflowMappings.every((wf) => {
    if (wf.mode === 'create_new') return wf.states.some((s) => s.stateType === 'done');
    return wf.existingWorkflowId !== '' && wf.states.every((s) => s.existingStateId !== '');
  });

  return (
    <div className="space-y-4">
      <div>
        <p className="text-sm text-muted-foreground">
          We auto-detected your Shortcut workflows and assigned state types. Review and adjust if
          needed, or just continue.
        </p>
      </div>

      {workflowMappings.map((wf, wfIdx) => {
        const hasDone = wf.states.some((s) => s.stateType === 'done');
        const isValid =
          wf.mode === 'create_new'
            ? hasDone && wf.newWorkflowName.trim() !== ''
            : wf.existingWorkflowId !== '' && wf.states.every((s) => s.existingStateId !== '');
        const totalTasks = wf.states.reduce((sum, s) => sum + s.storyCount, 0);

        return (
          <Card key={wf.shortcutWorkflowId || wf.shortcutWorkflowName}>
            <CardHeader
              className="cursor-pointer py-3 px-4"
              onClick={() => onUpdateMapping(wfIdx, { expanded: !wf.expanded })}
            >
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  {wf.expanded ? (
                    <ChevronDown className="h-4 w-4" />
                  ) : (
                    <ChevronRight className="h-4 w-4" />
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
                {/* Mode toggle */}
                <div className="flex gap-4">
                  <label className="flex items-center gap-1.5 text-sm cursor-pointer">
                    <input
                      type="radio"
                      checked={wf.mode === 'create_new'}
                      onChange={() => onUpdateMapping(wfIdx, { mode: 'create_new' })}
                      className="accent-primary"
                    />
                    Create new workflow
                  </label>
                  <label className="flex items-center gap-1.5 text-sm cursor-pointer">
                    <input
                      type="radio"
                      checked={wf.mode === 'use_existing'}
                      onChange={() => onUpdateMapping(wfIdx, { mode: 'use_existing' })}
                      className="accent-primary"
                    />
                    Use existing workflow
                  </label>
                </div>

                {wf.mode === 'create_new' ? (
                  <>
                    <div className="space-y-1.5">
                      <Label className="text-xs">Workflow name</Label>
                      <Input
                        value={wf.newWorkflowName}
                        onChange={(e) => onUpdateMapping(wfIdx, { newWorkflowName: e.target.value })}
                        className="h-8 text-sm"
                      />
                    </div>

                    <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={handleStateDragEnd(wfIdx)}>
                      <SortableContext items={wf.states.map((s) => s.shortcutState)} strategy={verticalListSortingStrategy}>
                        <Table>
                          <TableHeader>
                            <TableRow>
                              <TableHead className="text-xs w-[40px]" />
                              <TableHead className="text-xs">State Name</TableHead>
                              <TableHead className="text-xs text-right w-[80px]">Tasks</TableHead>
                              <TableHead className="text-xs w-[150px]">State Type</TableHead>
                            </TableRow>
                          </TableHeader>
                          <TableBody>
                            {wf.states.map((s, sIdx) => (
                              <SortableStateRow
                                key={s.shortcutState}
                                state={s}
                                onUpdateName={(name) => onUpdateState(wfIdx, sIdx, { newStateName: name })}
                                onUpdateType={(type) => onUpdateState(wfIdx, sIdx, { stateType: type as StateType })}
                              />
                            ))}
                          </TableBody>
                        </Table>
                      </SortableContext>
                    </DndContext>

                    {!hasDone && (
                      <p className="flex items-center gap-1.5 text-xs text-destructive">
                        <X className="h-3 w-3" /> At least one Done state is required.
                      </p>
                    )}
                  </>
                ) : (
                  <>
                    <div className="space-y-1.5">
                      <Label className="text-xs">Existing workflow</Label>
                      <Select
                        value={wf.existingWorkflowId}
                        onValueChange={(v) => onUpdateMapping(wfIdx, { existingWorkflowId: v })}
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

                    {wf.existingWorkflowId && (
                      <Table>
                        <TableHeader>
                          <TableRow>
                            <TableHead className="text-xs">Shortcut State</TableHead>
                            <TableHead className="text-xs text-right w-[80px]">Tasks</TableHead>
                            <TableHead className="text-xs w-[180px]">Helpin State</TableHead>
                          </TableRow>
                        </TableHeader>
                        <TableBody>
                          {wf.states.map((s, sIdx) => {
                            const selectedWf = existingWorkflows.find(
                              (ew) => ew.workflow.id === wf.existingWorkflowId,
                            );
                            return (
                              <TableRow key={s.shortcutState}>
                                <TableCell className="py-1.5 text-sm">{s.shortcutState}</TableCell>
                                <TableCell className="py-1.5 text-right text-xs text-muted-foreground">
                                  {s.storyCount.toLocaleString()}
                                </TableCell>
                                <TableCell className="py-1.5">
                                  <Select
                                    value={s.existingStateId}
                                    onValueChange={(v) =>
                                      onUpdateState(wfIdx, sIdx, { existingStateId: v })
                                    }
                                  >
                                    <SelectTrigger className="h-7 text-xs">
                                      <SelectValue placeholder="Select..." />
                                    </SelectTrigger>
                                    <SelectContent>
                                      {(selectedWf?.states || []).map((es) => (
                                        <SelectItem key={es.id} value={es.id}>
                                          {es.name}
                                        </SelectItem>
                                      ))}
                                    </SelectContent>
                                  </Select>
                                </TableCell>
                              </TableRow>
                            );
                          })}
                        </TableBody>
                      </Table>
                    )}
                  </>
                )}
              </CardContent>
            )}
          </Card>
        );
      })}

      {allReady && (
        <p className="text-xs text-green-600 dark:text-green-400 flex items-center gap-1.5">
          <Check className="h-3.5 w-3.5" />
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
            <Mail className="mr-1.5 h-3.5 w-3.5" />
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
                      <Check className="h-3.5 w-3.5 text-green-500" />
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
                          <Mail className="h-3.5 w-3.5 text-blue-500" />
                        ) : (
                          <AlertTriangle className="h-3.5 w-3.5 text-yellow-500" />
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
                                  <Mail className="h-3 w-3" /> Invite &amp; Map
                                </span>
                              </SelectItem>
                              <SelectItem value="__action__skip">
                                <span className="flex items-center gap-1">
                                  <X className="h-3 w-3" /> Skip
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
  const newWorkflows = workflowMappings.filter((w) => w.mode === 'create_new').length;
  const existingWorkflows = workflowMappings.filter((w) => w.mode === 'use_existing').length;
  const newStates = workflowMappings
    .filter((w) => w.mode === 'create_new')
    .reduce((sum, w) => sum + w.states.length, 0);

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
                ? `Failed at step — ${progress?.current_step || 'unknown'}`
                : `Step ${progress?.steps_completed || 0} of ${progress?.steps_total || 0} — ${progress?.current_step || '...'}`}
          </p>
        </div>

        {isRunning && progress && (
          <div className="space-y-1">
            {(hasApiToken ? IMPORT_STEPS_API : IMPORT_STEPS_BASE).map((stepLabel, i) => {
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
                    <Check className="h-3.5 w-3.5" />
                  ) : active ? (
                    <Loader2 className="h-3.5 w-3.5 animate-spin" />
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
            <Switch checked={importArchived} onCheckedChange={onArchived} />
          </div>
          <div className="flex items-center justify-between">
            <Label className="text-sm">Import completed tasks</Label>
            <Switch checked={importCompleted} onCheckedChange={onCompleted} />
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
                <SummaryRow label="Teams" value={`${s.teams_count}`} />
                <SummaryRow
                  label="Workflows"
                  value={`${workflowMappings.length} (${newWorkflows} new, ${existingWorkflows} existing)`}
                />
                <SummaryRow label="Workflow States" value={`${newStates} new`} />
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
                <AlertTriangle className="mt-0.5 h-3 w-3 shrink-0" />
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
    { label: 'Tasks', created: result.stories_created, skipped: result.stories_skipped },
    { label: 'Checklist Items', created: result.checklist_items_created },
    { label: 'Owner Links', created: result.owner_links_created },
    { label: 'Label Links', created: result.label_links_created },
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
                <AlertTriangle className="mt-0.5 h-3 w-3 shrink-0" />
                <span>{w}</span>
              </div>
            ))}
          </CardContent>
        </Card>
      )}
    </div>
  );
}
