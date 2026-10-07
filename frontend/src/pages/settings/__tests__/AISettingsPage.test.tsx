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
  aiConnectionService: { list: vi.fn(), endpoints: vi.fn(), create: vi.fn(), reconnect: vi.fn(), poll: vi.fn(), disconnect: vi.fn(), models: vi.fn(), refreshModels: vi.fn() },
}));
vi.mock('@/lib/services/aiProfileService', () => ({
  aiProfileService: { list: vi.fn(), save: vi.fn(), remove: vi.fn(), settings: vi.fn(), setDefault: vi.fn(), setVisibility: vi.fn(), enableModel: vi.fn() },
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


it('offers connections without a profile creation step', async () => {
  vi.mocked(aiConnectionService.list).mockResolvedValue({ data: { enabled: true, connections: [], models: [] }, error: null });
  await render('workspace');
  expect(document.body.textContent).toContain('No connections yet');
  expect(document.body.textContent).not.toContain('Create profile');
  expect(document.body.textContent).not.toContain('No profiles yet');
});

it('gives members personal setup and read-only workspace models without scope tabs', async () => {
  frame.permissions = new Set();
  vi.mocked(aiConnectionService.list).mockResolvedValue({ data: { enabled: true, connections: [], models: [] }, error: null });
  await render('workspace');
  expect(document.querySelector('[role="tab"]')).toBeNull();
  expect(document.body.textContent).toContain('Personal connections');
  expect(Array.from(document.querySelectorAll('button')).some(b => b.textContent === 'Add connection' && !b.disabled)).toBe(true);
});

it('lets admins choose workspace or personal setup', async () => {
  vi.mocked(aiConnectionService.list).mockResolvedValue({ data: { enabled: true, connections: [], models: [] }, error: null });
  await render('personal');
  expect(Array.from(document.querySelectorAll('[role="tab"]')).map(t => t.textContent)).toEqual(['Workspace', 'Personal']);
  expect(document.querySelector('[role="tab"][data-state="active"]')?.textContent).toBe('Personal');
});

it('opens model selection on a connection and enables a discovered model without naming a profile', async () => {
  vi.mocked(aiConnectionService.list).mockResolvedValue({ data: { enabled:true, connections:[{id:'c1', name:'Team key', provider:'openai', scope:'workspace', user_id:null, status:'connected'}], models:[] }, error:null });
  vi.mocked(aiConnectionService.models).mockResolvedValue({data:{models:[{id:'gpt-new', name:'New model'}],source:'provider',stale:false},error:null});
  vi.mocked(aiProfileService.enableModel).mockResolvedValue({data:null,error:null});
  await render('workspace');
  await act(async () => { document.querySelector<HTMLButtonElement>('[aria-label="Models for Team key"]')?.click(); });
  for (let i=0;i<5;i++) await act(async () => {await new Promise(r => setTimeout(r,0));});
  const toggle=document.querySelector<HTMLButtonElement>('[role="switch"][aria-label="Show New model in Ask Agent"]');
  expect(toggle).not.toBeNull();
  await act(async () => toggle!.click());
  expect(aiProfileService.enableModel).toHaveBeenCalledWith('ws', {name:'New model',connection_id:'c1',model:'gpt-new'});
  expect(document.body.textContent).not.toContain('Profile name');
});

it('keeps existing configurations reachable when their connection is unavailable', async () => {
  vi.mocked(aiConnectionService.list).mockResolvedValue({data:{enabled:true,connections:[],models:[]},error:null});
  vi.mocked(aiProfileService.list).mockResolvedValue({data:[{id:'old',workspace_id:'ws',user_id:null,scope:'workspace',name:'Legacy variant',revision:1,primary:{connection_id:'missing',model:{provider:'openai',model:'gpt-old',controls:{}}},fallback:null}],error:null});
  await render('workspace');
  expect(document.querySelector('button[aria-label="Models for unavailable connection missing"]')).not.toBeNull();
});
