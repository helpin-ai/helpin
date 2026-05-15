import { describe, expect, it } from 'vitest';

import { formatAgentOptionLabel, pickDefaultAgentForTarget } from '../AgentPickerCard';
import type { Agent } from '@/lib/pmTypes';

const agent = (overrides: Partial<Agent>): Agent => ({
  id: 'agent',
  workspace_id: 'workspace',
  is_system: false,
  name: 'Agent',
  status: 'idle',
  runtime_kind: 'native_sdk',
  skills: [],
  role: '',
  tools: [],
  allowed_tools: [],
  allowed_commands: [],
  allowed_targets: [],
  preset_key: '',
  trigger_mode: 'manual',
  tokens_used_this_month: 0,
  approval_mode: 'preset_default',
  max_concurrent_runs: 1,
  default_invocation_mode: 'interactive',
  created_at: '',
  updated_at: '',
  ...overrides,
});

describe('pickDefaultAgentForTarget', () => {
  it('prefers the system Atlas epic planner for epic creation', () => {
    const selected = pickDefaultAgentForTarget(
      [
        agent({ id: 'custom-epic', name: 'Custom planner', preset_key: 'epic_planner', allowed_targets: ['epic'] }),
        agent({ id: 'atlas', name: 'Atlas', preset_key: 'epic_planner', allowed_targets: ['epic'], is_system: true }),
      ],
      'epic',
    );

    expect(selected?.id).toBe('atlas');
  });

  it('prefers the system Scribe task planner for task creation', () => {
    const selected = pickDefaultAgentForTarget(
      [
        agent({ id: 'builder', name: 'Builder', preset_key: 'code_builder', allowed_targets: ['task'] }),
        agent({ id: 'scribe', name: 'Scribe', preset_key: 'task_planner', allowed_targets: ['task'], is_system: true }),
      ],
      'task',
    );

    expect(selected?.id).toBe('scribe');
  });
});

describe('formatAgentOptionLabel', () => {
  it('shows the agent name with the role when the role is available', () => {
    expect(formatAgentOptionLabel(agent({ name: 'Scribe', role: 'Task Planner' }))).toBe('Scribe - Task Planner');
  });

  it('falls back to just the agent name without duplicating blank roles', () => {
    expect(formatAgentOptionLabel(agent({ name: 'Atlas', role: ' ' }))).toBe('Atlas');
  });
});
