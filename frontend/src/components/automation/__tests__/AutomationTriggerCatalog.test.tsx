// @vitest-environment jsdom

import React, { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { TriggerCatalogList } from '../AutomationOverviewPanel';
import type { AutomationTriggerCatalogEntry } from '@/lib/types';

vi.mock('@/hooks/queries', () => ({
  useAgents: vi.fn(),
  useAutomationActivity: vi.fn(),
  useAutomationOverview: vi.fn(),
  useAutomationTriggerCatalog: vi.fn(),
}));

vi.mock('@/stores/workspaceStore', () => ({
  useWorkspaceStore: vi.fn(),
}));

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

let container: HTMLDivElement | null = null;
let root: Root | null = null;

afterEach(() => {
  act(() => root?.unmount());
  root = null;
  container?.remove();
  container = null;
});

function render(node: React.ReactNode) {
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
  act(() => root?.render(node));
}

const triggers: AutomationTriggerCatalogEntry[] = [
  {
    id: 'flow-trigger',
    binding_kind: 'automation_rule',
    category: 'workflow',
    trigger_type: 'github.pull_request_merged',
    title: 'Pull request merged',
    description: 'Runs a flow after a pull request is merged.',
    source_surface: 'Git connection',
    supports_agent_runs: true,
    binding_count: 2,
    execution_search: { trigger_type: 'github.pull_request_merged' },
    create_rule_search: { create_event_rule: true, trigger_type: 'github.pull_request_merged' },
    show_rules_search: { show_trigger: 'github.pull_request_merged' },
  },
  {
    id: 'manual-trigger',
    binding_kind: 'agent',
    category: 'manual',
    trigger_type: 'manual.document',
    title: 'Document action',
    description: 'Starts an agent from a document.',
    source_surface: 'Documents',
    supports_agent_runs: true,
    binding_count: 4,
  },
  {
    id: 'built-in-trigger',
    binding_kind: 'support_widget',
    category: 'support',
    trigger_type: 'support.message_received',
    title: 'Support message received',
    description: 'Replies to an incoming support request.',
    source_surface: 'Support settings',
    supports_agent_runs: true,
    binding_count: 1,
  },
];

function renderCatalog() {
  render(
    <TriggerCatalogList
      items={triggers}
      slug="acme"
      activityBasePath="/w/acme/automation/activity"
      flowsBasePath="/w/acme/automation/flows"
    />,
  );
}

function clickTab(name: string) {
  const tab = Array.from(container?.querySelectorAll('[role="tab"]') ?? [])
    .find((candidate) => candidate.textContent?.includes(name));
  act(() => tab?.dispatchEvent(new MouseEvent('mousedown', { bubbles: true, button: 0 })));
}

function enterSearch(value: string) {
  const input = container?.querySelector('input[aria-label="Search triggers"]') as HTMLInputElement;
  const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')?.set;
  act(() => {
    setter?.call(input, value);
    input.dispatchEvent(new Event('input', { bubbles: true }));
  });
}

describe('TriggerCatalogList', () => {
  it('defaults to flow triggers and uses divider rows without an outer card', () => {
    renderCatalog();

    expect(container?.querySelector('[role="tab"][data-state="active"]')?.textContent).toContain('Flow');
    expect(container?.textContent).toContain('Pull request merged');
    expect(container?.textContent).not.toContain('Document action');
    expect(container?.textContent).toContain('Trigger');
    expect(container?.textContent).toContain('Usage');
    expect(container?.textContent).toContain('Actions');

    const row = Array.from(container?.querySelectorAll('div') ?? [])
      .find((candidate) => candidate.textContent?.includes('Pull request merged') && candidate.className.includes('border-b'));
    expect(row?.className).not.toContain('rounded');
    expect(row?.className).not.toContain('bg-card');
  });

  it('switches categories while keeping the search query', () => {
    renderCatalog();

    enterSearch('Documents');
    expect(container?.textContent).toContain('No flow triggers match');

    clickTab('Manual');
    expect(container?.textContent).toContain('Document action');
    expect((container?.querySelector('input[aria-label="Search triggers"]') as HTMLInputElement).value).toBe('Documents');

    clickTab('Built-in');
    expect(container?.textContent).toContain('No built-in triggers match');
  });

  it('preserves the existing trigger actions and their destinations', () => {
    renderCatalog();

    const links = Array.from(container?.querySelectorAll('a') ?? []);
    expect(links.find((link) => link.textContent === 'Create flow')?.getAttribute('href')).toContain('/w/acme/automation/flows?');
    expect(links.find((link) => link.textContent === 'View flows')?.getAttribute('href')).toContain('show_trigger=github.pull_request_merged');
    expect(links.find((link) => link.textContent === 'View history')?.getAttribute('href')).toBe(
      '/w/acme/automation/activity?trigger_type=github.pull_request_merged#trigger-executions',
    );
  });
});
