import { useCallback, useEffect, useMemo, useState } from 'react';
import { Bot, Check, Clock, FileText, Loader2, Play, ShieldCheck, Sparkles, StopCircle, Trash2 } from 'lucide-react';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Textarea } from '@/components/ui/textarea';
import { Separator } from '@/components/ui/separator';
import { agentService } from '@/lib/services/agentService';
import type { Agent, AgentRun, AgentRunArtifact, OrchestrationProposal, ProposedStory } from '@/lib/pmTypes';

interface Props {
  epicId: string;
  workspaceId: string;
  orchestratorAgentId?: string;
  onStoriesCreated?: () => void;
}

const STATUS_CONFIG: Record<string, { label: string; variant: 'default' | 'secondary' | 'destructive' | 'outline' }> = {
  queued: { label: 'Queued', variant: 'secondary' },
  running: { label: 'Running', variant: 'default' },
  awaiting_approval: { label: 'Review required', variant: 'secondary' },
  completed: { label: 'Completed', variant: 'outline' },
  failed: { label: 'Failed', variant: 'destructive' },
  cancelled: { label: 'Cancelled', variant: 'secondary' },
};

function parseProposal(artifacts: AgentRunArtifact[], run: AgentRun | null): OrchestrationProposal | null {
  const proposalArtifact = [...artifacts]
    .sort((a, b) => b.sequence_no - a.sequence_no)
    .find((artifact) => artifact.artifact_type === 'orchestration_proposal' && artifact.inline_content);

  const candidate = proposalArtifact?.inline_content ?? (run?.output_summary && 'proposed_stories' in run.output_summary ? JSON.stringify(run.output_summary) : null);
  if (!candidate) return null;

  try {
    const parsed = JSON.parse(candidate) as OrchestrationProposal;
    if (!Array.isArray(parsed.proposed_stories)) return null;
    return parsed;
  } catch {
    return null;
  }
}

export function EpicOrchestrationPanel({ epicId, workspaceId, orchestratorAgentId, onStoriesCreated }: Props) {
  const [agents, setAgents] = useState<Agent[]>([]);
  const [assignedAgentId, setAssignedAgentId] = useState(orchestratorAgentId ?? '');
  const [selectedAgentId, setSelectedAgentId] = useState(orchestratorAgentId ?? '');
  const [runs, setRuns] = useState<AgentRun[]>([]);
  const [selectedRunId, setSelectedRunId] = useState<string | null>(null);
  const [artifacts, setArtifacts] = useState<AgentRunArtifact[]>([]);
  const [editedStories, setEditedStories] = useState<ProposedStory[]>([]);
  const [additionalContext, setAdditionalContext] = useState('');
  const [assigning, setAssigning] = useState(false);
  const [triggering, setTriggering] = useState(false);
  const [confirming, setConfirming] = useState(false);
  const [actingOnRun, setActingOnRun] = useState<string | null>(null);
  const [loadingRuns, setLoadingRuns] = useState(true);

  useEffect(() => {
    setAssignedAgentId(orchestratorAgentId ?? '');
    setSelectedAgentId(orchestratorAgentId ?? '');
  }, [orchestratorAgentId]);

  const fetchAgents = useCallback(async () => {
    const res = await agentService.list(workspaceId);
    if (res.error) {
      toast.error(res.error);
      return;
    }
    const data = Array.isArray(res.data) ? res.data : [];
    setAgents(data.filter((agent) => agent.agent_kind === 'llm'));
  }, [workspaceId]);

  const fetchRuns = useCallback(async () => {
    setLoadingRuns(true);
    try {
      const res = await agentService.listEpicRuns(workspaceId, epicId);
      if (res.error) {
        toast.error(res.error);
        return;
      }
      const data = Array.isArray(res.data) ? res.data : [];
      setRuns(data);
      setSelectedRunId((current) => current ?? data[0]?.id ?? null);
    } finally {
      setLoadingRuns(false);
    }
  }, [workspaceId, epicId]);

  const loadArtifacts = useCallback(async (runId: string) => {
    const res = await agentService.listRunArtifacts(workspaceId, runId);
    if (res.error) {
      toast.error(res.error);
      return;
    }
    setArtifacts(Array.isArray(res.data) ? res.data : []);
  }, [workspaceId]);

  useEffect(() => {
    void fetchAgents();
  }, [fetchAgents]);

  useEffect(() => {
    void fetchRuns();
  }, [fetchRuns]);

  useEffect(() => {
    const active = runs.some((run) => ['queued', 'running', 'awaiting_approval'].includes(run.status));
    if (!active) return;
    const interval = window.setInterval(() => {
      void fetchRuns();
    }, 5000);
    return () => window.clearInterval(interval);
  }, [runs, fetchRuns]);

  useEffect(() => {
    if (!selectedRunId) {
      setArtifacts([]);
      return;
    }
    void loadArtifacts(selectedRunId);
  }, [selectedRunId, loadArtifacts]);

  const selectedRun = useMemo(
    () => runs.find((run) => run.id === selectedRunId) ?? null,
    [runs, selectedRunId],
  );

  const proposal = useMemo(
    () => parseProposal(artifacts, selectedRun),
    [artifacts, selectedRun],
  );

  useEffect(() => {
    if (proposal?.proposed_stories) {
      setEditedStories(proposal.proposed_stories);
    } else {
      setEditedStories([]);
    }
  }, [proposal?.epic_id, proposal?.summary, proposal?.tokens_used, proposal?.proposed_stories]);

  const assignAgent = useCallback(async () => {
    if (!selectedAgentId) {
      toast.error('Choose an orchestrator agent first');
      return false;
    }
    setAssigning(true);
    try {
      const res = await agentService.assignOrchestrator(workspaceId, epicId, selectedAgentId);
      if (res.error) {
        toast.error(res.error);
        return false;
      }
      setAssignedAgentId(selectedAgentId);
      toast.success('Orchestrator assigned');
      return true;
    } finally {
      setAssigning(false);
    }
  }, [selectedAgentId, workspaceId, epicId]);

  const handleRun = useCallback(async () => {
    let effectiveAgentId = assignedAgentId;
    if (!effectiveAgentId) {
      const assigned = await assignAgent();
      if (!assigned) return;
      effectiveAgentId = selectedAgentId;
    }

    if (!effectiveAgentId) {
      toast.error('Assign an orchestrator first');
      return;
    }

    setTriggering(true);
    try {
      const res = await agentService.runEpicAgent(workspaceId, epicId, additionalContext);
      if (res.error) {
        toast.error(res.error);
        return;
      }
      toast.success('Epic orchestration run started');
      setAdditionalContext('');
      await fetchRuns();
      if (res.data?.id) {
        setSelectedRunId(res.data.id);
      }
    } finally {
      setTriggering(false);
    }
  }, [additionalContext, assignedAgentId, assignAgent, epicId, fetchRuns, selectedAgentId, workspaceId]);

  const handleCancelRun = useCallback(async (runId: string) => {
    setActingOnRun(runId);
    try {
      const res = await agentService.cancelRun(workspaceId, runId);
      if (res.error) {
        toast.error(res.error);
        return;
      }
      toast.success('Run cancelled');
      await fetchRuns();
    } finally {
      setActingOnRun(null);
    }
  }, [fetchRuns, workspaceId]);

  const handleConfirm = useCallback(async () => {
    if (!selectedRun) {
      toast.error('Select an orchestration run first');
      return;
    }
    if (!editedStories.length) {
      toast.error('No proposed stories to create');
      return;
    }

    setConfirming(true);
    try {
      const res = await agentService.confirmOrchestrationRun(workspaceId, selectedRun.id, editedStories);
      if (res.error) {
        toast.error(res.error);
        return;
      }
      toast.success(`Created ${editedStories.length} stories`);
      await fetchRuns();
      await loadArtifacts(selectedRun.id);
      onStoriesCreated?.();
    } finally {
      setConfirming(false);
    }
  }, [editedStories, fetchRuns, loadArtifacts, onStoriesCreated, selectedRun, workspaceId]);

  const updateStory = useCallback((index: number, field: keyof ProposedStory, value: string | number) => {
    setEditedStories((current) =>
      current.map((story, storyIndex) => (storyIndex === index ? { ...story, [field]: value } : story)),
    );
  }, []);

  const removeStory = useCallback((index: number) => {
    setEditedStories((current) => current.filter((_, storyIndex) => storyIndex !== index));
  }, []);

  const otherArtifacts = useMemo(
    () => artifacts.filter((artifact) => artifact.artifact_type !== 'orchestration_proposal'),
    [artifacts],
  );

  return (
    <div className="mt-6">
      <Separator className="mb-6" />
      <div className="flex items-center gap-2 mb-4">
        <Sparkles className="h-4 w-4 text-purple-500" />
        <h3 className="text-sm font-semibold">Epic Orchestration</h3>
      </div>

      <div className="rounded-md border border-border/60 bg-muted/20 p-3 space-y-3">
        <p className="text-xs text-muted-foreground">
          The orchestrator now runs through the shared agent-run pipeline, produces a proposal artifact, and waits for review before stories are created.
        </p>
        <div className="flex items-center gap-2">
          <select
            value={selectedAgentId}
            onChange={(event) => setSelectedAgentId(event.target.value)}
            className="h-8 rounded-md border border-border bg-background px-2 text-xs flex-1"
          >
            <option value="">Select orchestrator...</option>
            {agents.map((agent) => (
              <option key={agent.id} value={agent.id}>
                {agent.name} ({agent.capability_profile || agent.role || 'agent'})
              </option>
            ))}
          </select>
          <Button size="sm" variant="outline" onClick={() => void assignAgent()} disabled={!selectedAgentId || assigning}>
            {assigning ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Bot className="h-3.5 w-3.5" />}
            Assign
          </Button>
        </div>

        <Textarea
          value={additionalContext}
          onChange={(event) => setAdditionalContext(event.target.value)}
          placeholder="Additional context or constraints for this orchestration run..."
          className="text-xs min-h-[72px]"
        />

        <div className="flex items-center gap-2">
          <Button onClick={() => void handleRun()} disabled={triggering} className="gap-1.5" size="sm">
            {triggering ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Play className="h-3.5 w-3.5" />}
            {triggering ? 'Starting...' : 'Run Orchestrator'}
          </Button>
          {assignedAgentId ? (
            <Badge variant="outline" className="text-[10px]">assigned</Badge>
          ) : (
            <Badge variant="secondary" className="text-[10px]">no orchestrator assigned</Badge>
          )}
        </div>
      </div>

      <div className="mt-4 space-y-2">
        <div className="flex items-center gap-2">
          <Clock className="h-4 w-4 text-muted-foreground" />
          <h4 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">Runs</h4>
        </div>

        {loadingRuns ? (
          <div className="flex items-center gap-2 py-4 text-xs text-muted-foreground">
            <Loader2 className="h-3.5 w-3.5 animate-spin" />
            Loading orchestration runs...
          </div>
        ) : runs.length === 0 ? (
          <p className="py-2 text-xs text-muted-foreground">No orchestration runs yet.</p>
        ) : (
          <div className="space-y-2">
            {runs.map((run) => {
              const config = STATUS_CONFIG[run.status] ?? STATUS_CONFIG.queued;
              const selected = selectedRunId === run.id;
              return (
                <div key={run.id} className={`rounded-md border px-3 py-2 ${selected ? 'border-primary bg-accent/30' : 'border-border/60'}`}>
                  <button type="button" className="w-full text-left" onClick={() => setSelectedRunId(run.id)}>
                    <div className="flex items-center justify-between gap-3">
                      <div className="flex items-center gap-2">
                        <Badge variant={config.variant} className="text-[10px]">{config.label}</Badge>
                        <span className="text-[10px] text-muted-foreground">{run.runtime_kind}</span>
                        {run.runner_pool && <span className="text-[10px] text-muted-foreground">pool: {run.runner_pool}</span>}
                      </div>
                      <span className="text-[10px] text-muted-foreground">{run.tokens_used > 0 ? `${run.tokens_used.toLocaleString()} tokens` : 'no tokens yet'}</span>
                    </div>
                    <div className="mt-1 flex flex-wrap gap-2 text-[10px] text-muted-foreground">
                      {run.execution_stage && <span>stage: {run.execution_stage}</span>}
                      <span>approval: {run.approval_state}</span>
                      {run.error_message && <span className="text-destructive">{run.error_message}</span>}
                    </div>
                  </button>

                  {selected && (
                    <div className="mt-2 flex flex-wrap gap-2">
                      {['queued', 'running', 'awaiting_approval'].includes(run.status) && (
                        <Button
                          size="sm"
                          variant="outline"
                          className="h-7 gap-1 text-[11px]"
                          disabled={actingOnRun === run.id}
                          onClick={() => void handleCancelRun(run.id)}
                        >
                          {actingOnRun === run.id ? <Loader2 className="h-3 w-3 animate-spin" /> : <StopCircle className="h-3 w-3" />}
                          Cancel
                        </Button>
                      )}
                      {run.approval_state === 'pending' && proposal && (
                        <Button
                          size="sm"
                          className="h-7 gap-1 text-[11px]"
                          disabled={confirming}
                          onClick={() => void handleConfirm()}
                        >
                          {confirming ? <Loader2 className="h-3 w-3 animate-spin" /> : <Check className="h-3 w-3" />}
                          Create Stories
                        </Button>
                      )}
                    </div>
                  )}
                </div>
              );
            })}
          </div>
        )}
      </div>

      {selectedRun && proposal && (
        <div className="mt-4 space-y-3 rounded-md border border-border/60 p-3">
          <div className="flex items-center justify-between gap-3">
            <div>
              <h4 className="text-sm font-medium">Proposal</h4>
              <p className="text-xs text-muted-foreground">
                {editedStories.length} proposed stories · {proposal.tokens_used.toLocaleString()} tokens
              </p>
            </div>
            {selectedRun.approval_state === 'pending' ? (
              <Badge variant="secondary" className="gap-1 text-[10px]">
                <ShieldCheck className="h-3 w-3" />
                Awaiting review
              </Badge>
            ) : (
              <Badge variant="outline" className="text-[10px]">Reviewed</Badge>
            )}
          </div>

          {proposal.summary && (
            <p className="rounded bg-muted/30 p-2 text-xs text-muted-foreground">{proposal.summary}</p>
          )}

          <div className="space-y-2">
            {editedStories.map((story, index) => (
              <div key={`${index}-${story.name}`} className="rounded-md border border-border/60 p-3 space-y-2">
                <div className="flex items-start justify-between gap-2">
                  <input
                    value={story.name}
                    onChange={(event) => updateStory(index, 'name', event.target.value)}
                    className="flex-1 bg-transparent text-sm font-medium outline-none"
                  />
                  <div className="flex items-center gap-1 shrink-0">
                    <Badge variant="secondary" className="text-[10px]">{story.story_type || 'feature'}</Badge>
                    {story.estimate != null && <Badge variant="outline" className="text-[10px]">{story.estimate}pt</Badge>}
                    <Button variant="ghost" size="icon" className="h-6 w-6" onClick={() => removeStory(index)}>
                      <Trash2 className="h-3 w-3 text-muted-foreground" />
                    </Button>
                  </div>
                </div>
                <textarea
                  value={story.description}
                  onChange={(event) => updateStory(index, 'description', event.target.value)}
                  className="w-full resize-none bg-transparent text-xs text-muted-foreground outline-none min-h-[44px]"
                />
              </div>
            ))}
          </div>
        </div>
      )}

      {selectedRun && otherArtifacts.length > 0 && (
        <div className="mt-4 space-y-2">
          <div className="flex items-center gap-2">
            <FileText className="h-4 w-4 text-muted-foreground" />
            <h4 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">Artifacts</h4>
          </div>
          {otherArtifacts.map((artifact) => (
            <div key={artifact.id} className="rounded border border-border/60 bg-muted/20 p-2">
              <div className="mb-1 text-[11px] font-medium">
                {artifact.artifact_type.replace(/_/g, ' ')} <span className="text-muted-foreground">({artifact.format})</span>
              </div>
              {artifact.inline_content && (
                <pre className="max-h-32 overflow-auto whitespace-pre-wrap break-all text-[10px] text-muted-foreground">
                  {artifact.inline_content.slice(0, 2000)}
                  {artifact.inline_content.length > 2000 ? '...' : ''}
                </pre>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
