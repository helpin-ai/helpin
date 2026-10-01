// @vitest-environment jsdom

import { act, type ReactNode } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { Agent } from '@/lib/pmTypes';

vi.mock('@tanstack/react-router', () => ({
  Link: ({ children, to, params, className }: { children: ReactNode; to: string; params: { slug: string }; className?: string }) => (
    <a href={to.replace('$slug', params.slug)} className={className}>{children}</a>
  ),
}));
vi.mock('@/hooks/queries/useExternalAgents', () => ({
  useExternalAgents: () => ({
    data: {
      configured: true,
      items: [{
        id: 'ext-1',
        agent_id: 'agent-hermes',
        name: 'Hermes',
        description: 'Builds features on our infrastructure.',
        provider_name: 'Nous Research',
        protocol_binding: 'JSONRPC',
        protocol_version: '1.0',
        status: 'active',
        last_error: '',
      }],
    },
  }),
}));

import { ExternalAgentSummarySheet } from '../ExternalAgentSummarySheet';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

const hermes = {
  id: 'agent-hermes',
  workspace_id: 'ws-1',
  is_system: false,
  name: 'Hermes',
  role: 'External agent',
  runtime_kind: 'a2a',
  trigger_mode: 'auto_on_assignment',
} as Agent;

let root: Root | undefined;
let container: HTMLDivElement | undefined;

afterEach(() => {
  if (root) act(() => root?.unmount());
  container?.remove();
  root = undefined;
  container = undefined;
  document.body.innerHTML = '';
});

describe('ExternalAgentSummarySheet', () => {
  it('shows a read-only summary with a link to External agents settings', async () => {
    container = document.createElement('div');
    document.body.appendChild(container);
    root = createRoot(container);
    await act(async () => root?.render(
      <ExternalAgentSummarySheet agent={hermes} workspaceId="ws-1" workspaceSlug="acme" onOpenChange={vi.fn()} />,
    ));

    const sheet = document.querySelector<HTMLElement>('[data-external-agent-summary]')!;
    expect(sheet).toBeTruthy();
    expect(sheet.textContent).toContain('Hermes');
    expect(sheet.textContent).toContain('External agent (A2A)');
    expect(sheet.textContent).toContain('Builds features on our infrastructure.');
    expect(sheet.textContent).toContain('Starts when a task is assigned to it');
    expect(sheet.textContent).toContain('Nous Research');
    expect(sheet.querySelector('a[href="/w/acme/settings/external-agents"]')?.textContent).toContain('External agents');
    expect(sheet.querySelector('input, textarea, [role="combobox"]')).toBeNull();
  });

  it('renders nothing without an agent', async () => {
    container = document.createElement('div');
    document.body.appendChild(container);
    root = createRoot(container);
    await act(async () => root?.render(
      <ExternalAgentSummarySheet agent={null} workspaceId="ws-1" workspaceSlug="acme" onOpenChange={vi.fn()} />,
    ));
    expect(document.querySelector('[data-external-agent-summary]')).toBeNull();
  });
});
