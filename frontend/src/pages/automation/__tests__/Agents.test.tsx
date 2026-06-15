// @vitest-environment jsdom

import React, { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, describe, expect, it } from 'vitest';
import {
  AgentActions,
  AgentsListHeader,
  AgentsListTable,
  canEditWorkspacePresetVersionDescription,
  getAgentProviderConfigState,
} from '../Agents';
import type { Agent, AgentModelProviderOption, AgentPresetDefinition } from '@/lib/pmTypes';

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

describe('Agents list header', () => {
  it('shows config, run volume, and last run columns', () => {
    render(<AgentsListHeader />);

    expect(container?.textContent).toContain('Config');
    expect(container?.textContent).toContain('Runs · 7d');
    expect(container?.textContent).toContain('Last run');
    expect(container?.textContent).toContain('Action');
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

describe('getAgentProviderConfigState', () => {
  it('requires a compatible provider before Codex model config can be edited', () => {
    const options = [
      {
        value: 'anthropic',
        label: 'Anthropic',
        model_placeholder: 'claude-sonnet-4-20250514',
        supports_reasoning_effort: false,
        supports_service_tier: false,
      },
    ] satisfies AgentModelProviderOption[];

    expect(getAgentProviderConfigState('codex', 'openai', options)).toEqual({
      providerOptions: [],
      selectedProviderOption: undefined,
      hasCompatibleProvider: false,
      providerDisabled: true,
      modelDisabled: true,
      providerMessage: 'Add OpenAI, OpenRouter, or enable Codex ChatGPT auth.',
      modelMessage: 'Select a compatible AI provider first.',
    });
  });
});
