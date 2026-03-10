import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { Sparkles } from 'lucide-react';
import { toast } from 'sonner';

import { Separator } from '@/components/ui/separator';
import { useDocsDocument, useDocsLinkedDocs } from '@/hooks/queries/useDocs';
import { agentService } from '@/lib/services/agentService';
import type {
  Agent,
  AgentRun,
  AgentRunArtifact,
  Epic,
  KickoffExecutionResult,
  OrchestrationProposal,
  ProposedStory,
  SpecClarification,
} from '@/lib/pmTypes';

import { ApproveSpecStep } from './ApproveSpecStep';
import { ClarifySpecStep } from './ClarifySpecStep';
import { DraftSpecStep } from './DraftSpecStep';
import { ExecuteStep } from './ExecuteStep';
import { GenerateStoriesStep } from './GenerateStoriesStep';
import { PlannerSetupStep } from './PlannerSetupStep';
import { PlanningActivityLog } from './PlanningActivityLog';
import { PlanningProgress } from './PlanningProgress';
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
  const [assigning, setAssigning] = useState(false);
  const [loadingRuns, setLoadingRuns] = useState(true);
  const [triggeringDraft, setTriggeringDraft] = useState(false);
  const [savingClarifications, setSavingClarifications] = useState(false);
  const [approvingSpec, setApprovingSpec] = useState(false);
  const [triggeringPlan, setTriggeringPlan] = useState(false);
  const [confirmingPlan, setConfirmingPlan] = useState(false);
  const [kickingOff, setKickingOff] = useState(false);
  const [actingOnRun, setActingOnRun] = useState<string | null>(null);
  const [lastExecutionResult, setLastExecutionResult] = useState<KickoffExecutionResult | null>(null);
  const lastDraftRefreshKey = useRef<string>('');
  const lastPlanRefreshKey = useRef<string>('');

  useDocsLinkedDocs(workspaceId, 'epic', epic.id);
  const specDocQuery = useDocsDocument(workspaceId, epic.spec_document_id ?? '');

  // Sync props
  useEffect(() => {
    setAssignedAgentId(epic.orchestrator_agent_id ?? '');
    setSelectedAgentId(epic.orchestrator_agent_id ?? '');
  }, [epic.orchestrator_agent_id]);

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

  // Poll active runs
  useEffect(() => {
    const active = runs.some((run) => ['queued', 'running', 'awaiting_approval'].includes(run.status));
    if (!active) return;
    const interval = window.setInterval(() => { void fetchRuns(); }, 5000);
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
  }, [additionalContext, ensureAssignedAgent, epic.id, fetchRuns, onStoriesCreated, workspaceId]);

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
    setApprovingSpec(true);
    try {
      const res = await agentService.approveEpicSpec(workspaceId, epic.id);
      if (res.error) { toast.error(res.error); return; }
      toast.success('Spec approved');
      await fetchRuns();
      onStoriesCreated?.();
    } finally { setApprovingSpec(false); }
  }, [canApproveSpec, epic.id, fetchRuns, onStoriesCreated, workspaceId]);

  const handlePlanStories = useCallback(async () => {
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
  }, [additionalContext, ensureAssignedAgent, epic.id, fetchRuns, onStoriesCreated, workspaceId]);

  const handleConfirmPlan = useCallback(async () => {
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
  }, [editedStories, fetchRuns, latestPlanRun, loadArtifacts, onStoriesCreated, selectedRun, selectedRunStage, workspaceId]);

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

  const handleCancelRun = useCallback(async (runId: string) => {
    setActingOnRun(runId);
    try {
      const res = await agentService.cancelRun(workspaceId, runId);
      if (res.error) { toast.error(res.error); return; }
      toast.success('Run cancelled');
      await fetchRuns();
    } finally { setActingOnRun(null); }
  }, [fetchRuns, workspaceId]);

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
          canConfirm={editedStories.length > 0 && !!(selectedRunStage === 'plan_stories' ? selectedRun : latestPlanRun)}
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

      <div className="mt-4">
        <PlanningActivityLog
          runs={runs}
          loadingRuns={loadingRuns}
          selectedRunId={selectedRunId}
          onSelectRun={setSelectedRunId}
          selectedRun={selectedRun}
          artifacts={artifacts}
          actingOnRun={actingOnRun}
          onCancelRun={(id) => void handleCancelRun(id)}
        />
      </div>
    </div>
  );
}
