// @vitest-environment jsdom

import React, { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, describe, expect, it } from 'vitest';
import {
  AgentActions,
  AgentRow,
  AgentsListHeader,
  AgentsListTable,
  CUSTOM_AGENT_TARGET_OPTIONS,
  canEditWorkspacePresetVersionDescription,
  buildSystemPresetVersionForm,
  hasWorkspacePresetVersionChanges,
  getAgentAnalyticsSummary,
  getAgentRecentRunSummary,
  getAgentTokenUsageSummary,
  getAgentProviderConfigState,
  getVersionToolEditingState,
  runNowTargetOptions,
  sortAgentsForDisplay,
} from '../Agents';
import { agentEditorKind } from '@/lib/externalAgents';
import type { Agent, AgentModelProviderOption, AgentPresetDefinition, AgentRun } from '@/lib/pmTypes';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

let container: HTMLDivElement | null = null;
let root: Root | null = null;

afterEach(() => {
  act(() => {
    root?.unmount();
  });
  root = null;
  container?.remove();
  container = null;
});

function render(node: React.ReactNode) {
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
  act(() => {
    root?.render(node);
  });
}

describe('workspace preset version activation', () => {
  it('hydrates a duplicate from the saved version and ignores non-editable model controls', () => {
    const preset = {
      version_key: 'ask_agent_workspace_1',
      runtime_kind: 'native_sdk',
      provider: 'openrouter',
      model: 'new-model',
      model_tier: 'large',
      execution_config: { max_tool_steps: 24, openrouter: { provider: 'auto' } },
      system_prompt: 'Ask Agent instructions',
      instruction_skills: [],
      available_skills: [],
      allowed_tools: ['mcp__logs__search'],
      allowed_target_types: ['workspace'],
      supported_modes: ['autonomous', 'interactive'],
      default_invocation_mode: 'interactive',
    } as AgentPresetDefinition;
    const sourceForm = {
      preset_key: 'ask_agent',
      preset_version_key: 'ask_agent_default',
      provider: 'openai',
      model: 'old-model',
    } as Parameters<typeof buildSystemPresetVersionForm>[0];

    const savedForm = buildSystemPresetVersionForm(sourceForm, preset);

    expect(savedForm.preset_version_key).toBe(preset.version_key);
    expect(savedForm.provider).toBe(preset.provider);
    expect(savedForm.model).toBe(preset.model);
    expect(hasWorkspacePresetVersionChanges(savedForm, preset)).toBe(false);
    expect(hasWorkspacePresetVersionChanges({
      ...savedForm,
      allowed_tools: [...savedForm.allowed_tools, 'mcp__logs__read'],
    }, preset)).toBe(true);
  });
});

describe('Agents list header', () => {
  it('shows config, run volume, and last run columns', () => {
    render(<AgentsListHeader />);

    expect(container?.textContent).toContain('Config');
    expect(container?.textContent).toContain('Runs · 7d');
    expect(container?.textContent).toContain('Last run');
    expect(container?.textContent).toContain('Used in flows');
    expect(container?.textContent).not.toContain('Action');
    expect(container?.textContent).not.toContain('Recent activity');
  });

  it('keeps the action column reachable when the split columns overflow', () => {
    render(
      <AgentsListTable>
        <AgentsListHeader />
      </AgentsListTable>,
    );

    expect(container?.firstElementChild?.className).toContain('overflow-x-auto');
    expect(container?.firstElementChild?.className).not.toContain('overflow-hidden');
    expect(container?.firstElementChild?.className).not.toContain('rounded');
    expect(container?.firstElementChild?.className).not.toContain('border');
    expect(container?.firstElementChild?.className).not.toContain('bg-card');
    expect(container?.firstElementChild?.firstElementChild?.className).toContain('min-w-[64rem]');
    expect(container?.firstElementChild?.firstElementChild?.className).not.toContain('xl:min-w');
  });
});

describe('getAgentAnalyticsSummary', () => {
  it('summarizes the selected analytics range for drawer cards', () => {
    const summary = getAgentAnalyticsSummary({
      range: '30d',
      bucket: 'day',
      series: [
        { period: '2026-06-05', runs: 3, completed: 0, failed: 2, needs_attention: 0, tokens: 155_724 },
        { period: '2026-06-08', runs: 1, completed: 1, failed: 0, needs_attention: 1, tokens: 672_096 },
      ],
    });

    expect(summary.rangeLabel).toBe('30d');
    expect(summary.runs).toBe(4);
    expect(summary.completed).toBe(1);
    expect(summary.failed).toBe(2);
    expect(summary.needsAttention).toBe(1);
    expect(summary.tokens).toBe(827_820);
    expect(summary.successRate).toBe('25%');
    expect(summary.avgTokensPerRun).toBe(206_955);
  });
});

const baseRun: AgentRun = {
  id: 'run-1234567890',
  workspace_id: 'workspace-1',
  agent_id: 'agent-1',
  target_type: 'task',
  target_id: 'task-1',
  runtime_kind: 'native_sdk',
  invocation_mode: 'interactive',
  approval_state: 'not_required',
  pause_reason: 'none',
  status: 'completed',
  input: {},
  output_summary: {},
  cached_input_tokens: 0,
  input_tokens: 0,
  output_tokens: 0,
  tokens_used: 42,
  created_at: '2026-06-08T00:00:00Z',
  updated_at: '2026-06-08T00:00:00Z',
};

describe('getAgentRecentRunSummary', () => {
  it('uses task keys and titles together when both are available', () => {
    const summary = getAgentRecentRunSummary({
      ...baseRun,
      target_info: {
        target_type: 'task',
        target_id: 'task-1',
        task_key: 'HELP-42',
        title: 'Fix inbox selection after empty queue',
      },
      input: { trigger: { source: 'automation_rule', trigger_type: 'task.state_entered' } },
    });

    expect(summary.title).toBe('HELP-42 · Fix inbox selection after empty queue');
    expect(summary.subtitle).toBe('Task · State changed');
  });

  it('falls back to useful copy instead of repeating the target type', () => {
    const summary = getAgentRecentRunSummary({
      ...baseRun,
      target_info: undefined,
      input: {},
    });

    expect(summary.title).toBe('Task run');
    expect(summary.subtitle).toBe('Task · Interactive run');
    expect(summary.subtitle.toLowerCase()).not.toBe(summary.title.toLowerCase());
  });

  it('summarizes repository runs with repository and trigger context', () => {
    const summary = getAgentRecentRunSummary({
      ...baseRun,
      target_type: 'repository',
      target_id: 'repo-1',
      repo_full_name: 'helpin-ai/helpin',
      base_branch: 'main',
      input: { trigger: { source: 'automation_rule', trigger_type: 'github.pull_request_merged' } },
    });

    expect(summary.title).toBe('helpin-ai/helpin');
    expect(summary.subtitle).toBe('Repository · PR merged · main');
  });

  it('uses document titles when available', () => {
    const summary = getAgentRecentRunSummary({
      ...baseRun,
      target_type: 'document',
      target_id: 'doc-1',
      target_info: {
        target_type: 'document',
        target_id: 'doc-1',
        title: 'Q3 launch plan',
      },
      input: { trigger: { source: 'manual', trigger_type: 'manual' } },
    });

    expect(summary.title).toBe('Q3 launch plan');
    expect(summary.subtitle).toBe('Document · Manual run');
  });
});

describe('getAgentTokenUsageSummary', () => {
  it('does not show a lifetime total below the current month usage', () => {
    expect(getAgentTokenUsageSummary({
      ...baseAgent,
      tokens_used_this_month: 401_677,
      tokens_used_total: 286_294,
    })).toEqual({
      monthlyTokensUsed: 401_677,
      totalTokensUsed: 401_677,
      totalTokensAdjusted: true,
    });
  });

  it('uses the recorded lifetime total when it is higher than monthly usage', () => {
    expect(getAgentTokenUsageSummary({
      ...baseAgent,
      tokens_used_this_month: 10_000,
      tokens_used_total: 286_294,
    })).toEqual({
      monthlyTokensUsed: 10_000,
      totalTokensUsed: 286_294,
      totalTokensAdjusted: false,
    });
  });
});

const baseAgent: Agent = {
  id: 'agent-1',
  workspace_id: 'workspace-1',
  is_system: true,
  name: 'Atlas',
  role: 'Epic Planner',
  status: 'idle',
  runtime_kind: 'native_sdk',
  skills: [],
  trigger_mode: 'manual',
  tools: [],
  tokens_used_this_month: 0,
  allowed_tools: [],
  allowed_commands: [],
  allowed_targets: [],
  approval_mode: 'preset_default',
  max_concurrent_runs: 1,
  default_invocation_mode: 'interactive',
  created_at: '2026-05-07T00:00:00Z',
  updated_at: '2026-05-07T00:00:00Z',
};

describe('AgentRow', () => {
  it('opens from the row keyboard target without hijacking nested actions', () => {
    let openCount = 0;
    render(
      <AgentRow
        agent={baseAgent}
        presets={[]}
        onOpen={() => { openCount += 1; }}
        onOpenRun={() => {}}
        onRunNow={() => {}}
        onDelete={() => {}}
        canEdit={false}
        workspaceSlug="acme"
      />,
    );

    const row = container?.querySelector('[role="button"][aria-label="Open Atlas"]');
    const actions = container?.querySelector('[aria-label="More agent actions"]');
    expect(row).not.toBeNull();
    expect(actions).not.toBeNull();
    expect(row?.className).not.toContain('bg-amber');

    act(() => {
      actions?.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
    });
    expect(openCount).toBe(0);

    act(() => {
      row?.dispatchEvent(new KeyboardEvent('keydown', { key: ' ', bubbles: true }));
    });
    expect(openCount).toBe(1);
  });
});

describe('PM agent targets', () => {
  it('offers familiar CRM labels without changing any saved agent targets', () => {
    expect(CUSTOM_AGENT_TARGET_OPTIONS.filter((option) => option.value.startsWith('crm_'))).toEqual([
      { value: 'crm_deal', label: 'CRM deals', description: 'Run on CRM deal records.' },
      { value: 'crm_contact', label: 'CRM contacts', description: 'Run on CRM contact records.' },
      { value: 'crm_company', label: 'CRM companies', description: 'Run on CRM company records.' },
    ]);
    const allowedTargets = ['crm_contact', 'crm_company', 'crm_contact'] as Agent['allowed_targets'];
    expect(runNowTargetOptions({ ...baseAgent, allowed_targets: allowedTargets })).toEqual(['crm_contact', 'crm_company']);
    expect(allowedTargets).toEqual(['crm_contact', 'crm_company', 'crm_contact']);
    expect(runNowTargetOptions({ ...baseAgent, allowed_targets: ['task'] })).toEqual(['task']);
    expect(runNowTargetOptions({ ...baseAgent, allowed_targets: ['crm_deal'] })).toEqual(['crm_deal']);
  });

  it('shows sprint and objective targets and accepts both for Run now', () => {
    const visibleTargets = CUSTOM_AGENT_TARGET_OPTIONS.map((option) => option.value);
    expect(visibleTargets).toEqual(expect.arrayContaining(['sprint', 'objective']));

    const agent = {
      ...baseAgent,
      is_system: false,
      allowed_targets: ['sprint', 'objective'],
    } satisfies Agent;
    expect(runNowTargetOptions(agent)).toEqual(['sprint', 'objective']);
  });
});

describe('AgentActions', () => {
  it('uses a concrete actions menu instead of a decorative row arrow', () => {
    render(
      <AgentActions
        agent={baseAgent}
        workspaceSlug="acme"
        onOpenRun={() => {}}
        onRunNow={() => {}}
        canEdit
      />,
    );

    expect(container?.querySelector('[aria-label="More agent actions"]')).not.toBeNull();
  });
});

describe('sortAgentsForDisplay', () => {
  it('keeps system agents first and sorts each group alphabetically', () => {
    const agents = [
      { ...baseAgent, id: 'custom-zeta', is_system: false, name: 'Zeta' },
      { ...baseAgent, id: 'system-beta', is_system: true, name: 'Beta' },
      { ...baseAgent, id: 'custom-alpha', is_system: false, name: 'Alpha' },
      { ...baseAgent, id: 'system-alpha', is_system: true, name: 'Alpha' },
    ];

    expect(sortAgentsForDisplay(agents).map((agent) => agent.id)).toEqual([
      'system-alpha',
      'system-beta',
      'custom-alpha',
      'custom-zeta',
    ]);
  });
});

describe('canEditWorkspacePresetVersionDescription', () => {
  it('allows only persisted workspace preset versions to edit description from the detail panel', () => {
    const workspaceVersion = {
      id: 'version-1',
      scope: 'workspace',
    } as AgentPresetDefinition;
    const productVersion = {
      id: undefined,
      scope: 'product',
    } as AgentPresetDefinition;

    expect(canEditWorkspacePresetVersionDescription(workspaceVersion)).toBe(true);
    expect(canEditWorkspacePresetVersionDescription(productVersion)).toBe(false);
    expect(canEditWorkspacePresetVersionDescription(null)).toBe(false);
  });
});

describe('getVersionToolEditingState', () => {
  it('allows editing tools for editable Codex custom versions', () => {
    expect(getVersionToolEditingState({ versionReadOnly: false, runtimeKind: 'codex' })).toEqual({
      canEdit: true,
      disabledReason: '',
    });
  });

  it('explains why product-owned preset versions cannot edit tools directly', () => {
    expect(getVersionToolEditingState({ versionReadOnly: true, runtimeKind: 'codex' })).toEqual({
      canEdit: false,
      disabledReason: 'Duplicate this version to edit tools.',
    });
  });
});

describe('getAgentProviderConfigState', () => {
  it('allows configured providers for native model configuration', () => {
    const options = [
      {
        value: 'anthropic',
        label: 'Anthropic',
        default_model: 'claude-opus-4-8',
        model_placeholder: 'claude-opus-4-8',
        supports_reasoning_effort: false,
        supports_service_tier: false,
      },
    ] satisfies AgentModelProviderOption[];

    expect(getAgentProviderConfigState('native_sdk', 'openai', options)).toEqual({
      providerOptions: options,
      selectedProviderOption: undefined,
      hasCompatibleProvider: true,
      providerDisabled: false,
      modelDisabled: true,
      providerMessage: '',
      modelMessage: 'Select a compatible AI provider first.',
    });
  });
});

describe('agentEditorKind', () => {
  it('routes external A2A agents to the read-only summary instead of the editor', () => {
    expect(agentEditorKind({ is_system: false, runtime_kind: 'a2a' })).toBe('external');
    expect(agentEditorKind({ is_system: false, runtime_kind: 'native_sdk' })).toBe('custom');
    expect(agentEditorKind({ is_system: true, runtime_kind: 'native_sdk' })).toBe('system');
  });
});
