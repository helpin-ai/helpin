// @vitest-environment jsdom

import React, { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, describe, expect, it, vi } from 'vitest';
import {
  defaultTemplateInputs,
  draftFromRule,
  serializeDraft,
  validateDraft,
  FlowRow,
  flowActionSummary,
  flowDetailsSections,
  filterAutomationFlowsForSearch,
  flowMetadataPills,
  flowTriggerSummary,
  templateMatchesSearch,
  templateSelectChangeValue,
  templateSelectOptions,
  templateSelectValue,
  NO_REPOSITORY_VALUE,
} from '../AutomationFlows';
import type { AutomationRule, FlowTemplateManifest, GitRepository } from '@/lib/pmTypes';
import type { AutomationInventoryItem } from '@/lib/types';

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

const baseRule: AutomationRule = {
  id: 'rule-1',
  workspace_id: 'workspace-1',
  name: 'Release notes for acme/api',
  description: 'When GitHub publishes a release in acme/api, draft release notes.',
  enabled: true,
  trigger_type: 'github.release_published',
  trigger_config: { repo_full_name: 'acme/api' },
  action_type: 'start_agent_run',
  action_config: { agent_id: 'agent-1', target_type: 'repository', target_id: 'repo-1' },
  position: 0,
  stop_on_match: false,
  created_at: '2026-06-08T00:00:00Z',
  updated_at: '2026-06-08T00:00:00Z',
};

function healthItem(overrides: Partial<AutomationInventoryItem['health']> = {}): AutomationInventoryItem {
  return {
    inventory_id: `automation_rule:rule:${baseRule.id}`,
    catalog_id: 'automation-rule',
    kind: 'automation_rule',
    module: 'automation',
    group: 'flows',
    title: baseRule.name,
    description: baseRule.description ?? '',
    scope_type: 'workspace',
    scope_id: baseRule.workspace_id,
    scope_label: 'Workspace',
    target_types: [],
    trigger_modes: [],
    config_scope: 'workspace',
    execution_style: 'event',
    user_governed: true,
    enabled: true,
    current_write_surface: '',
    current_run_surface: '',
    output_surface: '',
    diagnostics_surface: '',
    health: {
      status: 'healthy',
      freshness: 'fresh',
      metrics: {},
      ...overrides,
    },
  };
}

describe('FlowRow', () => {
  it('shows the flow identity and compact trigger-to-action logic', () => {
    render(
      <FlowRow
        rule={baseRule}
        statesById={new Map()}
        agentNames={new Map([['agent-1', 'Release Notes Writer agent']])}
        canEdit={false}
        canRunNowAction={false}
        onEdit={() => {}}
        onRunNow={() => {}}
        onToggle={() => {}}
        onDelete={() => {}}
      />,
    );

    expect(container?.textContent).toContain('Release notes for acme/api');
    expect(container?.textContent).not.toContain('When GitHub publishes a release in acme/api, draft release notes.');
    expect(container?.textContent).toContain('When');
    expect(container?.textContent).toContain('GitHubRelease published');
    expect(container?.querySelector('[data-testid="flow-row-action-agent"]')?.textContent).toContain('Release Notes Writer agent');
    const actionAgentChildren = Array.from(container?.querySelector('[data-testid="flow-row-action-agent"]')?.children ?? []);
    expect(actionAgentChildren.map((child) => child.getAttribute('data-testid'))).toEqual([
      'flow-row-action-verb',
      'flow-row-action-agent-name',
      'flow-row-action-agent-avatar',
    ]);
  });

  it('enables the full-name tooltip when the flow name is truncated', () => {
    const originalScrollWidth = Object.getOwnPropertyDescriptor(HTMLElement.prototype, 'scrollWidth');
    const originalClientWidth = Object.getOwnPropertyDescriptor(HTMLElement.prototype, 'clientWidth');
    Object.defineProperty(HTMLElement.prototype, 'scrollWidth', { configurable: true, get: () => 240 });
    Object.defineProperty(HTMLElement.prototype, 'clientWidth', { configurable: true, get: () => 120 });

    try {
      render(
        <FlowRow
          rule={baseRule}
          statesById={new Map()}
          agentNames={new Map([['agent-1', 'Release Notes Writer agent']])}
          canEdit={false}
          canRunNowAction={false}
          onEdit={() => {}}
          onRunNow={() => {}}
          onToggle={() => {}}
          onDelete={() => {}}
        />,
      );

      expect(container?.querySelector('[data-testid="flow-row-name-text"]')?.getAttribute('data-tooltip-enabled')).toBe('true');
    } finally {
      if (originalScrollWidth) Object.defineProperty(HTMLElement.prototype, 'scrollWidth', originalScrollWidth);
      if (originalClientWidth) Object.defineProperty(HTMLElement.prototype, 'clientWidth', originalClientWidth);
    }
  });

  it('shows the finalized flow statuses', () => {
    const agentNames = new Map([['agent-1', 'Release Notes Writer agent']]);

    const cases: Array<{ rule: AutomationRule; healthItem?: AutomationInventoryItem; expected: string }> = [
      { rule: baseRule, expected: 'Active' },
      { rule: { ...baseRule, enabled: false }, expected: 'Paused' },
      { rule: baseRule, healthItem: healthItem({ status: 'error', last_error_at: '2026-06-08T01:00:00Z' }), expected: 'Errored' },
      { rule: baseRule, healthItem: healthItem({ metrics: { needs_review: true } }), expected: 'Needs review' },
      { rule: { ...baseRule, action_config: { agent_id: 'missing-agent' } }, expected: 'Incomplete' },
    ];

    for (const item of cases) {
      render(
        <FlowRow
          rule={item.rule}
          statesById={new Map()}
          agentNames={agentNames}
          healthItem={item.healthItem}
          canEdit={false}
          canRunNowAction={false}
          onEdit={() => {}}
          onRunNow={() => {}}
          onToggle={() => {}}
          onDelete={() => {}}
        />,
      );

      expect(container?.textContent).toContain(item.expected);

      act(() => {
        root?.unmount();
      });
      root = null;
      container?.remove();
      container = null;
    }
  });

  it('opens the detail drawer callback from the row with pointer and keyboard input', () => {
    const onOpen = vi.fn();
    render(
      <FlowRow
        rule={baseRule}
        statesById={new Map()}
        agentNames={new Map([['agent-1', 'Release Notes Writer agent']])}
        canEdit={false}
        canRunNowAction={false}
        onEdit={() => {}}
        onRunNow={() => {}}
        onToggle={() => {}}
        onDelete={() => {}}
        onOpen={onOpen}
      />,
    );

    const row = container?.querySelector('[role="button"]') as HTMLDivElement;
    act(() => row.click());
    act(() => row.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true })));
    expect(onOpen).toHaveBeenCalledTimes(2);
  });

  it('prioritizes current, paused, incomplete, scheduled, and trigger-driven states', () => {
    const agentNames = new Map([['agent-1', 'Release Notes Writer agent']]);
    const cases: Array<{ rule: AutomationRule; healthItem?: AutomationInventoryItem; expected: string[]; absent?: string }> = [
      {
        rule: baseRule,
        healthItem: healthItem({ last_seen_at: '2026-06-08T01:00:00Z', metrics: { last_run_status: 'running' } }),
        expected: ['Running now'],
      },
      { rule: { ...baseRule, enabled: false }, expected: ['Paused'] },
      { rule: { ...baseRule, action_config: { agent_id: 'missing-agent' } }, expected: ['Missing agent', 'Setup incomplete'] },
      {
        rule: { ...baseRule, trigger_type: 'cron', trigger_config: { schedule: '0 * * * *' } },
        expected: ['Never', 'Every hour'],
      },
      { rule: baseRule, expected: ['Never', 'Runs when triggered'] },
    ];

    for (const item of cases) {
      render(
        <FlowRow
          rule={item.rule}
          statesById={new Map()}
          agentNames={agentNames}
          healthItem={item.healthItem}
          canEdit={false}
          canRunNowAction={false}
          onEdit={() => {}}
          onRunNow={() => {}}
          onToggle={() => {}}
          onDelete={() => {}}
        />,
      );

      for (const expected of item.expected) {
        expect(container?.textContent).toContain(expected);
      }
      if (item.absent) expect(container?.textContent).not.toContain(item.absent);

      act(() => {
        root?.unmount();
      });
      root = null;
      container?.remove();
      container = null;
    }
  });

  it('shows run totals and trigger-driven scheduling beside recent activity', () => {
    render(
      <FlowRow
        rule={baseRule}
        statesById={new Map()}
        agentNames={new Map([['agent-1', 'Release Notes Writer agent']])}
        healthItem={healthItem({ last_seen_at: '2026-06-08T01:00:00Z', metrics: { total_runs: 7 } })}
        canEdit={false}
        canRunNowAction={false}
        onEdit={() => {}}
        onRunNow={() => {}}
        onToggle={() => {}}
        onDelete={() => {}}
      />,
    );

    expect(container?.textContent).toContain('7');
    expect(container?.textContent).toContain('Runs when triggered');
  });

  it('keeps error diagnostics compact in the list row', () => {
    const longError = `Failed to start run: ${'permission denied while validating repository settings '.repeat(12)}final diagnostic tail`;

    render(
      <FlowRow
        rule={baseRule}
        statesById={new Map()}
        agentNames={new Map([['agent-1', 'Release Notes Writer agent']])}
        healthItem={healthItem({
          status: 'error',
          last_error_at: '2026-06-08T01:00:00Z',
          last_error_message: longError,
        })}
        canEdit={false}
        canRunNowAction={false}
        onEdit={() => {}}
        onRunNow={() => {}}
        onToggle={() => {}}
        onDelete={() => {}}
      />,
    );

    expect(container?.textContent).toContain('Errored');
    expect(container?.querySelector('p')?.className).toContain('truncate');
  });
});

describe('filterAutomationFlowsForSearch', () => {
  it('filters flows by selected agent id', () => {
    const otherRule = {
      ...baseRule,
      id: 'rule-2',
      name: 'Other agent flow',
      action_config: { agent_id: 'agent-2' },
    } satisfies AutomationRule;

    expect(filterAutomationFlowsForSearch([baseRule, otherRule], { agent_id: 'agent-1' })).toEqual([baseRule]);
  });
});

describe('flow row summaries', () => {
  it('separates trigger and action content for table-like rows', () => {
    const agentNames = new Map([['agent-1', 'Release Notes Writer agent']]);

    expect(flowTriggerSummary(baseRule, new Map()).label).toBe('Release published');
    expect(flowActionSummary(baseRule, new Map(), agentNames).label).toBe('Run Release Notes Writer agent');
  });

  it('builds details sections with trigger, action, scope, instructions, and activity', () => {
    const sections = flowDetailsSections({
      rule: {
        ...baseRule,
        action_config: {
          ...baseRule.action_config,
          additional_context: 'Draft notes for customer-facing changes only.',
        },
      },
      statesById: new Map(),
      agentNames: new Map([['agent-1', 'Release Notes Writer agent']]),
      teamName: 'Docs',
      timezone: 'Asia/Karachi',
    });

    expect(sections.map((section) => section.title)).toEqual(['Flow setup', 'Trigger setup', 'Action setup', 'Instructions', 'Diagnostics']);
    expect(sections.find((section) => section.title === 'Instructions')?.rows).toContainEqual({
      label: 'Run context',
      value: 'Draft notes for customer-facing changes only.',
    });
  });
});

describe('defaultTemplateInputs', () => {
  const repositoryTemplate: FlowTemplateManifest = {
    key: 'run_on_release',
    version: 1,
    name: 'Run on release',
    icon: 'tag',
    short_description: 'Runs an agent when a release is published.',
    categories: ['engineering'],
    agent: { pick_existing: { required: true, constraints: { targets: ['repository'] } } },
    trigger: { type: 'event', event: 'github.release_published' },
    inputs: [
      { key: 'repository_id', type: 'repository', required: true, label: 'Repository' },
      { key: 'agent_id', type: 'agent', required: true, label: 'Agent' },
    ],
    flow: { action: 'start_agent_run' },
  };

  const singleRepo = {
    id: 'repo-1',
    full_name: 'helpin/dummy-flow-test',
  } as GitRepository;

  it('selects the only repository by default for repository-backed templates', () => {
    const inputs = defaultTemplateInputs(repositoryTemplate, null, [singleRepo]);

    expect(inputs.repository_id).toBe('repo-1');
  });

  it('does not select a repository when multiple repositories are available', () => {
    const inputs = defaultTemplateInputs(repositoryTemplate, null, [
      singleRepo,
      { ...singleRepo, id: 'repo-2', full_name: 'helpin/another-repo' } as GitRepository,
    ]);

    expect(inputs.repository_id).toBe('');
  });
});

describe('template repository select helpers', () => {
  const repo = {
    id: 'repo-1',
    full_name: 'helpin/dummy-flow-test',
  } as GitRepository;

  it('offers an explicit no repository choice for optional repository inputs', () => {
    const input = { key: 'source_repository_id', type: 'repository', required: false, label: 'Source repository' } as FlowTemplateManifest['inputs'][number];

    expect(templateSelectOptions(input, [repo])).toEqual([
      { value: NO_REPOSITORY_VALUE, label: 'No repository', description: 'Skip repository context' },
      { value: 'repo-1', label: 'helpin/dummy-flow-test' },
    ]);
  });

  it('keeps required repository inputs limited to repositories', () => {
    const input = { key: 'repository_id', type: 'repository', required: true, label: 'Repository' } as FlowTemplateManifest['inputs'][number];

    expect(templateSelectOptions(input, [repo])).toEqual([
      { value: 'repo-1', label: 'helpin/dummy-flow-test' },
    ]);
  });

  it('stores the no repository choice as an empty value', () => {
    const input = { key: 'source_repository_id', type: 'repository', required: false, label: 'Source repository' } as FlowTemplateManifest['inputs'][number];

    expect(templateSelectValue(input, '')).toBe(NO_REPOSITORY_VALUE);
    expect(templateSelectChangeValue(input, NO_REPOSITORY_VALUE)).toBe('');
  });
});

describe('templateMatchesSearch', () => {
  const template: FlowTemplateManifest = {
    key: 'run_on_release',
    version: 1,
    name: 'Run on release',
    icon: 'tag',
    short_description: 'Runs an agent when a release is published.',
    categories: ['engineering'],
    agent: { pick_existing: { required: true, constraints: { targets: ['repository'] } } },
    trigger: { type: 'event', event: 'github.release_published' },
    inputs: [],
    flow: { action: 'start_agent_run' },
  };

  it('matches template names, descriptions, categories, keys, and trigger metadata', () => {
    expect(templateMatchesSearch(template, '')).toBe(true);
    expect(templateMatchesSearch(template, 'release')).toBe(true);
    expect(templateMatchesSearch(template, 'agent')).toBe(true);
    expect(templateMatchesSearch(template, 'engineering')).toBe(true);
    expect(templateMatchesSearch(template, 'run_on')).toBe(true);
    expect(templateMatchesSearch(template, 'github')).toBe(true);
    expect(templateMatchesSearch(template, 'billing')).toBe(false);
  });

  it('handles templates without categories when search is non-empty', () => {
    const uncategorizedTemplate = {
      ...template,
      categories: null,
    } as unknown as FlowTemplateManifest;

    expect(templateMatchesSearch(uncategorizedTemplate, 'release')).toBe(true);
    expect(templateMatchesSearch(uncategorizedTemplate, 'engineering')).toBe(false);
  });
});

describe('flowMetadataPills', () => {
  it('does not duplicate cron schedules in scope metadata', () => {
    const rule = {
      ...baseRule,
      trigger_type: 'cron',
      trigger_config: { schedule: '0 0 * * 1' },
    };

    expect(flowTriggerSummary(rule, new Map(), 'Asia/Karachi').label).toBe('Every week on Mon at 05:00 Asia/Karachi');
    expect(flowMetadataPills(rule, undefined, 'Asia/Karachi').some((pill) => pill.key === 'schedule')).toBe(false);
  });

  it('does not show a generic template pill', () => {
    const rule = {
      ...baseRule,
      template_instance_id: 'template-instance-1',
    };

    expect(flowMetadataPills(rule)).not.toContainEqual(
      { key: 'template', label: 'Template', tone: 'info' },
    );
  });
});


describe('semantic condition persistence', () => {
  it('round-trips through the create/edit payload and allows removal', () => {
    const rule = { ...baseRule, trigger_config: { ...baseRule.trigger_config, semantic_condition: { text: 'The release concerns authentication' } } };
    const draft = draftFromRule(rule, [], 'UTC');
    expect(draft.semanticCondition).toBe('The release concerns authentication');
    expect(serializeDraft(draft, rule.workspace_id, 'UTC').trigger_config.semantic_condition).toEqual({ text: draft.semanticCondition });
    draft.semanticCondition = '';
    expect(serializeDraft(draft, rule.workspace_id, 'UTC').trigger_config).not.toHaveProperty('semantic_condition');
  });
  it('does not silently remove the condition when switching to a schedule', () => {
    const draft = draftFromRule({ ...baseRule, trigger_config: { semantic_condition: { text: 'Authentication fix' } } }, [], 'UTC');
    draft.triggerType = 'cron';
    expect(validateDraft(draft)).toContain('Remove the semantic condition');
  });
});
