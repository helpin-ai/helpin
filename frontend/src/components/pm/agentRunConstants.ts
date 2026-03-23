import type React from 'react';

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
  awaiting_input: { label: 'Awaiting input', variant: 'secondary' },
  awaiting_approval: { label: 'Awaiting approval', variant: 'secondary' },
  completed: { label: 'Completed', variant: 'outline' },
  failed: { label: 'Failed', variant: 'destructive' },
  cancelled: { label: 'Cancelled', variant: 'secondary' },
};

export const ACTIVE_RUN_STATUSES = new Set(['queued', 'running', 'awaiting_input', 'awaiting_approval']);

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
  git_persistence_result: 'Git Result',
};
