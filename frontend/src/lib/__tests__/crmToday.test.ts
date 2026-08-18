import { describe, expect, it } from 'vitest';
import { buildDealAttentionItems, buildTodayTaskItems } from '@/lib/crmToday';
import type { CRMDeal, CRMDealHealthScore } from '@/lib/crmTypes';
import type { Task } from '@/lib/pmTypes';

const now = new Date('2026-08-18T12:00:00');

const task = (overrides: Partial<Task>): Task => ({
  id: 'task-1',
  workspace_id: 'workspace-1',
  display_id: 1,
  task_key: 'CRM-1',
  name: 'Follow up',
  task_type: 'chore',
  workflow_id: 'workflow-1',
  workflow_state_id: 'state-1',
  priority: 'medium',
  severity: 'minor',
  position: 1,
  started: false,
  completed: false,
  blocked: false,
  contacts: [{ object_type: 'contact', object_id: 'contact-1', title: 'Buyer' }],
  archived: false,
  created_at: '2026-08-01T00:00:00Z',
  updated_at: '2026-08-01T00:00:00Z',
  ...overrides,
});

const deal = (overrides: Partial<CRMDeal>): CRMDeal => ({
  id: 'deal-1',
  workspace_id: 'workspace-1',
  display_id: 'DEAL-1',
  name: 'Acme expansion',
  pipeline_id: 'pipeline-1',
  stage_id: 'stage-1',
  currency: 'USD',
  custom_properties: {},
  created_at: '2026-08-01T00:00:00Z',
  updated_at: '2026-08-17T00:00:00',
  stage: {
    id: 'stage-1',
    pipeline_id: 'pipeline-1',
    name: 'Proposal',
    stage_type: 'open',
    position: 2,
    probability: 50,
    created_at: '2026-08-01T00:00:00Z',
    updated_at: '2026-08-01T00:00:00Z',
  },
  ...overrides,
});

describe('buildTodayTaskItems', () => {
  it('keeps CRM-linked incomplete tasks due today or earlier and sorts oldest first', () => {
    const items = buildTodayTaskItems([
      task({ id: 'future', deadline: '2026-08-19T09:00:00' }),
      task({ id: 'today', deadline: '2026-08-18T17:00:00' }),
      task({ id: 'overdue', deadline: '2026-08-16T09:00:00' }),
      task({ id: 'done', deadline: '2026-08-17T09:00:00', completed: true }),
      task({ id: 'generic', contacts: undefined, deadline: '2026-08-17T09:00:00' }),
      task({ id: 'done-state', deadline: '2026-08-17T09:00:00', state_type: 'done' }),
    ], now);

    expect(items.map((item) => [item.task.id, item.timing])).toEqual([
      ['overdue', 'overdue'],
      ['today', 'today'],
    ]);
  });
});

describe('buildDealAttentionItems', () => {
  it('prioritizes overdue open deals and excludes closed deals', () => {
    const healthScores = [{
      id: 'health-1',
      workspace_id: 'workspace-1',
      deal_id: 'at-risk',
      score: 25,
      factors: {},
      calculated_at: '2026-08-18T00:00:00Z',
      created_at: '2026-08-18T00:00:00Z',
    }] as CRMDealHealthScore[];

    const items = buildDealAttentionItems([
      deal({ id: 'stale', name: 'Stale', updated_at: '2026-08-01T00:00:00' }),
      deal({ id: 'at-risk', name: 'At risk' }),
      deal({ id: 'overdue', name: 'Overdue', close_date: '2026-08-10T00:00:00' }),
      deal({ id: 'won', stage: { ...deal({}).stage!, stage_type: 'won' } }),
    ], healthScores, now);

    expect(items.map((item) => item.deal.id)).toEqual(['overdue', 'at-risk', 'stale']);
    expect(items[0]?.reasons).toContain('Close date overdue');
    expect(items[1]?.reasons).toContain('Health score 25');
  });
});
