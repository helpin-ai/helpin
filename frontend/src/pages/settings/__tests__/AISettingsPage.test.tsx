// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { AISettingsPage } from '../AISettingsPage';
import { TooltipProvider } from '@/components/ui/tooltip';
import { aiConnectionService } from '@/lib/services/aiConnectionService';
import { aiProfileService } from '@/lib/services/aiProfileService';

vi.mock('@/lib/services/aiConnectionService', () => ({
  aiConnectionService: { list: vi.fn(), endpoints: vi.fn(), create: vi.fn(), reconnect: vi.fn(), poll: vi.fn(), disconnect: vi.fn() },
}));
vi.mock('@/lib/services/aiProfileService', () => ({
  aiProfileService: { list: vi.fn(), save: vi.fn(), remove: vi.fn(), settings: vi.fn(), setDefault: vi.fn() },
}));
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

const frame = vi.hoisted(() => ({ permissions: new Set<string>(['workspace.update']) }));
vi.mock('../SettingsPageFrame', () => ({
  SettingsPageFrame: ({ section, children }: { section: string; children: (ctx: unknown) => React.ReactNode }) => (
    <div>
      <h1>{section === 'ai' ? 'AI' : 'AI connections'}</h1>
      <p>
        {section === 'ai'
          ? 'Shared connections, profiles, and the default profile agents inherit.'
          : 'Your API keys and ChatGPT login, plus the profiles that use them.'}
      </p>
      {children({ workspaceId: 'ws', permissions: { has: (perm: string) => frame.permissions.has(perm) } })}
    </div>
  ),
}));
(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

let root: Root;
let client: QueryClient;

beforeEach(() => {
  vi.stubGlobal('ResizeObserver', class { observe() {} unobserve() {} disconnect() {} });
  HTMLElement.prototype.scrollIntoView = vi.fn();
  const container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
  client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  frame.permissions = new Set(['workspace.update']);
  vi.mocked(aiProfileService.list).mockResolvedValue({ data: [], error: null });
  vi.mocked(aiProfileService.settings).mockResolvedValue({ data: { default_profile_id: null }, error: null });
});

afterEach(() => {
  act(() => root.unmount());
  client.clear();
  document.body.innerHTML = '';
  vi.unstubAllGlobals();
  vi.clearAllMocks();
});

async function render(scope: 'personal' | 'workspace') {
  await act(async () =>
    root.render(
      <QueryClientProvider client={client}>
        <TooltipProvider>
          <AISettingsPage scope={scope} />
        </TooltipProvider>
      </QueryClientProvider>,
    ),
  );
  for (let i = 0; i < 10; i++) await act(async () => { await new Promise((resolve) => setTimeout(resolve, 0)); });
}

it('asks for configuration when the deployment has no AI connection support', async () => {
  vi.mocked(aiConnectionService.list).mockResolvedValue({ data: { enabled: false, connections: [], models: [] }, error: null });
  await render('workspace');
  expect(document.body.textContent).toContain('AI connections are not configured');
});

it('explains why a profile cannot be created before a connection exists', async () => {
  vi.mocked(aiConnectionService.list).mockResolvedValue({ data: { enabled: true, connections: [], models: [] }, error: null });
  await render('workspace');
  expect(document.body.textContent).toContain('No connections yet');
  expect(document.body.textContent).toContain('Add a connection first');
  const create = Array.from(document.querySelectorAll('button')).find((button) => button.textContent === 'Create profile');
  expect(create?.disabled).toBe(true);
  expect(document.body.textContent).not.toContain('Add a connection before creating a profile.');
  await act(async () => { create?.parentElement?.focus(); });
  expect(document.querySelector('[role="tooltip"]')?.textContent).toContain('Add a connection before creating a profile.');
});

it('keeps default selection in the profiles list without a separate section', async () => {
  vi.mocked(aiConnectionService.list).mockResolvedValue({
    data: {
      enabled: true,
      models: [],
      connections: [{ id: 'c1', scope: 'workspace', user_id: null, name: 'Team key', provider: 'openai', status: 'connected' }],
    },
    error: null,
  });
  await render('workspace');
  expect(document.body.textContent).not.toContain('Create a shared profile to choose a default.');
  expect(document.querySelector('#workspace-ai-default')).toBeNull();
});

it('does not repeat the section description in the page body', async () => {
  vi.mocked(aiConnectionService.list).mockResolvedValue({ data: { enabled: true, connections: [], models: [] }, error: null });
  await render('workspace');
  const description = 'Shared connections, profiles, and the default profile agents inherit.';
  const occurrences = document.body.textContent!.split(description).length - 1;
  expect(occurrences).toBe(1);
});

it('tells a member without manage permission that the page is read-only', async () => {
  frame.permissions = new Set();
  vi.mocked(aiConnectionService.list).mockResolvedValue({ data: { enabled: true, connections: [], models: [] }, error: null });
  await render('workspace');
  expect(document.body.textContent).toContain('Connections are read-only');
  expect(Array.from(document.querySelectorAll('button')).some((b) => b.textContent?.includes('Add connection'))).toBe(false);
});

it('orders Workspace before Personal and keeps management scoped when switching tabs', async () => {
  frame.permissions = new Set();
  vi.mocked(aiConnectionService.list).mockResolvedValue({ data: { enabled: true, connections: [], models: [] }, error: null });
  await render('workspace');
  const tabs = Array.from(document.querySelectorAll<HTMLButtonElement>('[role="tab"]'));
  expect(tabs.map(tab => tab.textContent)).toEqual(['Workspace', 'Personal']);
  expect(tabs[0].getAttribute('aria-selected')).toBe('true');
  expect(document.body.textContent).toContain('Connections are read-only');
  await act(async () => { tabs[1].dispatchEvent(new MouseEvent('mousedown', { bubbles: true, button: 0 })); });
  expect(tabs[1].getAttribute('aria-selected')).toBe('true');
  expect(document.body.textContent).not.toContain('Connections are read-only');
  expect(document.body.textContent).not.toContain('Default profile');
  expect(Array.from(document.querySelectorAll('button')).some(button => button.textContent === 'Add connection' && !button.disabled)).toBe(true);
  await act(async () => { tabs[0].dispatchEvent(new MouseEvent('mousedown', { bubbles: true, button: 0 })); });
  expect(document.body.textContent).toContain('Connections are read-only');
});

it('keeps personal setup links on the Personal tab', async () => {
  vi.mocked(aiConnectionService.list).mockResolvedValue({ data: { enabled: true, connections: [], models: [] }, error: null });
  await render('personal');
  expect(document.querySelector('[role="tab"][data-state="active"]')?.textContent).toBe('Personal');
  expect(document.body.textContent).not.toContain('Default profile');
});
