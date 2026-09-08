import { describe, expect, it } from 'vitest';

import {
  buildActivityTargetPresentation,
  buildExecutionMetadataPresentation,
  buildExecutionFlowPresentation,
  buildExecutionTargetPresentation,
  formatAttentionWaitDuration,
  normalizeActivityTargetType,
} from '../automationActivityRunPresentation';

describe('automation activity run presentation', () => {
  it('shows companies by name with a link to the CRM record', () => {
    expect(buildActivityTargetPresentation({
      targetType: 'crm_company', targetId: 'company-1', targetTitle: 'Acme',
    })).toMatchObject({ typeLabel: 'Company', primary: 'Acme', clickable: true });
  });

  it('uses task key and target title as the primary target label', () => {
    expect(
      buildActivityTargetPresentation({
        targetType: 'task',
        targetId: 'task-1',
        run: {
          target_type: 'task',
          target_id: 'task-1',
          target_info: { target_type: 'task', target_id: 'task-1', task_key: 'HLP-239', title: 'Fix shared run link' },
          input: {},
        },
      }).primary,
    ).toBe('HLP-239 · Fix shared run link');
  });

  it('labels documents, epics, and workspace targets without UUID-first text', () => {
    expect(
      buildActivityTargetPresentation({
        targetType: 'document',
        targetId: 'doc-1',
        targetTitle: 'API setup guide',
      }).primary,
    ).toBe('API setup guide');

    expect(
      buildActivityTargetPresentation({
        targetType: 'epic',
        targetId: 'epic-1',
        targetTitle: 'Billing automation cleanup',
      }).primary,
    ).toBe('Billing automation cleanup');

    expect(
      buildActivityTargetPresentation({
        targetType: 'workspace',
        targetId: 'ws-1',
      }).primary,
    ).toBe('Workspace');
  });

  it('uses a matching agent run to improve execution target labels', () => {
    const presentation = buildExecutionTargetPresentation(
      {
        target_type: 'task',
        target_id: 'task-1',
        reference_title: 'Customer escalation flow',
      },
      {
        target_type: 'task',
        target_id: 'task-1',
        target_info: { target_type: 'task', target_id: 'task-1', task_key: 'HLP-240', title: 'Review escalation queue' },
        input: {},
      },
    );

    expect(presentation.primary).toBe('HLP-240 · Review escalation queue');
    expect(presentation.clickable).toBe(true);
  });

  it('normalizes prefixed task target type names', () => {
    expect(normalizeActivityTargetType('pm_task')).toBe('task');
  });

  it('keeps automation flow names separate from trigger labels', () => {
    expect(
      buildExecutionFlowPresentation({
        binding_kind: 'automation_rule',
        binding_title: 'Task entered review',
        reference_title: 'Customer escalation flow',
        trigger_title: 'State changed',
      }),
    ).toEqual({
      flowLabel: 'Customer escalation flow',
      triggerLabel: 'State changed',
    });
  });

  it('builds a compact subject source trigger metadata row', () => {
    expect(
      buildExecutionMetadataPresentation({
        agent_name: 'Planner',
        binding_kind: 'automation_rule',
        binding_title: 'Task entered review',
        reference_title: 'Customer escalation flow',
        trigger_title: 'Cron',
      }),
    ).toEqual({
      subject: 'Planner',
      sourceLabel: 'Customer escalation flow',
      sourceIsFlow: true,
      triggerLabel: 'Cron',
    });

    expect(
      buildExecutionMetadataPresentation({
        agent_name: 'Planner',
        binding_kind: 'manual',
        binding_title: 'Manual run',
        trigger_title: 'Manual',
      }),
    ).toEqual({
      subject: 'Planner',
      sourceLabel: 'Manual',
      sourceIsFlow: false,
      triggerLabel: '',
    });
  });

  it('uses Flow as the subject when a flow-only row has no agent name', () => {
    expect(
      buildExecutionMetadataPresentation({
        agent_name: 'Unknown agent',
        binding_kind: 'automation_rule',
        binding_title: 'Move task to Done',
        reference_title: 'Review handoff',
        trigger_title: 'Task State Entered',
      }),
    ).toMatchObject({
      subject: 'Flow',
      sourceLabel: 'Review handoff',
      triggerLabel: 'Task State Entered',
    });
  });

  it('formats needs-you wait time with days when enough time has passed', () => {
    expect(formatAttentionWaitDuration('2026-06-14T07:00:00Z', '2026-06-17T09:30:00Z')).toBe('3d 2h');
    expect(formatAttentionWaitDuration('2026-06-17T07:00:00Z', '2026-06-17T09:30:00Z')).toBe('2h 30m');
  });
});
