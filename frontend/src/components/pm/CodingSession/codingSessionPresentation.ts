import type {
  AgentRunPauseReason,
  AgentRunStatus,
  CodingSessionTranscriptMessage,
} from '@/lib/pmTypes';

export const EXECUTION_STAGE_LABELS: Record<string, string> = {
  queued: 'Queued',
  preparing: 'Preparing workspace',
  starting: 'Starting runtime',
  codex_starting: 'Starting runtime',
  opencode_starting: 'Starting runtime',
  native_sdk_starting: 'Starting runtime',
  continuing: 'Continuing run',
  resuming: 'Resuming',
  approved: 'Resuming',
  feedback_received: 'Resuming',
  input_received: 'Resuming',
  auth_completed: 'Resuming',
  codex_running: 'Agent working',
  opencode_running: 'Agent working',
  native_sdk_running: 'Agent working',
  awaiting_approval: 'Awaiting approval',
  awaiting_review: 'Awaiting review',
  awaiting_input: 'Awaiting input',
  awaiting_auth: 'Awaiting sign-in',
  finishing: 'Wrapping up',
  completed: 'Completed',
  failed: 'Failed',
  failed_to_start: 'Failed to start',
  cancelled: 'Cancelled',
};

export function codingSessionStageLabel(stage?: string | null): string {
  const key = stage?.trim();
  if (!key) return 'Agent working';
  return EXECUTION_STAGE_LABELS[key] ?? 'Agent working';
}

export function codingSessionStatusLabel({
  status,
  pauseReason,
  executionStage,
}: {
  status?: AgentRunStatus | string | null;
  pauseReason?: AgentRunPauseReason | string | null;
  executionStage?: string | null;
}): string {
  if (status === 'queued') return 'Queued';
  if (status === 'paused') {
    if (pauseReason === 'human_approval') {
      if (executionStage === 'awaiting_review') return 'Awaiting review';
      return 'Awaiting approval';
    }
    if (pauseReason === 'awaiting_user_message') return 'Awaiting reply';
    if (pauseReason === 'authentication') return 'Awaiting sign-in';
    return 'Awaiting input';
  }
  if (status === 'completed') return 'Completed';
  if (status === 'failed') return 'Failed';
  if (status === 'cancelled') return 'Cancelled';
  if (status === 'running') return codingSessionStageLabel(executionStage);
  return 'Loading';
}

export function isPromptTranscriptMessage(message: Pick<CodingSessionTranscriptMessage, 'message_type'>): boolean {
  return message.message_type === 'system_prompt' || message.message_type === 'developer_prompt';
}

export function isStatusTranscriptMessage(message: Pick<CodingSessionTranscriptMessage, 'message_type'>): boolean {
  return message.message_type === 'status';
}

export function isVisibleConversationTurn(message: CodingSessionTranscriptMessage): boolean {
  return (
    !isPromptTranscriptMessage(message)
    && !isStatusTranscriptMessage(message)
    && (message.role === 'assistant' || message.role === 'user')
    && message.content.trim().length > 0
  );
}

export function countVisibleTranscriptTurns(messages: CodingSessionTranscriptMessage[]): number {
  return messages.filter(isVisibleConversationTurn).length;
}

export function formatCodingSessionElapsed(ms: number): string {
  const totalSec = Math.max(0, Math.floor(ms / 1000));
  const d = Math.floor(totalSec / 86400);
  const h = Math.floor((totalSec % 86400) / 3600);
  const m = Math.floor((totalSec % 3600) / 60);
  const s = totalSec % 60;
  if (d > 0) return h > 0 ? `${d}d ${h}h` : `${d}d`;
  if (h > 0) return `${h}h ${m}m`;
  if (m > 0) return `${m}m ${s.toString().padStart(2, '0')}s`;
  return `${s}s`;
}
