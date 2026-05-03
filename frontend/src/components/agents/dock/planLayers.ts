import type { CommandBarPlan, CommandBarPlanStep } from '@/lib/pmTypes';

export interface TaskNode {
  /** Stable identity — `target.entity_id` if present, else `step-${i}`. */
  key: string;
  title: string;
  /**
   * Human task key like `HLP-60` if the planner attached it via
   * `target.metadata.task_key`. Used to render `#60` style row prefixes.
   */
  taskKey?: string;
  entityType?: CommandBarPlanStep['target']['entity_type'];
  /**
   * Step indexes into `plan.steps`, ordered topologically by intra-task
   * `depends_on_step_indexes` so dependents come after their deps.
   */
  stepIndexes: number[];
  /** Upstream task `key`s this group waits on (cross-task edges only). */
  depTaskKeys: Set<string>;
}

export interface PlanLayer {
  index: number;
  tasks: TaskNode[];
}

function stepKey(step: CommandBarPlanStep, index: number): string {
  return step.target?.entity_id ?? `step-${index}`;
}

export function buildTaskNodes(plan: CommandBarPlan): TaskNode[] {
  const indexToKey = plan.steps.map((s, i) => stepKey(s, i));

  const groups = new Map<string, TaskNode>();
  plan.steps.forEach((step, i) => {
    const key = indexToKey[i];
    const existing = groups.get(key);
    if (existing) {
      existing.stepIndexes.push(i);
      return;
    }
    const taskKey = (() => {
      const meta = step.target?.metadata as Record<string, unknown> | undefined;
      const candidate = meta?.task_key;
      return typeof candidate === 'string' && candidate.trim().length > 0
        ? candidate
        : undefined;
    })();
    groups.set(key, {
      key,
      title: step.target?.display_title ?? key,
      taskKey,
      entityType: step.target?.entity_type,
      stepIndexes: [i],
      depTaskKeys: new Set(),
    });
  });

  // Cross-task dependency edges + intra-task ordering.
  for (const node of groups.values()) {
    // Build adjacency for intra-task topo sort.
    const intraDeps = new Map<number, number[]>();
    for (const i of node.stepIndexes) {
      const deps = plan.steps[i].depends_on_step_indexes ?? [];
      const intra: number[] = [];
      for (const d of deps) {
        const depKey = indexToKey[d];
        if (depKey === node.key) intra.push(d);
        else if (depKey != null) node.depTaskKeys.add(depKey);
      }
      intraDeps.set(i, intra);
    }
    node.stepIndexes = topoSortIndexes(node.stepIndexes, intraDeps);
  }

  return Array.from(groups.values());
}

function topoSortIndexes(
  indexes: number[],
  deps: Map<number, number[]>,
): number[] {
  const remaining = new Set(indexes);
  const out: number[] = [];
  const indegree = new Map<number, number>();
  for (const i of indexes) indegree.set(i, (deps.get(i) ?? []).length);

  while (remaining.size > 0) {
    let progressed = false;
    for (const i of indexes) {
      if (!remaining.has(i)) continue;
      if ((indegree.get(i) ?? 0) === 0) {
        out.push(i);
        remaining.delete(i);
        progressed = true;
        for (const j of indexes) {
          if (!remaining.has(j)) continue;
          if ((deps.get(j) ?? []).includes(i)) {
            indegree.set(j, (indegree.get(j) ?? 1) - 1);
          }
        }
      }
    }
    if (!progressed) {
      // Cycle within a single task — preserve original order for the rest.
      for (const i of indexes) if (remaining.has(i)) out.push(i);
      break;
    }
  }
  return out;
}

export function layerTasks(nodes: TaskNode[]): PlanLayer[] {
  if (nodes.length === 0) return [];

  const byKey = new Map(nodes.map((n) => [n.key, n]));
  const indegree = new Map<string, number>();
  for (const n of nodes) {
    // Only count edges that point at known nodes (defensive against bad data).
    const realDeps = Array.from(n.depTaskKeys).filter((k) => byKey.has(k));
    indegree.set(n.key, realDeps.length);
  }

  const layers: PlanLayer[] = [];
  let frontier = nodes
    .filter((n) => (indegree.get(n.key) ?? 0) === 0)
    .map((n) => n.key);

  const placed = new Set<string>();
  let safety = 0;
  while (frontier.length > 0 && safety++ < 64) {
    layers.push({
      index: layers.length,
      tasks: frontier.map((k) => byKey.get(k)!).filter(Boolean),
    });
    for (const k of frontier) placed.add(k);
    const next: string[] = [];
    for (const n of nodes) {
      if (placed.has(n.key)) continue;
      const remaining = Array.from(n.depTaskKeys).filter(
        (dk) => byKey.has(dk) && !placed.has(dk),
      );
      if (remaining.length === 0) next.push(n.key);
    }
    frontier = next;
  }

  if (placed.size !== nodes.length) {
    const cycleKeys = nodes.filter((n) => !placed.has(n.key)).map((n) => n.key);
    console.warn('[planLayers] cycle detected — collapsing to single layer', cycleKeys);
    return [{ index: 0, tasks: nodes }];
  }

  return layers;
}

export function hasAnyDependencies(plan: CommandBarPlan): boolean {
  return plan.steps.some((s) => (s.depends_on_step_indexes?.length ?? 0) > 0);
}

/**
 * Pluralized task noun based on the dominant `target.entity_type` across
 * the plan's task nodes — e.g. `stories` for epics-of-stories, `contacts`
 * for CRM fan-outs, `tasks` as the generic fallback.
 */
export function taskNounFor(nodes: TaskNode[]): { singular: string; plural: string } {
  const counts = new Map<string, number>();
  for (const n of nodes) {
    if (!n.entityType) continue;
    counts.set(n.entityType, (counts.get(n.entityType) ?? 0) + 1);
  }
  let dominant: string | undefined;
  let best = 0;
  for (const [k, v] of counts.entries()) {
    if (v > best) {
      best = v;
      dominant = k;
    }
  }
  switch (dominant) {
    case 'task':
      return { singular: 'story', plural: 'stories' };
    case 'epic':
      return { singular: 'epic', plural: 'epics' };
    case 'document':
      return { singular: 'doc', plural: 'docs' };
    case 'crm_contact':
      return { singular: 'contact', plural: 'contacts' };
    case 'crm_deal':
      return { singular: 'deal', plural: 'deals' };
    case 'workspace':
      return { singular: 'workspace', plural: 'workspaces' };
    default:
      return { singular: 'target', plural: 'targets' };
  }
}
