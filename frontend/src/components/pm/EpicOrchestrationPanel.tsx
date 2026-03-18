import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { Sparkles } from 'lucide-react';
import { toast } from 'sonner';

import { Separator } from '@/components/ui/separator';
import type { WSEvent } from '@/hooks/useWebSocket';
import { useDocsDocument, useDocsLinkedDocs } from '@/hooks/queries/useDocs';
import { agentService } from '@/lib/services/agentService';
import type {
  Agent,
  AgentRun,
  AgentRunArtifact,
  Epic,
  FlowNodeRun,
  KickoffExecutionResult,
  OrchestrationProposal,
  ProposedStory,
  SpecClarification,
} from '@/lib/pmTypes';

import { useFlowRun, useSendFlowNodeAction, useStartFlowRun } from '@/hooks/queries';

import { ApproveSpecStep } from './ApproveSpecStep';
import { ClarifySpecStep } from './ClarifySpecStep';
import { DraftSpecStep } from './DraftSpecStep';
import { ExecuteStep } from './ExecuteStep';
import { GenerateStoriesStep } from './GenerateStoriesStep';
import { PlannerSetupStep } from './PlannerSetupStep';
import { PlanningProgress } from './PlanningProgress';
import { PlanningSessionPanel } from './PlanningSessionPanel';
import { computeCurrentStep, getStepStatus } from './planningStepUtils';
import { ReviewStoriesStep } from './ReviewStoriesStep';

interface Props {
  epic: Epic;
  workspaceId: string;
  workspaceSlug: string;
  onStoriesCreated?: () => void;
}

interface ProductSpecDraft {
  title: string;
  summary: string;
  spec_markdown: string;
  risks?: string[];
  assumptions?: string[];
  open_questions?: string[];
  sources?: {
    title: string;
    url: string;
    note?: string;
    published_at?: string;
  }[];
}

interface CreatedPlanningStory {
  story_id: string;
  ref?: string;
  name: string;
  story_type: string;
  estimate?: number;
  priority?: string;
  acceptance_criteria?: string[];
  dependency_refs?: string[];
}

interface PlanningRunSummary {
  stage?: string;
  spec_document_id?: string;
  spec_version_id?: string;
  summary?: string;
  risks?: string[];
  assumptions?: string[];
  open_questions?: string[];
  clarifications?: SpecClarification[];
  proposal?: OrchestrationProposal;
  created_story_ids?: string[];
  created_stories?: CreatedPlanningStory[];
}

// --- Helpers ---

function parseJSONValue<T>(value: unknown): T | null {
  if (!value) return null;
  if (typeof value === 'string') {
    try { return JSON.parse(value) as T; } catch { return null; }
  }
  if (typeof value === 'object') return value as T;
  return null;
}

function getRunStage(run: AgentRun | null): string {
  if (!run) return '';
  const stage = run.input?.stage;
  return typeof stage === 'string' ? stage : '';
}

function findArtifactPayload<T>(artifacts: AgentRunArtifact[], artifactTypes: string[]): T | null {
  const artifact = [...artifacts]
    .sort((a, b) => b.sequence_no - a.sequence_no)
    .find((item) => artifactTypes.includes(item.artifact_type) && item.inline_content);
  if (!artifact?.inline_content) return null;
  return parseJSONValue<T>(artifact.inline_content);
}

function parsePlanningSummary(run: AgentRun | null): PlanningRunSummary | null {
  return parseJSONValue<PlanningRunSummary>(run?.output_summary);
}

function parseProposal(artifacts: AgentRunArtifact[], run: AgentRun | null): OrchestrationProposal | null {
  const artifactProposal = findArtifactPayload<OrchestrationProposal>(artifacts, ['story_plan_proposal', 'orchestration_proposal']);
  if (artifactProposal?.proposed_stories?.length) return artifactProposal;
  const summary = parsePlanningSummary(run);
  if (summary?.proposal?.proposed_stories?.length) return summary.proposal;
  const runProposal = parseJSONValue<OrchestrationProposal>(run?.output_summary);
  if (runProposal?.proposed_stories?.length) return runProposal;
  return null;
}

function parseSpecDraft(artifacts: AgentRunArtifact[]): ProductSpecDraft | null {
  return findArtifactPayload<ProductSpecDraft>(artifacts, ['product_spec_draft']);
}

function parseExecutionResult(value: unknown): KickoffExecutionResult | null {
  return parseJSONValue<KickoffExecutionResult>(value);
}

function latestFlowNodeRun(nodeRuns: FlowNodeRun[], nodeIds: string | string[]): FlowNodeRun | null {
  const allowed = new Set(Array.isArray(nodeIds) ? nodeIds : [nodeIds]);
  return [...nodeRuns].reverse().find((item) => allowed.has(item.node_id)) ?? null;
}

// --- Component ---

export function EpicOrchestrationPanel({ epic, workspaceId, workspaceSlug, onStoriesCreated }: Props) {
  const navigate = useNavigate();

  // State
  const [agents, setAgents] = useState<Agent[]>([]);
  const [assignedAgentId, setAssignedAgentId] = useState(epic.orchestrator_agent_id ?? '');
  const [selectedAgentId, setSelectedAgentId] = useState(epic.orchestrator_agent_id ?? '');
  const [runs, setRuns] = useState<AgentRun[]>([]);
  const [selectedRunId, setSelectedRunId] = useState<string | null>(null);
  const [artifacts, setArtifacts] = useState<AgentRunArtifact[]>([]);
  const [editedStories, setEditedStories] = useState<ProposedStory[]>([]);
  const [selectedStoryIds, setSelectedStoryIds] = useState<string[]>([]);
  const [clarifications, setClarifications] = useState<SpecClarification[]>(epic.spec_clarifications ?? []);
  const [additionalContext, setAdditionalContext] = useState('');
  const [localSessionId, setLocalSessionId] = useState<string | null>(epic.active_planning_session_id ?? null);
  const [localFlowRunId, setLocalFlowRunId] = useState<string | null>(epic.active_flow_run_id ?? null);
  const [dismissedSessionId, setDismissedSessionId] = useState<string | null>(null);
  const [dismissedFlowRunId, setDismissedFlowRunId] = useState<string | null>(null);
  const [assigning, setAssigning] = useState(false);
  const [, setLoadingRuns] = useState(true);
  const [triggeringDraft, setTriggeringDraft] = useState(false);
  const [savingClarifications, setSavingClarifications] = useState(false);
  const [approvingSpec, setApprovingSpec] = useState(false);
  const [requestingSpecChanges, setRequestingSpecChanges] = useState(false);
  const [specReviewComment, setSpecReviewComment] = useState('');
  const [triggeringPlan, setTriggeringPlan] = useState(false);
  const [confirmingPlan, setConfirmingPlan] = useState(false);
  const [requestingPlanChanges, setRequestingPlanChanges] = useState(false);
  const [planReviewComment, setPlanReviewComment] = useState('');
  const [kickingOff, setKickingOff] = useState(false);
  const [lastExecutionResult, setLastExecutionResult] = useState<KickoffExecutionResult | null>(null);
  const lastDraftRefreshKey = useRef<string>('');
  const lastPlanRefreshKey = useRef<string>('');
  const lastFlowRefreshKey = useRef<string>('');

  useDocsLinkedDocs(workspaceId, 'epic', epic.id);
  const specDocQuery = useDocsDocument(workspaceId, epic.spec_document_id ?? '');
  const epicFlowRunId = epic.active_flow_run_id === dismissedFlowRunId ? null : epic.active_flow_run_id;
  const resolvedLocalFlowRunId = localFlowRunId === dismissedFlowRunId ? null : localFlowRunId;
  const activeFlowRunId = resolvedLocalFlowRunId ?? epicFlowRunId;
  const { data: flowRunView } = useFlowRun(workspaceId, activeFlowRunId ?? undefined);

  // Sync props
  useEffect(() => {
    setAssignedAgentId(epic.orchestrator_agent_id ?? '');
    setSelectedAgentId(epic.orchestrator_agent_id ?? '');
  }, [epic.orchestrator_agent_id]);

  useEffect(() => {
    setLocalSessionId(epic.active_planning_session_id ?? null);
  }, [epic.active_planning_session_id]);

  useEffect(() => {
    setLocalFlowRunId(epic.active_flow_run_id ?? null);
  }, [epic.active_flow_run_id]);

  useEffect(() => {
    setClarifications(epic.spec_clarifications ?? []);
  }, [epic.spec_clarifications]);

  // Fetchers
  const fetchAgents = useCallback(async () => {
    const res = await agentService.list(workspaceId);
    if (res.error) { toast.error(res.error); return; }
    const data = Array.isArray(res.data) ? res.data : [];
    setAgents(data.filter((agent) => agent.agent_kind === 'llm' && agent.agent_class === 'product_planner'));
  }, [workspaceId]);

  const fetchRuns = useCallback(async () => {
    setLoadingRuns(true);
    try {
      const res = await agentService.listEpicRuns(workspaceId, epic.id);
      if (res.error) { toast.error(res.error); return; }
      const data = Array.isArray(res.data) ? res.data : [];
      setRuns(data);
      setSelectedRunId((current) => current ?? data[0]?.id ?? null);
    } finally {
      setLoadingRuns(false);
    }
  }, [workspaceId, epic.id]);

  const loadArtifacts = useCallback(async (runId: string) => {
    const res = await agentService.listRunArtifacts(workspaceId, runId);
    if (res.error) { toast.error(res.error); return; }
    setArtifacts(Array.isArray(res.data) ? res.data : []);
  }, [workspaceId]);

  useEffect(() => { void fetchAgents(); }, [fetchAgents]);
  useEffect(() => { void fetchRuns(); }, [fetchRuns]);

  // Listen for WebSocket agent_run events targeting this epic
  useEffect(() => {
    const handler = (e: Event) => {
      const detail = (e as CustomEvent<WSEvent>).detail;
      if (detail.parent_type === 'epic' && detail.parent_id === epic.id) {
        void fetchRuns();
      }
    };
    window.addEventListener('agent_run-updated', handler);
    return () => window.removeEventListener('agent_run-updated', handler);
  }, [epic.id, fetchRuns]);

  // Fallback poll at 30s for Temporal-driven terminal states (no WS event yet)
  useEffect(() => {
    const active = runs.some((r) => ['queued', 'running'].includes(r.status));
    if (!active) return;
    const interval = window.setInterval(() => { void fetchRuns(); }, 30_000);
    return () => window.clearInterval(interval);
  }, [runs, fetchRuns]);

  // Load artifacts for selected run
  useEffect(() => {
    if (!selectedRunId) { setArtifacts([]); return; }
    void loadArtifacts(selectedRunId);
  }, [selectedRunId, loadArtifacts]);

  // Derived data
  const selectedRun = useMemo(() => runs.find((r) => r.id === selectedRunId) ?? null, [runs, selectedRunId]);
  const selectedRunStage = useMemo(() => getRunStage(selectedRun), [selectedRun]);

  const selectedProposal = useMemo(() => parseProposal(artifacts, selectedRun), [artifacts, selectedRun]);
  const selectedDraft = useMemo(() => parseSpecDraft(artifacts), [artifacts]);

  const latestPlanRun = useMemo(() => runs.find((r) => getRunStage(r) === 'plan_stories') ?? null, [runs]);
  const latestDraftRun = useMemo(() => runs.find((r) => getRunStage(r) === 'draft_spec') ?? null, [runs]);

  const selectedSummary = useMemo(() => parsePlanningSummary(selectedRun), [selectedRun]);
  const planSummary = useMemo(
    () => (selectedRunStage === 'plan_stories' ? selectedSummary : parsePlanningSummary(latestPlanRun)),
    [latestPlanRun, selectedRunStage, selectedSummary],
  );
  const createdStories = useMemo(() => planSummary?.created_stories ?? [], [planSummary]);

  // Sync edited stories from proposal
  useEffect(() => {
    if (selectedProposal?.proposed_stories) { setEditedStories(selectedProposal.proposed_stories); return; }
    setEditedStories([]);
  }, [selectedProposal]);

  // Sync selected story IDs from created stories
  useEffect(() => {
    if (createdStories.length === 0) { setSelectedStoryIds([]); return; }
    setSelectedStoryIds((current) => {
      if (current.length > 0) return current;
      return createdStories.map((s) => s.story_id);
    });
  }, [createdStories]);

  useEffect(() => {
    if (!latestDraftRun || !onStoriesCreated) return;
    const key = `${latestDraftRun.id}:${latestDraftRun.status}:${latestDraftRun.updated_at}`;
    if (lastDraftRefreshKey.current === key) return;
    if (['awaiting_approval', 'completed', 'failed', 'cancelled'].includes(latestDraftRun.status)) {
      lastDraftRefreshKey.current = key;
      onStoriesCreated();
    }
  }, [latestDraftRun, onStoriesCreated]);

  useEffect(() => {
    if (!latestPlanRun || !onStoriesCreated) return;
    const key = `${latestPlanRun.id}:${latestPlanRun.status}:${latestPlanRun.updated_at}`;
    if (lastPlanRefreshKey.current === key) return;
    if (['awaiting_approval', 'completed', 'failed', 'cancelled'].includes(latestPlanRun.status)) {
      lastPlanRefreshKey.current = key;
      onStoriesCreated();
    }
  }, [latestPlanRun, onStoriesCreated]);

  // Compute current step
  const currentStep = useMemo(() => computeCurrentStep(epic, agents, runs), [epic, agents, runs]);
  const flowNodeRuns = flowRunView?.node_runs ?? [];
  const specPlanningNode = useMemo(() => latestFlowNodeRun(flowNodeRuns, ['spec_draft', 'spec_planning']), [flowNodeRuns]);
  const specApprovalNode = useMemo(() => latestFlowNodeRun(flowNodeRuns, 'spec_approval'), [flowNodeRuns]);
  const planApprovalNode = useMemo(() => latestFlowNodeRun(flowNodeRuns, 'plan_approval'), [flowNodeRuns]);
  const epicSessionId = epic.active_planning_session_id === dismissedSessionId ? null : epic.active_planning_session_id;
  const resolvedLocalSessionId = localSessionId === dismissedSessionId ? null : localSessionId;
  const activeInteractiveSessionId =
    specPlanningNode?.child_session_id ?? resolvedLocalSessionId ?? epicSessionId ?? null;
  const activeInteractiveNodeId =
    ['spec_draft', 'spec_planning'].includes(flowRunView?.run.current_node_id ?? '') && specPlanningNode?.status === 'awaiting_input'
      ? specPlanningNode.id
      : undefined;
  const flowInteractiveActive =
    Boolean(activeFlowRunId) &&
    ['spec_draft', 'spec_planning'].includes(flowRunView?.run.current_node_id ?? '') &&
    specPlanningNode?.status === 'awaiting_input' &&
    Boolean(activeInteractiveSessionId);
  const legacyInteractiveActive = !activeFlowRunId && Boolean(activeInteractiveSessionId);
  const pendingClarifyCount = useMemo(
    () =>
      clarifications.filter((item) => {
        if (item.kind === 'open_question') return item.disposition !== 'answered' || !item.response?.trim();
        if (item.kind === 'assumption') {
          if (item.disposition === 'accepted') return false;
          if (item.disposition === 'rejected') return !item.response?.trim();
          return true;
        }
        return false;
      }).length,
    [clarifications],
  );
  const canApproveSpec = pendingClarifyCount === 0;
  const specNodeAction = useSendFlowNodeAction(workspaceId, activeFlowRunId ?? '', specApprovalNode?.id ?? activeInteractiveNodeId ?? '');
  const planNodeAction = useSendFlowNodeAction(workspaceId, activeFlowRunId ?? '', planApprovalNode?.id ?? '');

  useEffect(() => {
    if (!flowRunView || !onStoriesCreated) return;
    const key = `${flowRunView.run.id}:${flowRunView.run.current_node_id}:${flowRunView.run.status}:${flowRunView.run.updated_at}`;
    if (lastFlowRefreshKey.current === key) return;
    lastFlowRefreshKey.current = key;
    void fetchRuns();
    onStoriesCreated();
  }, [fetchRuns, flowRunView, onStoriesCreated]);

  // --- Handlers ---

  const ensureAssignedAgent = useCallback(async () => {
    if (assignedAgentId) return assignedAgentId;
    if (!selectedAgentId) { toast.error('Choose a planner first'); return ''; }
    setAssigning(true);
    try {
      const res = await agentService.assignOrchestrator(workspaceId, epic.id, selectedAgentId);
      if (res.error) { toast.error(res.error); return ''; }
      setAssignedAgentId(selectedAgentId);
      toast.success('Planner assigned');
      return selectedAgentId;
    } finally { setAssigning(false); }
  }, [assignedAgentId, epic.id, selectedAgentId, workspaceId]);

  const handleAssignAgent = useCallback(async () => {
    if (!selectedAgentId) { toast.error('Choose a planner first'); return; }
    setAssigning(true);
    try {
      const res = await agentService.assignOrchestrator(workspaceId, epic.id, selectedAgentId);
      if (res.error) { toast.error(res.error); return; }
      setAssignedAgentId(selectedAgentId);
      toast.success('Planner assigned');
    } finally { setAssigning(false); }
  }, [epic.id, selectedAgentId, workspaceId]);

  const handleDraftSpec = useCallback(async () => {
    if (activeFlowRunId) {
      toast.message('Spec planning is already managed by the active flow');
      return;
    }
    const agentId = await ensureAssignedAgent();
    if (!agentId) return;
    setTriggeringDraft(true);
    try {
      const res = await agentService.draftEpicSpec(workspaceId, epic.id, additionalContext);
      if (res.error) { toast.error(res.error); return; }
      toast.success('Spec draft started');
      setAdditionalContext('');
      setLastExecutionResult(null);
      await fetchRuns();
      onStoriesCreated?.();
      if (res.data?.id) setSelectedRunId(res.data.id);
    } finally { setTriggeringDraft(false); }
  }, [activeFlowRunId, additionalContext, ensureAssignedAgent, epic.id, fetchRuns, onStoriesCreated, workspaceId]);

  const updateClarification = useCallback((id: string, patch: Partial<SpecClarification>) => {
    setClarifications((current) =>
      current.map((item) => (item.id === id ? { ...item, ...patch } : item)),
    );
  }, []);

  const handleSaveClarifications = useCallback(async () => {
    if (clarifications.length === 0) {
      toast.success('No clarifications needed');
      return;
    }
    setSavingClarifications(true);
    try {
      const res = await agentService.clarifyEpicSpec(workspaceId, epic.id, clarifications);
      if (res.error) { toast.error(res.error); return; }
      if (Array.isArray(res.data?.clarifications)) {
        setClarifications(res.data.clarifications);
      }
      toast.success(res.data?.pending_clarify_count ? 'Clarification responses saved' : 'Clarifications resolved');
      onStoriesCreated?.();
    } finally { setSavingClarifications(false); }
  }, [clarifications, epic.id, onStoriesCreated, workspaceId]);

  const handleApproveSpec = useCallback(async () => {
    if (!canApproveSpec) {
      toast.error('Resolve all open questions and assumptions before approval');
      return;
    }
    if (activeFlowRunId && specApprovalNode?.id && specApprovalNode.status === 'awaiting_approval') {
      setApprovingSpec(true);
      try {
        await specNodeAction.mutateAsync({ actionType: 'approve' });
        toast.success('Spec approved');
        await fetchRuns();
        onStoriesCreated?.();
      } catch (err) {
        toast.error(err instanceof Error ? err.message : 'Failed to approve spec');
      } finally {
        setApprovingSpec(false);
      }
      return;
    }
    setApprovingSpec(true);
    try {
      const res = await agentService.approveEpicSpec(workspaceId, epic.id);
      if (res.error) { toast.error(res.error); return; }
      toast.success('Spec approved');
      await fetchRuns();
      onStoriesCreated?.();
    } finally { setApprovingSpec(false); }
  }, [activeFlowRunId, canApproveSpec, epic.id, fetchRuns, onStoriesCreated, specApprovalNode?.id, specApprovalNode?.status, specNodeAction, workspaceId]);

  const handlePlanStories = useCallback(async () => {
    if (activeFlowRunId) {
      toast.message('Story planning is already managed by the active flow');
      return;
    }
    const agentId = await ensureAssignedAgent();
    if (!agentId) return;
    setTriggeringPlan(true);
    try {
      const res = await agentService.planEpicStories(workspaceId, epic.id, additionalContext);
      if (res.error) { toast.error(res.error); return; }
      toast.success('Story generation started');
      setAdditionalContext('');
      await fetchRuns();
      onStoriesCreated?.();
      if (res.data?.id) setSelectedRunId(res.data.id);
    } finally { setTriggeringPlan(false); }
  }, [activeFlowRunId, additionalContext, ensureAssignedAgent, epic.id, fetchRuns, onStoriesCreated, workspaceId]);

  const handleRequestSpecChanges = useCallback(async () => {
    if (!activeFlowRunId || !specApprovalNode?.id || specApprovalNode.status !== 'awaiting_approval') {
      toast.error('Spec is not currently awaiting approval');
      return;
    }
    if (!specReviewComment.trim()) {
      toast.error('Add reviewer feedback before requesting changes');
      return;
    }
    setRequestingSpecChanges(true);
    try {
      await specNodeAction.mutateAsync({
        actionType: 'request_changes',
        payload: { comment: specReviewComment.trim() },
      });
      setSpecReviewComment('');
      toast.success('Requested spec changes');
      await fetchRuns();
      onStoriesCreated?.();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to request spec changes');
    } finally {
      setRequestingSpecChanges(false);
    }
  }, [activeFlowRunId, fetchRuns, onStoriesCreated, specApprovalNode?.id, specApprovalNode?.status, specNodeAction, specReviewComment]);

  const handleConfirmPlan = useCallback(async () => {
    if (activeFlowRunId && planApprovalNode?.id && planApprovalNode.status === 'awaiting_approval') {
      if (!editedStories.length) { toast.error('No proposed stories to create'); return; }
      setConfirmingPlan(true);
      try {
        await planNodeAction.mutateAsync({
          actionType: 'approve',
          payload: { proposed_stories: editedStories },
        });
        toast.success(`Created ${editedStories.length} stories`);
        await fetchRuns();
        onStoriesCreated?.();
      } catch (err) {
        toast.error(err instanceof Error ? err.message : 'Failed to confirm plan');
      } finally {
        setConfirmingPlan(false);
      }
      return;
    }
    const run = selectedRunStage === 'plan_stories' ? selectedRun : latestPlanRun;
    if (!run) { toast.error('Select a story planning run first'); return; }
    if (!editedStories.length) { toast.error('No proposed stories to create'); return; }
    setConfirmingPlan(true);
    try {
      const res = await agentService.confirmOrchestrationRun(workspaceId, run.id, editedStories);
      if (res.error) { toast.error(res.error); return; }
      toast.success(`Created ${editedStories.length} stories`);
      await fetchRuns();
      await loadArtifacts(run.id);
      onStoriesCreated?.();
    } finally { setConfirmingPlan(false); }
  }, [activeFlowRunId, editedStories, fetchRuns, latestPlanRun, loadArtifacts, onStoriesCreated, planApprovalNode?.id, planApprovalNode?.status, planNodeAction, selectedRun, selectedRunStage, workspaceId]);

  const handleRequestPlanChanges = useCallback(async () => {
    if (!activeFlowRunId || !planApprovalNode?.id || planApprovalNode.status !== 'awaiting_approval') {
      toast.error('Story plan is not currently awaiting approval');
      return;
    }
    if (!planReviewComment.trim()) {
      toast.error('Add reviewer feedback before requesting changes');
      return;
    }
    setRequestingPlanChanges(true);
    try {
      await planNodeAction.mutateAsync({
        actionType: 'request_changes',
        payload: { comment: planReviewComment.trim() },
      });
      setPlanReviewComment('');
      toast.success('Requested story plan changes');
      await fetchRuns();
      onStoriesCreated?.();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to request plan changes');
    } finally {
      setRequestingPlanChanges(false);
    }
  }, [activeFlowRunId, fetchRuns, onStoriesCreated, planApprovalNode?.id, planApprovalNode?.status, planNodeAction, planReviewComment]);

  const handleKickoffExecution = useCallback(async () => {
    const run = selectedRunStage === 'plan_stories' ? selectedRun : latestPlanRun;
    if (!run) { toast.error('Select a confirmed story plan first'); return; }
    if (selectedStoryIds.length === 0) { toast.error('Select at least one story'); return; }
    setKickingOff(true);
    try {
      const res = await agentService.kickoffEpicExecution(workspaceId, epic.id, run.id, selectedStoryIds);
      if (res.error) { toast.error(res.error); return; }
      setLastExecutionResult(parseExecutionResult(res.data) ?? null);
      toast.success('Execution started');
      await fetchRuns();
      onStoriesCreated?.();
    } finally { setKickingOff(false); }
  }, [epic.id, fetchRuns, latestPlanRun, onStoriesCreated, selectedRun, selectedRunStage, selectedStoryIds, workspaceId]);

  // TODO: Wire up cancel run UI
  // const handleCancelRun = useCallback(async (runId: string) => {
  //   try {
  //     const res = await agentService.cancelRun(workspaceId, runId);
  //     if (res.error) { toast.error(res.error); return; }
  //     toast.success('Run cancelled');
  //     await fetchRuns();
  //   } catch { /* noop */ }
  // }, [fetchRuns, workspaceId]);

  const openSpecDoc = useCallback(() => {
    if (!epic.spec_document_id) { toast.error('No spec document yet'); return; }
    navigate({ to: '/w/$slug/docs/documents/$docId', params: { slug: workspaceSlug, docId: epic.spec_document_id } });
  }, [epic.spec_document_id, navigate, workspaceSlug]);

  const updateStoryField = useCallback(<K extends keyof ProposedStory>(index: number, field: K, value: ProposedStory[K]) => {
    setEditedStories((current) =>
      current.map((story, i) => (i === index ? { ...story, [field]: value } : story)),
    );
  }, []);

  const toggleExecutionStory = useCallback((storyId: string, checked: boolean) => {
    setSelectedStoryIds((current) =>
      checked ? [...current, storyId] : current.filter((v) => v !== storyId),
    );
  }, []);

  const specDocTitle = specDocQuery.data?.title ?? 'Product Spec';

  // Interactive session
  const startFlowMutation = useStartFlowRun(workspaceId);
  const handleStartSession = async () => {
    const agentId = await ensureAssignedAgent();
    if (!agentId) return;
    if (activeFlowRunId) {
      toast.message('Epic planning flow is already active');
      return;
    }
    try {
      const flow = await startFlowMutation.mutateAsync({
        template_id: 'pm.epic_planning_v2',
        target_type: 'epic',
        target_id: epic.id,
        input: {
          spec_planner_agent_id: agentId,
          additional_context: additionalContext || undefined,
        },
      });
      setDismissedFlowRunId(null);
      setDismissedSessionId(null);
      setLocalFlowRunId(flow.run.id);
      const sessionNode = latestFlowNodeRun(flow.node_runs ?? [], ['spec_draft', 'spec_planning']);
      setLocalSessionId(sessionNode?.child_session_id ?? null);
      setAdditionalContext('');
      onStoriesCreated?.();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to start session');
    }
  };

  const flowTerminal = flowRunView?.run.status === 'cancelled' || flowRunView?.run.status === 'failed' || flowRunView?.run.status === 'completed';
  const isInSession = !flowTerminal && (flowInteractiveActive || legacyInteractiveActive);

  // --- Render ---

  return (
    <div className="mt-6">
      <Separator className="mb-6" />

      <div className="mb-4 flex items-center gap-2">
        <Sparkles className="h-4 w-4 text-amber-500" />
        <h3 className="text-sm font-semibold">Product Planning</h3>
      </div>

      <div className="mb-5 flex justify-center">
        <PlanningProgress currentStep={currentStep} />
      </div>

      {isInSession ? (
        <PlanningSessionPanel
          epic={epic}
          workspaceId={workspaceId}
          sessionId={activeInteractiveSessionId ?? undefined}
          flowRunId={activeFlowRunId ?? undefined}
          nodeRunId={activeInteractiveNodeId}
          onComplete={() => {
            setDismissedSessionId(activeInteractiveSessionId ?? null);
            setDismissedFlowRunId(activeFlowRunId ?? null);
            setLocalSessionId(null);
            setLocalFlowRunId(null);
            void fetchRuns();
            onStoriesCreated?.();
          }}
        />
      ) : (
      <div className="space-y-3">
        <PlannerSetupStep
          status={getStepStatus('setup', currentStep)}
          epic={epic}
          agents={agents}
          selectedAgentId={selectedAgentId}
          onSelectAgent={setSelectedAgentId}
          onAssign={() => void handleAssignAgent()}
          assigning={assigning}
        />

        <DraftSpecStep
          status={getStepStatus('draft', currentStep)}
          epic={epic}
          specDocTitle={specDocTitle}
          latestDraftRun={latestDraftRun}
          specDraft={selectedDraft}
          additionalContext={additionalContext}
          onAdditionalContextChange={setAdditionalContext}
          onDraftSpec={() => void handleDraftSpec()}
          onOpenSpecDoc={openSpecDoc}
          triggeringDraft={triggeringDraft}
          onStartSession={() => void handleStartSession()}
          startingSession={startFlowMutation.isPending}
        />

        <ClarifySpecStep
          status={getStepStatus('clarify', currentStep)}
          clarifications={clarifications}
          saving={savingClarifications}
          specDocTitle={specDocTitle}
          onUpdateClarification={updateClarification}
          onSaveClarifications={() => void handleSaveClarifications()}
          onOpenSpecDoc={openSpecDoc}
        />

        <ApproveSpecStep
          status={getStepStatus('approve', currentStep)}
          epic={epic}
          specDocTitle={specDocTitle}
          canApprove={canApproveSpec}
          pendingClarifyCount={pendingClarifyCount}
          onApproveSpec={() => void handleApproveSpec()}
          onOpenSpecDoc={openSpecDoc}
          approvingSpec={approvingSpec}
          canRequestChanges={Boolean(activeFlowRunId && specApprovalNode?.status === 'awaiting_approval')}
          requestChangesComment={specReviewComment}
          onRequestChangesCommentChange={setSpecReviewComment}
          onRequestChanges={() => void handleRequestSpecChanges()}
          requestingChanges={requestingSpecChanges}
        />

        <GenerateStoriesStep
          status={getStepStatus('generate', currentStep)}
          epic={epic}
          latestPlanRun={latestPlanRun}
          proposal={selectedProposal}
          additionalContext={additionalContext}
          onAdditionalContextChange={setAdditionalContext}
          onGenerateStories={() => void handlePlanStories()}
          triggeringPlan={triggeringPlan}
        />

        <ReviewStoriesStep
          status={getStepStatus('review', currentStep)}
          proposal={selectedProposal}
          editedStories={editedStories}
          onUpdateStory={updateStoryField}
          onConfirmPlan={() => void handleConfirmPlan()}
          confirmingPlan={confirmingPlan}
          canConfirm={
            editedStories.length > 0 &&
            (Boolean(activeFlowRunId && planApprovalNode?.status === 'awaiting_approval') ||
              Boolean(selectedRunStage === 'plan_stories' ? selectedRun : latestPlanRun))
          }
          canRequestChanges={Boolean(activeFlowRunId && planApprovalNode?.status === 'awaiting_approval')}
          requestChangesComment={planReviewComment}
          onRequestChangesCommentChange={setPlanReviewComment}
          onRequestChanges={() => void handleRequestPlanChanges()}
          requestingChanges={requestingPlanChanges}
        />

        <ExecuteStep
          status={getStepStatus('execute', currentStep)}
          createdStories={createdStories}
          selectedStoryIds={selectedStoryIds}
          onToggleStory={toggleExecutionStory}
          onKickoff={() => void handleKickoffExecution()}
          kickingOff={kickingOff}
          executionResult={lastExecutionResult}
        />
      </div>
      )}

    </div>
  );
}
