import { describe, expect, it } from 'vitest';

import {
  buildRunsById,
  isDeliveryPlanKind,
  pickLatestDeliveryPlan,
  planRunIdSet,
} from '../epicDeliveryDag';
import { planSummaryToRunPlan } from '@/components/agents/dock/planSummary';
import type { AgentRun, CommandBarPlanSummary } from '@/lib/pmTypes';

function summary(overrides: Partial<CommandBarPlanSummary>): CommandBarPlanSummary {
  return {
    id: 'plan-1',
    status: 'running',
    plan_kind: 'dag',
    prompt: 'implement the epic',
    page_context: { entity_type: 'epic', entity_id: 'epic-1', display_title: 'Epic' },
    steps: [],
    run_ids_by_step: {},
    current_step_index: 0,
    run_count: 0,
    created_at: '2026-05-01T00:00:00Z',
    updated_at: '2026-05-01T00:00:00Z',
    ...overrides,
  };
}

function agentRun(overrides: Partial<AgentRun>): AgentRun {
  return {
    id: 'run-1',
    workspace_id: 'ws-1',
    agent_id: 'agent-1',
    target_type: 'task',
    target_id: 'task-1',
    runtime_kind: 'native_sdk',
    invocation_mode: 'autonomous',
    approval_state: 'not_required',
    pause_reason: 'none',
    status: 'completed',
    input: {},
    output_summary: {},
    cached_input_tokens: 0,
    input_tokens: 0,
    output_tokens: 0,
    tokens_used: 0,
    created_at: '2026-05-01T00:00:00Z',
    updated_at: '2026-05-01T00:00:00Z',
    ...overrides,
  };
}

describe('isDeliveryPlanKind', () => {
  it('accepts dag and task_pipeline_fan_out', () => {
    expect(isDeliveryPlanKind('dag')).toBe(true);
    expect(isDeliveryPlanKind('task_pipeline_fan_out')).toBe(true);
  });

  it('rejects single / fan_out / undefined kinds', () => {
    expect(isDeliveryPlanKind('fan_out')).toBe(false);
    expect(isDeliveryPlanKind('one_shot_command')).toBe(false);
    expect(isDeliveryPlanKind('known_agent')).toBe(false);
    expect(isDeliveryPlanKind(undefined)).toBe(false);
  });
});

describe('pickLatestDeliveryPlan', () => {
  it('returns null when there are no plans', () => {
    expect(pickLatestDeliveryPlan([])).toBeNull();
  });

  it('returns null when no plan is a delivery kind', () => {
    expect(
      pickLatestDeliveryPlan([
        summary({ id: 'a', plan_kind: 'fan_out' }),
        summary({ id: 'b', plan_kind: 'one_shot_command' }),
      ]),
    ).toBeNull();
  });

  it('prefers a running delivery plan over a newer completed one', () => {
    const picked = pickLatestDeliveryPlan([
      summary({ id: 'done', status: 'completed', created_at: '2026-05-02T00:00:00Z' }),
      summary({ id: 'live', status: 'running', created_at: '2026-05-01T00:00:00Z' }),
    ]);
    expect(picked?.id).toBe('live');
  });

  it('picks the newest among completed when none are running', () => {
    const picked = pickLatestDeliveryPlan([
      summary({ id: 'old', status: 'completed', created_at: '2026-05-01T00:00:00Z' }),
      summary({ id: 'new', status: 'completed', created_at: '2026-05-03T00:00:00Z' }),
    ]);
    expect(picked?.id).toBe('new');
  });

  it('ignores non-delivery plans when choosing', () => {
    const picked = pickLatestDeliveryPlan([
      summary({ id: 'oneshot', plan_kind: 'one_shot_command', status: 'running', created_at: '2026-05-09T00:00:00Z' }),
      summary({ id: 'dag', plan_kind: 'dag', status: 'completed', created_at: '2026-05-01T00:00:00Z' }),
    ]);
    expect(picked?.id).toBe('dag');
  });
});

describe('buildRunsById', () => {
  it('indexes hydrated runs by id', () => {
    const map = buildRunsById(summary({ runs: [agentRun({ id: 'r1' }), agentRun({ id: 'r2' })] }));
    expect(Object.keys(map).sort()).toEqual(['r1', 'r2']);
  });

  it('returns an empty map when no runs are hydrated', () => {
    expect(buildRunsById(summary({ runs: undefined }))).toEqual({});
  });

  it('tolerates run ids referenced by steps but missing from runs', () => {
    const map = buildRunsById(summary({ run_ids_by_step: { 0: 'missing' }, runs: [agentRun({ id: 'r1' })] }));
    expect(map.missing).toBeUndefined();
    expect(map.r1).toBeDefined();
  });
});

describe('planRunIdSet', () => {
  it('collects all run ids from runIdsByStep', () => {
    const plan = planSummaryToRunPlan(summary({ run_ids_by_step: { 0: 'r1', 1: 'r2' } }));
    expect(planRunIdSet(plan)).toEqual(new Set(['r1', 'r2']));
  });

  it('returns an empty set for a plan with no runs', () => {
    expect(planRunIdSet(planSummaryToRunPlan(summary({ run_ids_by_step: {} })))).toEqual(new Set());
  });
});

describe('planSummaryToRunPlan', () => {
  it('maps snake_case summary fields to the camelCase run plan', () => {
    const plan = planSummaryToRunPlan(
      summary({
        id: 'p',
        plan_kind: 'task_pipeline_fan_out',
        status: 'running',
        run_ids_by_step: { 0: 'r1' },
        current_step_index: 2,
        created_at: '2026-05-04T00:00:00Z',
      }),
    );
    expect(plan).toMatchObject({
      id: 'p',
      planKind: 'task_pipeline_fan_out',
      status: 'running',
      runIdsByStep: { 0: 'r1' },
      currentStepIndex: 2,
      createdAt: '2026-05-04T00:00:00Z',
    });
  });

  it('defaults run_ids_by_step to an empty object', () => {
    const plan = planSummaryToRunPlan(summary({ run_ids_by_step: undefined as unknown as Record<number, string> }));
    expect(plan.runIdsByStep).toEqual({});
  });
});
