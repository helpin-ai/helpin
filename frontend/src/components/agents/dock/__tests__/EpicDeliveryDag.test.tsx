// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { EpicDeliveryDag } from '../EpicDeliveryDag';
import type { CommandBarRunPlan } from '../planSummary';
import type { AgentRun } from '@/lib/pmTypes';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

let container: HTMLDivElement;
let root: Root;

beforeEach(() => {
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
});

afterEach(() => {
  act(() => root.unmount());
  container.remove();
});

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

function dagPlan(overrides: Partial<CommandBarRunPlan> = {}): CommandBarRunPlan {
  return {
    id: 'plan-1',
    planKind: 'dag',
    status: 'running',
    steps: [
      {
        agent_id: 'agent-forge',
        agent_name: 'Forge',
        target: { entity_type: 'task', entity_id: 'task-1', display_title: 'USE-70' },
        instructions: '',
      },
      {
        agent_id: 'agent-lens',
        agent_name: 'Lens',
        target: { entity_type: 'task', entity_id: 'task-2', display_title: 'USE-71' },
        instructions: '',
        depends_on_step_indexes: [0],
      },
    ],
    runIdsByStep: { 0: 'run-1' },
    currentStepIndex: 0,
    ...overrides,
  };
}

describe('EpicDeliveryDag', () => {
  it('renders nothing for a non-delivery plan kind', () => {
    act(() => {
      root.render(
        <EpicDeliveryDag plan={dagPlan({ planKind: 'one_shot_command' })} runsById={{}} onOpenRun={vi.fn()} />,
      );
    });
    expect(container.textContent).toBe('');
  });

  it('renders the DAG steps and header for a dag plan', () => {
    act(() => {
      root.render(
        <EpicDeliveryDag plan={dagPlan()} runsById={{ 'run-1': agentRun({ id: 'run-1' }) }} onOpenRun={vi.fn()} />,
      );
    });
    expect(container.textContent).toContain('Forge');
    expect(container.textContent).toContain('Lens');
    // planKindLabel('dag') => 'Orchestrated'
    expect(container.textContent).toContain('Orchestrated');
  });

  it('opens the run for a step that has one when clicked', () => {
    const onOpenRun = vi.fn();
    act(() => {
      root.render(
        <EpicDeliveryDag plan={dagPlan()} runsById={{ 'run-1': agentRun({ id: 'run-1' }) }} onOpenRun={onOpenRun} />,
      );
    });
    const clickable = container.querySelector<HTMLElement>('[role="button"]');
    expect(clickable).toBeTruthy();
    act(() => {
      clickable?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });
    expect(onOpenRun).toHaveBeenCalledWith('run-1');
  });

  it('groups dag steps into stages and marks parallel and blocked steps', () => {
    act(() => {
      root.render(
        <EpicDeliveryDag
          plan={dagPlan({
            steps: [
              {
                agent_id: 'agent-forge',
                agent_name: 'Forge',
                target: { entity_type: 'task', entity_id: 'task-1', display_title: 'USE-70' },
                instructions: '',
              },
              {
                agent_id: 'agent-forge',
                agent_name: 'Forge',
                target: { entity_type: 'task', entity_id: 'task-2', display_title: 'USE-71' },
                instructions: '',
              },
              {
                agent_id: 'agent-lens',
                agent_name: 'Lens',
                target: { entity_type: 'task', entity_id: 'task-1', display_title: 'USE-70' },
                instructions: '',
                depends_on_step_indexes: [0, 1],
              },
            ],
            runIdsByStep: { 0: 'run-1' },
          })}
          runsById={{ 'run-1': agentRun({ id: 'run-1', status: 'running' }) }}
          onOpenRun={vi.fn()}
        />,
      );
    });
    expect(container.textContent).toContain('Stage 1 · 2 in parallel');
    expect(container.textContent).toContain('Stage 2');
    expect(container.textContent).toContain('Running');
    expect(container.textContent).toContain('waiting on 2');
  });

  it('renders epic scaffolding steps as Setup/Finalize bookends, not lanes', () => {
    act(() => {
      root.render(
        <EpicDeliveryDag
          plan={dagPlan({
            planKind: 'task_pipeline_fan_out',
            steps: [
              {
                agent_id: 'agent-cmd',
                agent_name: 'Command Agent',
                step_type: 'ensure_epic_branch',
                target: { entity_type: 'epic', entity_id: 'epic-1', display_title: 'My long epic title' },
                instructions: '',
              },
              {
                agent_id: 'agent-forge',
                agent_name: 'Forge',
                target: { entity_type: 'task', entity_id: 'task-1', display_title: 'USE-70' },
                instructions: '',
                depends_on_step_indexes: [0],
              },
              {
                agent_id: 'agent-cmd',
                agent_name: 'Command Agent',
                step_type: 'open_epic_pr',
                target: { entity_type: 'epic', entity_id: 'epic-1', display_title: 'My long epic title' },
                instructions: '',
                depends_on_step_indexes: [1],
              },
            ],
            runIdsByStep: { 0: 'run-1' },
          })}
          runsById={{ 'run-1': agentRun({ id: 'run-1' }) }}
          onOpenRun={vi.fn()}
        />,
      );
    });
    expect(container.textContent).toContain('Setup');
    expect(container.textContent).toContain('Finalize');
    expect(container.textContent).toContain('Epic branch');
    expect(container.textContent).toContain('Final PR');
    // The epic title must not appear as a lane label anymore.
    expect(container.textContent).not.toContain('My long epic title');
  });

  it('renders task-pipeline rows for a task_pipeline_fan_out plan', () => {
    act(() => {
      root.render(
        <EpicDeliveryDag
          plan={dagPlan({ planKind: 'task_pipeline_fan_out' })}
          runsById={{ 'run-1': agentRun({ id: 'run-1' }) }}
          onOpenRun={vi.fn()}
        />,
      );
    });
    // planKindLabel('task_pipeline_fan_out') => 'Task pipeline'
    expect(container.textContent).toContain('Task pipeline');
    expect(container.textContent).toContain('USE-70');
  });
});
