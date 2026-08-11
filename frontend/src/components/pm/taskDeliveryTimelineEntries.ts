import type { AgentRun, TaskGitLink } from '@/lib/pmTypes';

export type TaskDeliveryTimelineEntry =
  | { id: string; kind: 'run'; occurredAt: string; run: AgentRun }
  | { id: string; kind: 'git'; occurredAt: string; link: TaskGitLink };

export function buildTaskDeliveryTimelineEntries(runs: AgentRun[], links: TaskGitLink[]): TaskDeliveryTimelineEntry[] {
  return [
    ...runs.map((run): TaskDeliveryTimelineEntry => ({
      id: `run:${run.id}`,
      kind: 'run',
      occurredAt: run.completed_at ?? run.updated_at ?? run.created_at,
      run,
    })),
    ...links.map((link): TaskDeliveryTimelineEntry => ({
      id: `git:${link.id}`,
      kind: 'git',
      occurredAt: link.updated_at,
      link,
    })),
  ].sort((left, right) => Date.parse(right.occurredAt) - Date.parse(left.occurredAt));
}
