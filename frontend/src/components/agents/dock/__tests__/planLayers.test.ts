import { describe, expect, it, vi } from 'vitest';
import type { CommandBarPlan, CommandBarPlanStep } from '@/lib/pmTypes';
import { buildTaskNodes, hasAnyDependencies, layerTasks } from '../planLayers';

function step(
  agent: string,
  entity: string,
  options: {
    title?: string;
    taskKey?: string;
    deps?: number[];
    entityType?: CommandBarPlanStep['target']['entity_type'];
  } = {},
): CommandBarPlanStep {
  return {
    agent_id: agent,
    agent_name: agent,
    target: {
      entity_type: options.entityType ?? 'task',
      entity_id: entity,
      display_title: options.title ?? entity,
      metadata: options.taskKey ? { task_key: options.taskKey } : undefined,
    },
    instructions: '',
    depends_on_step_indexes: options.deps,
  };
}

function plan(steps: CommandBarPlanStep[]): CommandBarPlan {
  return { steps, run_count: steps.length };
}

describe('planLayers', () => {
  it('groups steps by target and orders intra-task by deps', () => {
    const p = plan([
      step('Forge', 't1', { taskKey: 'HLP-1' }),
      step('Lens', 't1', { deps: [0] }),
      step('Forge', 't2'),
    ]);
    const nodes = buildTaskNodes(p);
    expect(nodes).toHaveLength(2);
    expect(nodes[0]).toMatchObject({ key: 't1', taskKey: 'HLP-1' });
    expect(nodes[0].stepIndexes).toEqual([0, 1]);
    expect(nodes[1]).toMatchObject({ key: 't2' });
    expect(nodes[1].stepIndexes).toEqual([2]);
  });

  it('reorders intra-task steps when deps come before non-deps in input', () => {
    const p = plan([
      step('Lens', 't1', { deps: [1] }),
      step('Forge', 't1'),
    ]);
    const nodes = buildTaskNodes(p);
    expect(nodes[0].stepIndexes).toEqual([1, 0]);
  });

  it('produces a single layer when there are no cross-task deps', () => {
    const p = plan([
      step('Forge', 't1'),
      step('Lens', 't1', { deps: [0] }),
      step('Forge', 't2'),
      step('Lens', 't2', { deps: [2] }),
    ]);
    const layers = layerTasks(buildTaskNodes(p));
    expect(layers).toHaveLength(1);
    expect(layers[0].tasks.map((t) => t.key)).toEqual(['t1', 't2']);
  });

  it('matches the mockup: 1 blocker → 5 parallel', () => {
    // Steps: blocker task t60 (Forge=0, Lens=1).
    // Five sibling tasks t61..t65 each with Forge depending on Lens of t60,
    // and Lens depending on the same task's Forge.
    const steps: CommandBarPlanStep[] = [
      step('Forge', 't60', { title: 'Auth metrics', taskKey: 'HLP-60' }),
      step('Lens', 't60', { deps: [0] }),
    ];
    for (let i = 0; i < 5; i++) {
      const tid = `t${61 + i}`;
      const forgeIdx = steps.length;
      steps.push(step('Forge', tid, { taskKey: `HLP-${61 + i}`, deps: [1] }));
      steps.push(step('Lens', tid, { deps: [forgeIdx] }));
    }
    const layers = layerTasks(buildTaskNodes(plan(steps)));
    expect(layers).toHaveLength(2);
    expect(layers[0].tasks.map((t) => t.key)).toEqual(['t60']);
    expect(layers[1].tasks).toHaveLength(5);
    expect(layers[1].tasks.map((t) => t.key)).toEqual([
      't61',
      't62',
      't63',
      't64',
      't65',
    ]);
  });

  it('falls back to a single layer + warns on a cycle', () => {
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {});
    // Two tasks that mutually depend on each other across tasks.
    const steps = [
      step('A', 'tA', { deps: [1] }), // tA waits on tB step
      step('B', 'tB', { deps: [0] }), // tB waits on tA step
    ];
    const nodes = buildTaskNodes(plan(steps));
    const layers = layerTasks(nodes);
    expect(layers).toHaveLength(1);
    expect(layers[0].tasks).toHaveLength(2);
    expect(warn).toHaveBeenCalled();
    warn.mockRestore();
  });

  it('hasAnyDependencies returns true only when at least one step has deps', () => {
    expect(hasAnyDependencies(plan([step('A', 't1'), step('B', 't1')]))).toBe(false);
    expect(
      hasAnyDependencies(plan([step('A', 't1'), step('B', 't1', { deps: [0] })])),
    ).toBe(true);
  });
});
