import type React from 'react';
import type { AgentRun, AgentRunPauseReason } from '@/lib/pmTypes';

export interface StatusConfig {
  label: string;
  icon: React.ReactNode;
  variant: 'default' | 'secondary' | 'destructive' | 'outline';
}

// Populated at runtime by AgentRunTable/AgentRunDetail to avoid importing lucide here.
// Instead, each consumer builds its own icon inline and references this for label+variant.

export const STATUS_META: Record<string, { label: string; variant: 'default' | 'secondary' | 'destructive' | 'outline' }> = {
  queued: { label: 'Queued', variant: 'secondary' },
  running: { label: 'Running', variant: 'default' },
  paused: { label: 'Paused', variant: 'secondary' },
  awaiting_input: { label: 'Awaiting input', variant: 'secondary' },
  awaiting_approval: { label: 'Awaiting approval', variant: 'secondary' },
  completed: { label: 'Completed', variant: 'outline' },
  failed: { label: 'Failed', variant: 'destructive' },
  cancelled: { label: 'Cancelled', variant: 'secondary' },
};

export const ACTIVE_RUN_STATUSES = new Set(['queued', 'running', 'paused']);

export function getAgentRunPauseReason(run: Pick<AgentRun, 'status' | 'pause_reason' | 'approval_state'>): AgentRunPauseReason {
  if (run.pause_reason && run.pause_reason !== 'none') return run.pause_reason;
  if (run.status === 'paused' && run.approval_state === 'pending') return 'human_approval';
  if (run.status === 'paused') return 'human_input';
  return 'none';
}

export function getAgentRunDisplayStatus(run: Pick<AgentRun, 'status' | 'pause_reason' | 'approval_state'>): string {
  if (run.status === 'paused') {
    return getAgentRunPauseReason(run) === 'human_approval' ? 'awaiting_approval' : 'awaiting_input';
  }
  return run.status;
}

export function isPausedAgentRun(run: Pick<AgentRun, 'status' | 'pause_reason' | 'approval_state'> | null | undefined): boolean {
  if (!run) return false;
  return getAgentRunDisplayStatus(run) === 'awaiting_input' || getAgentRunDisplayStatus(run) === 'awaiting_approval';
}

export const ARTIFACT_TYPE_LABELS: Record<string, string> = {
  conversation_log: 'Conversation Log',
  tool_log: 'Tool Log',
  diff: 'Diff',
  test_report: 'Test Report',
  pr_metadata: 'PR Metadata',
  agent_summary: 'Agent Summary',
  file_bundle: 'File Bundle',
  handoff_note: 'Handoff Note',
  opencode_config: 'Config',
  opencode_stdout: 'Output (stdout)',
  opencode_stderr: 'Output (stderr)',
  codex_config: 'Config',
  codex_prompt: 'Prompt',
  codex_response: 'Response',
  codex_stdout: 'Output (stdout)',
  codex_stderr: 'Output (stderr)',
  git_status: 'Git Status',
  git_diff_stat: 'Git Diff Stat',
  git_persistence_result: 'Git Result',
};
