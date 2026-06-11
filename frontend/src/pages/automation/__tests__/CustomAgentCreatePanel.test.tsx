// @vitest-environment jsdom

import React, { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { CustomAgentCreatePanel } from '../CustomAgentCreatePanel';
import { createDefaultCustomAgentForm, type CustomAgentFormData } from '../customAgentCreateModel';

vi.mock('@/lib/services/automationService', () => ({
  automationService: {
    draftCustomAgent: vi.fn(),
  },
}));

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
(globalThis as typeof globalThis & { ResizeObserver: typeof ResizeObserver }).ResizeObserver = class {
  observe() {}
  unobserve() {}
  disconnect() {}
};
Element.prototype.scrollIntoView = vi.fn();

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

function renderPanel(overrides: Partial<{
  form: CustomAgentFormData;
  onChange: (form: CustomAgentFormData) => void;
  onCreate: () => void;
  mode: 'create' | 'edit';
  canSave: boolean;
}> = {}) {
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
  const props = {
    form: createDefaultCustomAgentForm(),
    workspaceId: 'ws-1',
    onChange: vi.fn(),
    teams: [{ id: 'team-product', workspace_id: 'ws-1', name: 'Product' }],
    tools: [
      {
        name: 'list_tasks',
        description: 'Read tasks in the workspace.',
        category: 'Project management',
        input_schema: { type: 'object', properties: {} },
        presets: [],
      },
      {
        name: 'create_task',
        description: 'Create project tasks.',
        category: 'Project management',
        input_schema: { type: 'object', properties: {} },
        presets: [],
      },
      {
        name: 'read_document',
        description: 'Read workspace documents.',
        category: 'Knowledge',
        input_schema: { type: 'object', properties: {} },
        presets: [],
      },
    ],
    skills: [
      {
        key: 'planning_skill',
        title: 'Planning skill',
        description: 'Plan work clearly.',
        source_kind: 'built_in',
        required_tools: ['list_tasks'],
      },
    ],
    providerOptions: [
      {
        value: 'anthropic',
        label: 'Anthropic',
        model_placeholder: 'claude-sonnet-4-6',
        supports_reasoning_effort: false,
        supports_service_tier: false,
      },
      {
        value: 'openai',
        label: 'OpenAI',
        model_placeholder: 'gpt-5.5',
        supports_reasoning_effort: true,
        supports_service_tier: true,
      },
    ],
    advancedOpen: false,
    onAdvancedOpenChange: vi.fn(),
    onCreate: vi.fn(),
    saving: false,
    ...overrides,
  };
  act(() => {
    root?.render(<CustomAgentCreatePanel {...props} />);
  });
  return props;
}

function click(text: string) {
  const scopedButtons = Array.from(container?.querySelectorAll('button') ?? []);
  const portalButtons = Array.from(document.querySelectorAll('button'))
    .filter((button) => !container?.contains(button));
  const commandItems = Array.from(document.querySelectorAll('[cmdk-item]'));
  const button = [...scopedButtons, ...portalButtons, ...commandItems]
    .find((item) => item.textContent?.includes(text));
  if (!button) throw new Error(`Button not found: ${text}`);
  act(() => {
    button.dispatchEvent(new MouseEvent('click', { bubbles: true }));
  });
  return button as HTMLButtonElement;
}

describe('CustomAgentCreatePanel', () => {
  it('opens edit mode directly in settings with save copy', () => {
    renderPanel({
      mode: 'edit',
      form: {
        ...createDefaultCustomAgentForm(),
        name: 'Docs helper',
        system_prompt: 'Keep docs current.',
      },
    });

    expect(container?.textContent).toContain('Edit Custom Agent');
    expect(container?.textContent).toContain('Agent settings');
    expect(container?.textContent).toContain('Save changes');
    expect(container?.textContent).not.toContain('Describe the agent you want');
    expect(container?.textContent).not.toContain('Generate agent setup');
  });

  it('disables edit save when there are no changes', () => {
    renderPanel({
      mode: 'edit',
      canSave: false,
      form: {
        ...createDefaultCustomAgentForm(),
        name: 'Docs helper',
      },
    });

    const saveButton = Array.from(container?.querySelectorAll('button') ?? [])
      .find((button) => button.textContent?.includes('Save changes'));
    expect(saveButton).toBeTruthy();
    expect((saveButton as HTMLButtonElement).disabled).toBe(true);
  });

  it('renders a describe-or-start-blank purpose step without pseudo-template cards', () => {
    renderPanel();

    expect(container?.textContent).toContain('Describe the agent you want');
    expect(container?.textContent).toContain('Share the outcome you want in a few sentences');
    expect(container?.textContent).toContain('Generate agent setup');
    expect(container?.textContent).toContain('Start blank');
    expect(container?.textContent).not.toContain('Planning helper');
    expect(container?.textContent).not.toContain('Coding helper');
  });

  it('focuses the description input when the drawer opens', () => {
    renderPanel();

    expect(document.activeElement).toBe(container?.querySelector('textarea'));
  });

  it('start blank opens the editable form with safe defaults', () => {
    const onChange = vi.fn();
    renderPanel({ onChange });

    click('Start blank');

    expect(container?.textContent).toContain('Agent name');
    expect(onChange).toHaveBeenCalledWith(expect.objectContaining({
      approval_mode: 'always',
      runtime_kind: 'native_sdk',
      default_invocation_mode: 'interactive',
    }));
  });

  it('shows all primary settings on one page and keeps approval and advanced collapsed', () => {
    renderPanel({ form: { ...createDefaultCustomAgentForm(), name: 'Planner' } });

    click('Start blank');
    expect(container?.textContent).toContain('Agent settings');
    expect(container?.textContent).toContain('Agent name');
    expect(container?.textContent).toContain('Instructions');
    expect(container?.textContent).toContain('Access and work area');
    expect(container?.textContent).toContain('Capabilities');
    expect(container?.textContent).toContain('Run approval');
    expect(container?.textContent).toContain('Advanced settings');
    expect(container?.textContent).not.toContain('Review new agent');
    expect(container?.textContent).not.toContain('AI Provider');
  });

  it('shows approval preference by default', () => {
    renderPanel({
      form: { ...createDefaultCustomAgentForm(), name: 'Planner' },
    });

    click('Start blank');
    expect(container?.textContent).toContain('Require approval before each run');
  });

  it('keeps runtime options aligned with the old custom-agent drawer', () => {
    renderPanel({
      form: { ...createDefaultCustomAgentForm(), name: 'Planner' },
      advancedOpen: true,
    });

    click('Start blank');
    click('Advanced settings');

    expect(container?.textContent).toContain('Native SDK');
    expect(container?.textContent).toContain('Interactive');

    click('Native SDK');
    expect(document.body.textContent).toContain('OpenCode');

    click('Interactive');
    expect(document.body.textContent).toContain('Autonomous');
  });

  it('keeps model entry simple with auto as the default', () => {
    renderPanel({
      form: { ...createDefaultCustomAgentForm(), name: 'Planner' },
      advancedOpen: true,
    });

    click('Start blank');
    click('Advanced settings');

    const modelInput = Array.from(container?.querySelectorAll('input') ?? [])
      .find((input) => input.placeholder === 'Auto');
    expect(modelInput).toBeTruthy();
    expect(container?.textContent).not.toContain('claude-sonnet-4-6');
    expect(container?.textContent).not.toContain('Use suggested');
    expect(container?.textContent).toContain('Coming soon');
  });

  it('supports all-teams and multi-team access', () => {
    const onChange = vi.fn();
    renderPanel({
      form: { ...createDefaultCustomAgentForm(), name: 'Planner', teamAccessMode: 'specific_teams' },
      onChange,
    });

    click('Start blank');
    expect(container?.textContent).toContain('All teams');
    expect(container?.textContent).toContain('Specific teams');
    click('Product');
    expect(onChange).toHaveBeenLastCalledWith(expect.objectContaining({
      teamAccessMode: 'specific_teams',
      team_ids: ['team-product'],
    }));
  });

  it('selects tools and skills from the behavior step', () => {
    const onChange = vi.fn();
    renderPanel({
      form: { ...createDefaultCustomAgentForm(), name: 'Planner' },
      onChange,
    });

    click('Start blank');
    click('Select tools');
    expect(document.body.textContent).toContain('All');
    expect(document.body.textContent).toContain('Project management');
    click('Project management');
    click('create_task');
    expect(onChange).toHaveBeenLastCalledWith(expect.objectContaining({
      allowed_tools: ['create_task'],
    }));

    renderPanel({
      form: { ...createDefaultCustomAgentForm(), name: 'Planner' },
      onChange,
    });
    click('Start blank');
    click('Select skills');
    click('Planning skill');
    expect(onChange).toHaveBeenLastCalledWith(expect.objectContaining({
      allowed_tools: ['list_tasks'],
      skills: [expect.objectContaining({ key: 'planning_skill' })],
    }));
  });

  it('explains why a skill-required tool cannot be removed directly', () => {
    const onChange = vi.fn();
    renderPanel({
      form: {
        ...createDefaultCustomAgentForm(),
        name: 'Planner',
        allowed_tools: ['list_tasks'],
        skills: [{ key: 'planning_skill' }],
      },
      onChange,
    });

    click('Start blank');
    click('list_tasks');

    expect(container?.textContent).toContain('list_tasks is required by Planning skill');
    expect(onChange).not.toHaveBeenCalledWith(expect.objectContaining({
      allowed_tools: [],
    }));
  });
});
