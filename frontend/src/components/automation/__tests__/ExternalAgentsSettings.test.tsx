// @vitest-environment jsdom

import { act, type ReactNode } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { AgentCardSummary, ExternalAgent } from '@/lib/externalAgentTypes';

const { service, confirmMock, permissions } = vi.hoisted(() => ({
  service: {
    list: vi.fn(),
    preview: vi.fn(),
    create: vi.fn(),
    update: vi.fn(),
    refreshCard: vi.fn(),
    remove: vi.fn(),
  },
  confirmMock: vi.fn(),
  permissions: { canManageSettings: true },
}));

vi.mock('@/lib/services/externalAgentsService', () => ({ externalAgentsService: service }));
vi.mock('@/components/ui/confirm-dialog', () => ({ useConfirm: () => confirmMock }));
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }));
vi.mock('@/hooks/useWorkspaceTeams', () => ({
  useWorkspaceTeams: () => ({
    teams: [{ id: 'team-eng', workspace_id: 'ws-1', name: 'Engineering' }],
    loading: false,
    findTeamName: (id: string) => (id === 'team-eng' ? 'Engineering' : undefined),
  }),
}));
vi.mock('@/pages/settings/SettingsPageFrame', () => ({
  SettingsPageFrame: ({ section, headerAction, children }: {
    section: string;
    headerAction?: (context: unknown) => ReactNode;
    children: (context: unknown) => ReactNode;
  }) => {
    const context = { workspaceId: 'ws-1', currentWorkspaceSlug: 'acme', permissions };
    return (
      <div data-section={section}>
        <div data-header-action>{headerAction?.(context)}</div>
        {children(context)}
      </div>
    );
  },
}));

import { ExternalAgentsSettingsPage } from '@/pages/settings/ExternalAgentsSettingsPage';
import { ExternalAgentBadge } from '@/components/agents/ExternalAgentBadge';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

const card: AgentCardSummary = {
  name: 'Hermes',
  description: 'Builds and ships features on our infrastructure.',
  card_url: 'https://hermes.example.com/.well-known/agent-card.json',
  interface_url: 'https://hermes.example.com/a2a',
  protocol_binding: 'JSONRPC',
  protocol_version: '1.0',
  provider_name: 'Nous Research',
  version: '0.21.5',
  skills: [
    { id: 'code', name: 'Write code', description: 'Implements changes.' },
    { id: 'review', name: 'Review code' },
  ],
  capabilities: { streaming: true, push_notifications: false },
};

const hermes: ExternalAgent = {
  ...card,
  id: 'ext-1',
  workspace_id: 'ws-1',
  agent_id: 'agent-1',
  status: 'active',
  last_checked_at: new Date().toISOString(),
  last_error: '',
  token_hint: '…a1b2',
  allowed_team_ids: ['team-eng'],
  created_at: '2026-09-01T00:00:00Z',
  updated_at: '2026-09-01T00:00:00Z',
};

let root: Root | undefined;
let container: HTMLDivElement | undefined;
let client: QueryClient | undefined;

async function renderPage() {
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
  client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  await act(async () => root?.render(
    <QueryClientProvider client={client!}>
      <ExternalAgentsSettingsPage />
    </QueryClientProvider>,
  ));
  await flush();
  return container;
}

async function flush() {
  for (let i = 0; i < 5; i += 1) {
    await act(async () => { await new Promise((resolve) => setTimeout(resolve, 0)); });
  }
}

function buttonByText(scope: ParentNode, text: string) {
  return Array.from(scope.querySelectorAll<HTMLButtonElement>('button')).find((button) => button.textContent?.trim() === text);
}

function typeInto(input: HTMLInputElement, value: string) {
  const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!;
  setter.call(input, value);
  input.dispatchEvent(new Event('input', { bubbles: true }));
}

beforeEach(() => {
  permissions.canManageSettings = true;
  service.list.mockResolvedValue({ data: { items: [hermes] }, error: null, status: 200 });
});

afterEach(() => {
  if (root) act(() => root?.unmount());
  container?.remove();
  client?.clear();
  root = undefined;
  container = undefined;
  client = undefined;
  document.body.innerHTML = '';
  vi.clearAllMocks();
});

describe('ExternalAgentsSettingsPage', () => {
  it('lists external agents with their card details, status, token hint, and teams', async () => {
    const view = await renderPage();

    expect(view.querySelector('[data-section="external-agents"]')).toBeTruthy();
    const row = view.querySelector('[data-external-agent-row="ext-1"]')!;
    expect(row).toBeTruthy();
    expect(row.textContent).toContain('Hermes');
    expect(row.textContent).toContain('Active');
    expect(row.textContent).toContain('Nous Research');
    expect(row.textContent).toContain('v0.21.5');
    expect(row.querySelector('[data-external-agent-skills]')?.textContent).toBe('2 skills: Write code, Review code');
    expect(row.textContent).toContain('Token …a1b2');
    expect(row.textContent).toContain('Teams: Engineering');
    expect(row.querySelector('button[aria-label="More actions for Hermes"]')).toBeTruthy();
    expect(buttonByText(view, 'Add agent')?.disabled).toBe(false);
  });

  it('shows the last error for agents in the error state', async () => {
    service.list.mockResolvedValue({
      data: { items: [{ ...hermes, status: 'error', last_error: 'Agent Card returned 401' }] },
      error: null,
      status: 200,
    });
    const view = await renderPage();

    const row = view.querySelector('[data-external-agent-row="ext-1"]')!;
    expect(row.textContent).toContain('Error');
    expect(row.textContent).toContain('Agent Card returned 401');
  });

  it('explains A2A in the empty state and offers to add an agent', async () => {
    service.list.mockResolvedValue({ data: { items: [] }, error: null, status: 200 });
    const view = await renderPage();

    expect(view.textContent).toContain('No external agents yet');
    expect(view.textContent).toContain('Agent2Agent (A2A)');
    expect(view.querySelector('a[href="https://a2a-protocol.org"]')).toBeTruthy();
    expect(buttonByText(view, 'Add an external agent')).toBeTruthy();
  });

  it('shows a not-configured notice when the server returns 503', async () => {
    service.list.mockResolvedValue({ data: null, error: 'External agents are not configured on this server', status: 503 });
    const view = await renderPage();

    expect(view.querySelector('[data-external-agents-not-configured]')).toBeTruthy();
    expect(view.textContent).toContain('EXTERNAL_A2A_ENCRYPTION_KEY');
    expect(buttonByText(view, 'Add agent')?.disabled).toBe(true);
    expect(view.querySelector('[data-external-agents-list]')).toBeNull();
  });

  it('is read-only without settings.manage', async () => {
    permissions.canManageSettings = false;
    const view = await renderPage();

    expect(view.textContent).toContain('External agents are read-only');
    expect(view.querySelector('[data-external-agent-row="ext-1"]')).toBeTruthy();
    expect(buttonByText(view, 'Add agent')).toBeUndefined();
    expect(view.querySelector('button[aria-label="More actions for Hermes"]')).toBeNull();
    expect(view.querySelector('button[aria-label="Refresh Agent Card for Hermes"]')).toBeNull();

    service.list.mockResolvedValue({ data: { items: [] }, error: null, status: 200 });
    await act(async () => { await client?.invalidateQueries(); });
    await flush();
    expect(buttonByText(view, 'Add an external agent')).toBeUndefined();
  });

  it('checks the agent card before adding it', async () => {
    service.list.mockResolvedValue({ data: { items: [] }, error: null, status: 200 });
    service.preview.mockResolvedValue({ data: { card }, error: null, status: 200 });
    service.create.mockResolvedValue({ data: hermes, error: null, status: 201 });
    const view = await renderPage();

    await act(async () => buttonByText(view, 'Add agent')!.click());
    const dialog = document.querySelector<HTMLElement>('[role="dialog"]')!;
    expect(dialog).toBeTruthy();
    expect(dialog.querySelector('[data-external-agent-data-notice]')?.textContent).toContain('Tasks sent to this agent leave Helpin');
    const check = buttonByText(dialog, 'Check agent')!;
    expect(check.disabled).toBe(true);

    await act(async () => {
      typeInto(dialog.querySelector<HTMLInputElement>('#external-agent-url')!, 'https://hermes.example.com');
      typeInto(dialog.querySelector<HTMLInputElement>('#external-agent-token')!, 'secret-token');
    });
    expect(dialog.querySelector<HTMLInputElement>('#external-agent-token')!.type).toBe('password');
    expect(buttonByText(dialog, 'Check agent')!.disabled).toBe(false);

    await act(async () => buttonByText(dialog, 'Check agent')!.click());
    await flush();
    expect(service.preview).toHaveBeenCalledWith('ws-1', { card_url: 'https://hermes.example.com', token: 'secret-token' });
    expect(service.create).not.toHaveBeenCalled();

    const review = document.querySelector<HTMLElement>('[data-external-agent-step="review"]')!;
    expect(review).toBeTruthy();
    const summary = review.querySelector('[data-external-agent-card]')!;
    expect(summary.textContent).toContain('Hermes');
    expect(summary.textContent).toContain('Nous Research');
    expect(summary.textContent).toContain('A2A 1.0 · JSONRPC');
    expect(summary.textContent).toContain('Supports streaming');
    expect(summary.textContent).toContain('Write code');
    expect(review.textContent).toContain('All teams');

    await act(async () => buttonByText(review, 'Add agent')!.click());
    await flush();
    expect(service.create).toHaveBeenCalledWith('ws-1', { card_url: 'https://hermes.example.com', token: 'secret-token' });
    expect(document.querySelector('[data-external-agent-step]')).toBeNull();
  });

  it('keeps the dialog on the first step and shows the preview error', async () => {
    service.list.mockResolvedValue({ data: { items: [] }, error: null, status: 200 });
    service.preview.mockResolvedValue({ data: null, error: 'Agent Card not found', status: 422 });
    const view = await renderPage();

    await act(async () => buttonByText(view, 'Add agent')!.click());
    const dialog = document.querySelector<HTMLElement>('[role="dialog"]')!;
    await act(async () => {
      typeInto(dialog.querySelector<HTMLInputElement>('#external-agent-url')!, 'https://hermes.example.com');
      typeInto(dialog.querySelector<HTMLInputElement>('#external-agent-token')!, 'secret-token');
    });
    await act(async () => buttonByText(dialog, 'Check agent')!.click());
    await flush();

    expect(dialog.querySelector('[role="alert"]')?.textContent).toBe('Agent Card not found');
    expect(document.querySelector('[data-external-agent-step="connect"]')).toBeTruthy();
  });
});

describe('ExternalAgentBadge', () => {
  it('renders only for a2a agents', async () => {
    container = document.createElement('div');
    document.body.appendChild(container);
    root = createRoot(container);
    await act(async () => root?.render(
      <>
        <ExternalAgentBadge agent={{ runtime_kind: 'a2a' }} />
        <ExternalAgentBadge agent={{ runtime_kind: 'native_sdk' }} />
      </>,
    ));
    const badges = container.querySelectorAll('[data-external-agent-badge]');
    expect(badges).toHaveLength(1);
    expect(badges[0].textContent).toBe('External');
  });
});
