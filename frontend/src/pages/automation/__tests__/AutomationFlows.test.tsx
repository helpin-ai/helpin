// @vitest-environment jsdom

import React, { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, describe, expect, it } from 'vitest';
import {
  defaultTemplateInputs,
  FlowRow,
  templateSelectChangeValue,
  templateSelectOptions,
  templateSelectValue,
  NO_REPOSITORY_VALUE,
} from '../AutomationFlows';
import type { AutomationRule, FlowTemplateManifest, GitRepository } from '@/lib/pmTypes';

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

describe('FlowRow', () => {
  it('shows the saved flow name and internal description', () => {
    render(
      <FlowRow
        rule={baseRule}
        statesById={new Map()}
        agentNames={new Map([['agent-1', 'Release Notes Writer agent']])}
        canEdit={false}
        onEdit={() => {}}
        onToggle={() => {}}
        onDelete={() => {}}
        onUninstallTemplate={() => {}}
        agentReferencedElsewhere={false}
      />,
    );

    expect(container?.textContent).toContain('Release notes for acme/api');
    expect(container?.textContent).toContain('When GitHub publishes a release in acme/api, draft release notes.');
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
