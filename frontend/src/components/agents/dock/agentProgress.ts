import { describeToolCall } from '@/components/pm/CodingSession/toolCallPresentation';
import { resolveVisibleTurn } from './agentTurnState';
import { isRuntimeControlToolName } from '@/lib/toolNames';
import type { AgentRun, CodingSessionStreamState, CommandBarPlanSummary, RunPlanArtifact } from '@/lib/pmTypes';

export interface AgentLiveProgress {
  label: string;
  startedAt?: string;
  tone: 'working' | 'waiting';
  completed?: boolean;
  delegated?: boolean;
}

interface ResolveAgentLiveProgressInput {
  run: Pick<AgentRun, 'status' | 'pause_reason' | 'execution_stage' | 'started_at' | 'created_at'> | null;
  stream: Pick<CodingSessionStreamState, 'transcript_messages' | 'live_turn_segments' | 'live_reasoning_message' | 'activity_events' | 'turn_state'> | null;
  currentPlan: RunPlanArtifact | null;
  delegatedPlans?: CommandBarPlanSummary[];
  sending: boolean;
  localStartedAt?: string;
}

function activePlanStep(plan: RunPlanArtifact | null): string | null {
  const step = plan?.plan?.find((candidate) => candidate.status === 'in_progress')?.step?.trim();
  return step || null;
}

function delegatedProgress(plans: CommandBarPlanSummary[]): AgentLiveProgress | null {
  const children = plans.filter(plan => plan.status === 'running').flatMap(plan =>
    Object.entries(plan.run_ids_by_step ?? {}).flatMap(([index, id]) => {
      const child = plan.runs?.find(run => run.id === id);
      return child && ['queued', 'running', 'paused'].includes(child.status)
        ? [{ name: plan.steps[Number(index)]?.agent_name?.trim() || 'Sub-agent', run: child }] : [];
    }));
  if (!children.length) {
    return plans.some(plan => plan.status === 'running' && !plan.runs?.length)
      ? { label: 'Starting sub-agent…', tone: 'working', delegated: true } : null;
  }
  const blockers: Partial<Record<AgentRun['pause_reason'], string>> = {
    human_approval: 'Approval needed', human_input: 'Your input needed', authentication: 'Sign-in needed',
  };
  const active = children.some(child => child.run.status !== 'paused');
  let label: string;
  if (children.length === 1) {
    const { name, run } = children[0];
    const state = run.status === 'running' ? 'is running' : run.status === 'queued' ? 'is queued'
      : run.pause_reason === 'human_approval' ? 'needs approval'
      : run.pause_reason === 'human_input' ? 'needs your input'
      : run.pause_reason === 'authentication' ? 'needs sign-in'
      : run.pause_reason === 'manual' ? 'is paused' : 'is waiting';
    label = `${name} ${state}`;
  } else {
    const blocker = children.filter(child => child.run.status === 'paused')
      .map(child => blockers[child.run.pause_reason]).find(Boolean);
    label = `${children.length} sub-agents ${active ? 'active' : 'waiting'}${blocker ? ` · ${blocker}` : ''}`;
  }
  return { label, tone: active ? 'working' : 'waiting', delegated: true };
}

/**
 * Turns noisy runtime state into one calm, user-facing line. This deliberately
 * contains no controls: the composer owns Pause, Resume, Stop, and Send.
 */
export function resolveAgentLiveProgress({
  run,
  stream,
  currentPlan,
  delegatedPlans = [],
  sending,
  localStartedAt,
}: ResolveAgentLiveProgressInput): AgentLiveProgress | null {
  // A follow-up message can reuse the same backing run. Prefer the local
  // submission timestamp so each user turn gets an independent timer instead
  // of inheriting the run's original start time.
  const turn = resolveVisibleTurn(stream, localStartedAt);
  const startedAt = turn.startedAt || run?.started_at || run?.created_at;

  if (sending) {
    return { label: 'Starting…', startedAt, tone: 'working' };
  }
  if (!run) return null;
  if (run.execution_stage === 'pausing') {
    return { label: 'Pausing…', startedAt, tone: 'waiting' };
  }
  if (run.status === 'paused' && run.pause_reason === 'manual') {
    return { label: 'Paused', startedAt, tone: 'waiting' };
  }
  // Finishing the parent's reply does not finish delegated work.
  const delegated = delegatedProgress(delegatedPlans);
  if (delegated) {
    const waiting = run.status === 'completed'
      || (run.status === 'paused' && run.pause_reason === 'awaiting_user_message');
    return { ...delegated, startedAt, label: `${delegated.label}${waiting ? ' · Ask Agent is waiting' : ''}` };
  }
  if (turn.answered) return null;
  if (turn.answerPending) return { label: 'Loading answer…', startedAt, tone: 'waiting' };
  if (turn.missingAnswer) return { label: 'Run ended without a final answer', startedAt, tone: 'waiting' };

  if (run.status === 'paused') {
    switch (run.pause_reason) {
      case 'human_approval':
        return { label: 'Waiting for approval', startedAt, tone: 'waiting' };
      case 'human_input':
        return { label: 'Waiting for your reply', startedAt, tone: 'waiting' };
      case 'awaiting_user_message':
        return null;
      case 'authentication':
        return { label: 'Waiting for sign-in', startedAt, tone: 'waiting' };
      default:
        return null;
    }
  }

  if (run.status === 'completed') {
    return { label: 'Worked', startedAt, tone: 'waiting', completed: true };
  }

  if (run.status === 'queued') return { label: 'Waiting to start…', startedAt, tone: 'working' };
  if (run.status !== 'running') return null;

  const segments = stream?.live_turn_segments ?? [];
  for (let index = segments.length - 1; index >= 0; index -= 1) {
    const segment = segments[index];
    if (segment.kind !== 'tool_call') continue;
    if (isRuntimeControlToolName(segment.tool_call.tool_name)) continue;
    if (segment.tool_call.status === 'running') {
      return { label: `${describeToolCall(segment.tool_call).primaryLabel}…`, startedAt, tone: 'working' };
    }
    break;
  }

  const lastSegment = segments[segments.length - 1];
  const step = activePlanStep(currentPlan);
  if (step) return { label: step, startedAt, tone: 'working' };

  if (
    lastSegment?.kind === 'assistant_message'
    && lastSegment.assistant_message.status === 'streaming'
    && lastSegment.assistant_message.content.trim()
  ) {
    return { label: 'Preparing a response…', startedAt, tone: 'working' };
  }
  if (stream?.live_reasoning_message?.status === 'streaming') {
    return { label: 'Thinking…', startedAt, tone: 'working' };
  }
  if (
    segments.length === 0
    && !stream?.live_reasoning_message
    && !currentPlan
    && (stream?.activity_events?.length ?? 0) === 0
  ) {
    return { label: 'Working…', startedAt, tone: 'working' };
  }
  return { label: 'Working…', startedAt, tone: 'working' };
}

export function formatAgentElapsed(startedAt: string | undefined, now = Date.now(), pausedMs = 0): string | null {
  if (!startedAt) return null;
  const started = Date.parse(startedAt);
  if (!Number.isFinite(started)) return null;
  const totalSeconds = Math.max(0, Math.floor((now - started - pausedMs) / 1_000));
  if (totalSeconds < 60) return `${totalSeconds}s`;
  const hours = Math.floor(totalSeconds / 3_600);
  const minutes = Math.floor((totalSeconds % 3_600) / 60);
  const seconds = totalSeconds % 60;
  if (hours > 0) return `${hours}h ${minutes.toString().padStart(2, '0')}m`;
  return `${minutes}m ${seconds.toString().padStart(2, '0')}s`;
}
