import type { AgentRunStatus, AgentRunPauseReason } from '@/lib/pmTypes';
import type { DockRunSummary } from '@/lib/dockTypes';

export type DockRunGroup = 'needs_you' | 'running' | 'recent';
export type DockRunTone = 'attention' | 'running' | 'paused' | 'failed' | 'done' | 'cancelled' | 'queued';

export interface DockRunPresentation {
  group: DockRunGroup;
  tone: DockRunTone;
  label: string;
  dot: string;
  chipBackground: string;
  chipForeground: string;
}

export function presentDockRun(
  status: AgentRunStatus,
  pauseReason: AgentRunPauseReason,
  attentionKind?: DockRunSummary['attention_kind'],
): DockRunPresentation {
  if (attentionKind === 'approval' || (status === 'paused' && pauseReason === 'human_approval')) {
    return attentionPresentation('Approve');
  }
  if (attentionKind === 'authentication' || (status === 'paused' && pauseReason === 'authentication')) {
    return attentionPresentation('Sign in');
  }
  if (attentionKind === 'input' || (status === 'paused' && pauseReason === 'human_input')) {
    return attentionPresentation('Needs you');
  }
  switch (status) {
    case 'queued':
      return {
        group: 'running', tone: 'queued', label: 'Queued', dot: '#94a3b8',
        chipBackground: '#f2f4f6', chipForeground: '#5b6674',
      };
    case 'running':
      return {
        group: 'running', tone: 'running', label: 'Running', dot: '#059669',
        chipBackground: '#eafaf2', chipForeground: '#047857',
      };
    case 'paused':
      return {
        group: 'recent', tone: 'paused', label: 'Paused', dot: '#94a3b8',
        chipBackground: '#f2f4f6', chipForeground: '#5b6674',
      };
    case 'failed':
      return {
        group: 'recent', tone: 'failed', label: 'Failed', dot: '#dc2626',
        chipBackground: '#fdf0ef', chipForeground: '#b91c1c',
      };
    case 'cancelled':
      return {
        group: 'recent', tone: 'cancelled', label: 'Cancelled', dot: '#a5a29b',
        chipBackground: '#f4f2ee', chipForeground: '#6b6862',
      };
    case 'completed':
    default:
      return {
        group: 'recent', tone: 'done', label: 'Done', dot: '#a5a29b',
        chipBackground: '#f4f2ee', chipForeground: '#6b6862',
      };
  }
}

function attentionPresentation(label: string): DockRunPresentation {
  return {
    group: 'needs_you', tone: 'attention', label, dot: '#d97706',
    chipBackground: '#fff7ea', chipForeground: '#b45309',
  };
}

export function dockRunTitle(summary: DockRunSummary): string {
  const target = summary.run.target_info;
  const title = target?.title?.trim();
  const key = target?.task_key?.trim();
  if (key && title) return `${key} · ${title}`;
  return title || key || `${humanizeTarget(summary.run.target_type)} agent run`;
}

export function dockRunSubtitle(summary: DockRunSummary): string {
  const run = summary.run;
  const presentation = presentDockRun(run.status, run.pause_reason, summary.attention_kind);
  if (run.execution_stage?.trim()) return `${summary.agent.name || 'Agent'} · ${run.execution_stage.trim()}`;
  return `${summary.agent.name || 'Agent'} · ${presentation.label}`;
}

export function dockRunContext(summary: DockRunSummary): string {
  const run = summary.run;
  if (run.repo_full_name) {
    const branch = run.working_branch || run.base_branch;
    return branch ? `${run.repo_full_name} · ${branch}` : run.repo_full_name;
  }
  const target = run.target_info;
  if (target?.task_key) return target.task_key;
  return humanizeTarget(run.target_type);
}

function humanizeTarget(value: string): string {
  return value.replaceAll('_', ' ').replace(/\b\w/g, (letter) => letter.toUpperCase());
}

export function relativeDockTime(value?: string | null, now = Date.now()): string {
  if (!value) return '';
  const timestamp = Date.parse(value);
  if (!Number.isFinite(timestamp)) return '';
  const seconds = Math.max(0, Math.round((now - timestamp) / 1000));
  if (seconds < 45) return 'now';
  const minutes = Math.round(seconds / 60);
  if (minutes < 60) return `${minutes}m`;
  const hours = Math.round(minutes / 60);
  if (hours < 24) return `${hours}h`;
  const days = Math.round(hours / 24);
  return `${days}d`;
}
