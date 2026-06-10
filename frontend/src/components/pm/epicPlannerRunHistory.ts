import { formatDistance, parseISO } from 'date-fns';

import { outputSummaryText } from '@/components/agents/dock/utils';
import type { AgentRun } from '@/lib/pmTypes';

/**
 * Command-bar runs (the "Command Agent" / DAG orchestration shells dispatched
 * from the Ask-agents dock) target the epic but are not planner runs — they
 * carry no transcript of their own and are surfaced in the command-bar dock,
 * not here. They're identified canonically by their trigger source, not by the
 * agent name (a DAG can use any agent). See backend
 * model.AgentRunTriggerSourceCommandBar.
 */
export function isCommandBarRun(run: Pick<AgentRun, 'input'>): boolean {
  const trigger = (run.input as { trigger?: { source?: string } } | null | undefined)?.trigger;
  return trigger?.source === 'command_bar';
}

export const TERMINAL_RUN_STATUSES = new Set(['completed', 'failed', 'cancelled']);
export const RUN_DECAY_AGE_MS = 48 * 60 * 60 * 1000;
export const HISTORY_VISIBLE_ROW_LIMIT = 5;

export interface HistoryRunGroup {
  kind: 'single' | 'collapsed';
  agentId: string;
  runs: AgentRun[];
}

export function groupHistoryRuns(runs: AgentRun[]): HistoryRunGroup[] {
  const groups: HistoryRunGroup[] = [];
  let buffer: AgentRun[] = [];

  const flush = () => {
    if (buffer.length === 0) return;
    groups.push({
      kind: buffer.length >= 2 ? 'collapsed' : 'single',
      agentId: buffer[0].agent_id,
      runs: buffer,
    });
    buffer = [];
  };

  for (const run of runs) {
    if (!TERMINAL_RUN_STATUSES.has(run.status)) {
      flush();
      groups.push({ kind: 'single', agentId: run.agent_id, runs: [run] });
      continue;
    }
    if (buffer.length > 0 && buffer[0].agent_id !== run.agent_id) {
      flush();
    }
    buffer.push(run);
  }
  flush();

  return groups;
}

export function isDecayedRun(
  run: Pick<AgentRun, 'status' | 'created_at'>,
  now: Date = new Date(),
): boolean {
  if (run.status !== 'failed' && run.status !== 'cancelled') return false;
  try {
    return now.getTime() - parseISO(run.created_at).getTime() > RUN_DECAY_AGE_MS;
  } catch {
    return false;
  }
}

export function runResultSummary(run: AgentRun): string {
  if (run.status === 'failed' && run.error_message?.trim()) {
    return run.error_message.trim();
  }
  return outputSummaryText(run);
}

export function compactRelativeAge(iso: string, now: Date = new Date()): string {
  try {
    const elapsedMs = now.getTime() - parseISO(iso).getTime();
    if (Number.isNaN(elapsedMs)) return '';
    const minutes = Math.max(0, Math.floor(elapsedMs / 60_000));
    if (minutes < 60) return `${minutes}m`;
    const hours = Math.floor(minutes / 60);
    if (hours < 48) return `${hours}h`;
    return `${Math.floor(hours / 24)}d`;
  } catch {
    return '';
  }
}

export function historyGroupTimeLabel(group: HistoryRunGroup, now: Date = new Date()): string {
  if (group.runs.length === 0) return '';
  if (group.kind === 'single' || group.runs.length === 1) {
    try {
      return formatDistance(parseISO(group.runs[0].created_at), now, { addSuffix: true });
    } catch {
      return '';
    }
  }
  const newest = compactRelativeAge(group.runs[0].created_at, now);
  const oldest = compactRelativeAge(group.runs[group.runs.length - 1].created_at, now);
  if (!newest && !oldest) return '';
  if (newest === oldest) return `${newest} ago`;
  return `${newest}–${oldest} ago`;
}
