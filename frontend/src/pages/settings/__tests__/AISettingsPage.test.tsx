// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { AISettingsPage } from '../AISettingsPage';
import { TooltipProvider } from '@/components/ui/tooltip';
import { aiConnectionService } from '@/lib/services/aiConnectionService';
import { aiUsagePricingText } from '@edition/ai';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import type { AIExecutionPolicySnapshot } from '@/lib/services/aiConnectionService';
import { aiProfileService } from '@/lib/services/aiProfileService';

vi.mock('@/lib/services/aiConnectionService', () => ({
  aiConnectionService: { list: vi.fn(), endpoints: vi.fn(), create: vi.fn(), reconnect: vi.fn(), poll: vi.fn(), disconnect: vi.fn(), models: vi.fn(), refreshModels: vi.fn() },
}));
vi.mock('@/lib/services/aiProfileService', () => ({
  aiProfileService: { list: vi.fn(), save: vi.fn(), remove: vi.fn(), settings: vi.fn(), setDefault: vi.fn(), setPersonalDefault: vi.fn(), setVisibility: vi.fn(), enableModel: vi.fn() },
}));
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

const frame = vi.hoisted(() => ({ permissions: new Set<string>(['workspace.update']) }));
vi.mock('../SettingsPageFrame', () => ({
  SettingsPageFrame: ({ section, descriptionSuffix, children }: { section: string; descriptionSuffix?: string; children: (ctx: unknown) => React.ReactNode }) => (
    <div>
      <h1>{section === 'ai' ? 'AI' : 'AI connections'}</h1>
      <p>
        {section === 'ai'
          ? 'Shared connections, profiles, and the default profile agents inherit.'
          : 'Your API keys and ChatGPT login, plus the profiles that use them.'}
        {descriptionSuffix}
      </p>
      {children({ workspaceId: 'ws', permissions: { has: (perm: string) => frame.permissions.has(perm) } })}
    </div>
  ),
}));
(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

let root: Root;
let client: QueryClient;

beforeEach(() => {
  useWorkspaceStore.setState({ currentWorkspace: { id: 'ws', slug: 'test', name: 'Test' } });
  vi.stubGlobal('ResizeObserver', class { observe() {} unobserve() {} disconnect() {} });
  HTMLElement.prototype.scrollIntoView = vi.fn();
  const container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
  client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  frame.permissions = new Set(['workspace.update']);
  vi.mocked(aiProfileService.list).mockResolvedValue({ data: [], error: null });
  vi.mocked(aiProfileService.settings).mockResolvedValue({ data: { default_profile_id: null }, error: null });
  vi.mocked(aiConnectionService.refreshModels).mockImplementation((workspace, id) => aiConnectionService.models(workspace, id));
});

afterEach(() => {
  act(() => root.unmount());
  client.clear();
  useWorkspaceStore.setState({ currentWorkspace: null });
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
  expect(document.body.textContent).toContain('No models added');
  expect(document.body.textContent).not.toContain('Create profile');
  expect(document.body.textContent).not.toContain('No profiles yet');
});

it('gives members personal setup and read-only workspace models without scope tabs', async () => {
  frame.permissions = new Set();
  vi.mocked(aiConnectionService.list).mockResolvedValue({ data: { enabled: true, connections: [], models: [] }, error: null });
  await render('workspace');
  expect(document.querySelector('[role="tab"]')).toBeNull();
  expect(document.body.textContent).toContain('Add models');
  expect(Array.from(document.querySelectorAll('button')).some(b => b.textContent === 'Add models' && !b.disabled)).toBe(true);
});

it('shows a unified model table without scope tabs for admins', async () => {
  vi.mocked(aiConnectionService.list).mockResolvedValue({ data: { enabled: true, connections: [], models: [] }, error: null });
  await render('personal');
  expect(document.querySelector('[role="tab"]')).toBeNull();
  expect(document.body.textContent).toContain('Add models');
});

it('opens model selection on a connection and enables a discovered model without naming a profile', async () => {
  vi.mocked(aiConnectionService.list).mockResolvedValue({ data: { enabled:true, connections:[{id:'c1', name:'Team key', provider:'openai', scope:'workspace', user_id:null, status:'connected'}], models:[] }, error:null });
  vi.mocked(aiConnectionService.models).mockResolvedValue({data:{models:[{id:'gpt-new', name:'New model'}],source:'provider',stale:false},error:null});
  vi.mocked(aiProfileService.enableModel).mockResolvedValue({data:null,error:null});
  await render('workspace');
  await act(async () => { Array.from(document.querySelectorAll('button')).find(b => b.textContent === 'Add models')?.click(); });
  for (let i=0;i<5;i++) await act(async () => {await new Promise(r => setTimeout(r,0));});
  await act(async () => { document.querySelector<HTMLButtonElement>('[aria-label="Choose OpenAI"]')?.click(); });
  await act(async () => { document.querySelector<HTMLButtonElement>('[aria-label="Models for Team key"]')?.click(); });
  for (let i=0;i<5;i++) await act(async () => {await new Promise(r => setTimeout(r,0));});
  const toggle=document.querySelector<HTMLButtonElement>('[role="switch"][aria-label="Show New model in model pickers"]');
  expect(toggle).not.toBeNull();
  expect(aiConnectionService.refreshModels).toHaveBeenCalledWith('ws', 'c1');
  await act(async () => toggle!.click());
  expect(aiProfileService.enableModel).toHaveBeenCalledWith('ws', {name:'New model',connection_id:'c1',model:'gpt-new'});
  expect(document.body.textContent).not.toContain('Profile name');
});

it('keeps existing configurations reachable when their connection is unavailable', async () => {
  vi.mocked(aiConnectionService.list).mockResolvedValue({data:{enabled:true,connections:[],models:[]},error:null});
  vi.mocked(aiProfileService.list).mockResolvedValue({data:[{id:'old',workspace_id:'ws',user_id:null,scope:'workspace',name:'Legacy variant',revision:1,primary:{connection_id:'missing',model:{provider:'openai',model:'gpt-old',controls:{}}},fallback:null}],error:null});
  await render('workspace');
  expect(document.body.textContent).toContain('Legacy variant');
  expect(document.body.textContent).toContain('Connection missing');
});

it('retries the provider when an automatic refresh returns a cached list with a warning', async () => {
  vi.mocked(aiConnectionService.list).mockResolvedValue({ data: { enabled: true, connections: [
    { id: 'refresh', name: 'Refresh key', provider: 'openai', scope: 'personal', user_id: 'me', status: 'connected' },
  ], models: [] }, error: null });
  vi.mocked(aiConnectionService.models).mockResolvedValue({ data: { models: [], source: 'provider', stale: false }, error: null });
  vi.mocked(aiConnectionService.refreshModels)
    .mockResolvedValueOnce({ data: { models: [], source: 'provider', stale: true, warning: 'Could not refresh models.' }, error: null })
    .mockResolvedValueOnce({ data: { models: [{ id: 'new-id', name: 'Fresh model' }], source: 'provider', stale: false }, error: null });
  await render('personal');
  await act(async () => { Array.from(document.querySelectorAll('button')).find(button => button.textContent === 'Manage connections')?.click(); });
  await act(async () => document.querySelector<HTMLButtonElement>('[aria-label="Models for Refresh key"]')?.click());
  for (let i = 0; i < 5; i++) await act(async () => { await new Promise(resolve => setTimeout(resolve, 0)); });
  expect(document.body.textContent).toContain('Could not refresh models.');
  await act(async () => { Array.from(document.querySelectorAll('button')).find(button => button.textContent?.trim() === 'Retry')?.click(); });
  for (let i = 0; i < 5; i++) await act(async () => { await new Promise(resolve => setTimeout(resolve, 0)); });
  expect(aiConnectionService.refreshModels).toHaveBeenCalledTimes(2);
  expect(document.body.textContent).toContain('Fresh model');
});

it('lets a ChatGPT connection add a model by its exact runtime ID', async () => {
  vi.mocked(aiConnectionService.list).mockResolvedValue({ data: { enabled: true, connections: [
    { id: 'chatgpt', name: 'My ChatGPT', provider: 'openai_chatgpt', scope: 'personal', user_id: 'me', status: 'connected' },
  ], models: [] }, error: null });
  vi.mocked(aiConnectionService.models).mockResolvedValue({ data: { models: [], source: 'catalog', stale: false }, error: null });
  vi.mocked(aiProfileService.enableModel).mockResolvedValue({ data: null, error: null });
  await render('personal');
  await act(async () => { Array.from(document.querySelectorAll('button')).find(button => button.textContent === 'Manage connections')?.click(); });
  await act(async () => document.querySelector<HTMLButtonElement>('[aria-label="Models for My ChatGPT"]')?.click());
  for (let i = 0; i < 5; i++) await act(async () => { await new Promise(resolve => setTimeout(resolve, 0)); });
  const input = document.querySelector<HTMLInputElement>('[aria-label="Model ID"]');
  expect(input).not.toBeNull();
  expect(aiConnectionService.refreshModels).toHaveBeenCalledWith('ws', 'chatgpt');
  await act(async () => {
    Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!.call(input, 'future-chatgpt-model');
    input!.dispatchEvent(new Event('input', { bubbles: true }));
  });
  await act(async () => input!.closest('form')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true })));
  expect(aiProfileService.enableModel).toHaveBeenCalledWith('ws', {
    connection_id: 'chatgpt', model: 'future-chatgpt-model', name: 'future-chatgpt-model',
  });
});

it('opens Manage connections without automatically opening a connection tooltip', async () => {
  vi.mocked(aiConnectionService.list).mockResolvedValue({ data: { enabled: true, connections: [
    { id: 'team', name: 'Team key', provider: 'openai', scope: 'workspace', user_id: null, status: 'connected' },
  ], models: [] }, error: null });
  await render('workspace');
  await act(async () => {
    Array.from(document.querySelectorAll('button')).find(button => button.textContent === 'Manage connections')?.click();
  });
  expect(document.activeElement).toBe(document.querySelector('[role="dialog"]'));
  expect(document.querySelector('[role="tooltip"]')).toBeNull();
  const details = document.querySelector<HTMLButtonElement>('[aria-label="Connection details for Team key"]');
  await act(async () => details?.focus());
  expect(document.querySelector('[role="tooltip"]')).not.toBeNull();
});


it('keeps hidden models in settings and marks the effective default beside the model', async () => {
  vi.mocked(aiConnectionService.list).mockResolvedValue({data:{enabled:true,connections:[
    {id:'team',name:'Team key',provider:'openai',scope:'workspace',user_id:null,status:'connected'},
    {id:'mine',name:'My key',provider:'anthropic',scope:'personal',user_id:'me',status:'disconnected'},
  ],models:[]},error:null});
  vi.mocked(aiProfileService.list).mockResolvedValue({data:[
    {id:'team-model',workspace_id:'ws',scope:'workspace',user_id:null,name:'Shared model',revision:1,primary:{connection_id:'team',model:{provider:'openai',model:'gpt-test',controls:{}}},fallback:null},
    {id:'my-model',workspace_id:'ws',scope:'personal',user_id:'me',name:'My model',revision:1,primary:{connection_id:'mine',model:{provider:'anthropic',model:'claude-test',controls:{}}},fallback:null},
    {id:'hidden',workspace_id:'ws',scope:'personal',user_id:'me',name:'Hidden model',hidden_from_ask_agent:true,revision:1,primary:{connection_id:'mine',model:{provider:'anthropic',model:'old',controls:{}}},fallback:null},
  ],error:null});
  vi.mocked(aiProfileService.settings).mockResolvedValue({data:{default_profile_id:'team-model',personal_default_profile_id:'my-model'},error:null});
  await render('workspace');
  expect(Array.from(document.querySelectorAll('thead th')).map(el=>el.textContent)).toEqual(['Model','Connection','Access','Status','Actions']);
  expect(document.body.textContent).toContain('Shared model');
  expect(document.body.textContent).toContain('My model');
  expect(document.body.textContent).toContain('Hidden model');
  const hiddenRow = Array.from(document.querySelectorAll('tbody tr')).find(el => el.textContent?.includes('Hidden model'));
  expect(hiddenRow?.textContent).toContain('Hidden from pickers');
  expect(document.body.textContent).toContain('Workspace-wide');
  const row=Array.from(document.querySelectorAll('tbody tr')).find(el=>el.textContent?.includes('My model'));
  expect(row?.querySelector('td')?.textContent).toContain('Default');
});

it('puts recent discoveries before saved models and older choices, without marking the baseline or expired discoveries new', async () => {
  const now = Date.now();
  vi.mocked(aiConnectionService.list).mockResolvedValue({ data: { enabled: true, connections: [
    { id: 'discoveries', name: 'Discovery key', provider: 'openai', scope: 'workspace', user_id: null, status: 'connected' },
  ], models: [] }, error: null });
  vi.mocked(aiProfileService.list).mockResolvedValue({ data: [
    { id: 'saved', workspace_id: 'ws', user_id: null, scope: 'workspace', name: 'Saved model', revision: 1, primary: { connection_id: 'discoveries', model: { provider: 'openai', model: 'gpt-saved', controls: {} } }, fallback: null },
  ], error: null });
  vi.mocked(aiConnectionService.models).mockResolvedValue({ data: { models: [
    { id: 'gpt-baseline', name: 'Baseline model' },
    { id: 'gpt-expired', name: 'Older discovery', discovered_at: new Date(now - 8 * 86400000).toISOString() },
    { id: 'gpt-recent', name: 'Recent discovery', discovered_at: new Date(now - 86400000).toISOString() },
    { id: 'gpt-latest', name: 'Latest discovery', discovered_at: new Date(now - 1000).toISOString() },
  ], source: 'provider', stale: false }, error: null });
  await render('workspace');
  await act(async () => { Array.from(document.querySelectorAll('button')).find(button => button.textContent === 'Manage connections')?.click(); });
  await act(async () => document.querySelector<HTMLButtonElement>('[aria-label="Models for Discovery key"]')?.click());
  for (let i = 0; i < 5; i++) await act(async () => { await new Promise(resolve => setTimeout(resolve, 0)); });
  const dialog = Array.from(document.querySelectorAll('[role="dialog"]')).find(el => el.textContent?.includes('Discovery key models'))!;
  const toggles = Array.from(dialog.querySelectorAll('[role="switch"]'));
  expect(toggles.map(el => el.getAttribute('aria-label'))).toEqual([
    'Show Latest discovery in model pickers', 'Show Recent discovery in model pickers', 'Show Saved model in model pickers', 'Show Baseline model in model pickers', 'Show Older discovery in model pickers',
  ]);
  expect(Array.from(dialog.querySelectorAll('span')).filter(el => el.textContent === 'New')).toHaveLength(2);
});

it('shows common pricing in the page subtext without repeating usage tooltips on models or connections', async () => {
  const pricing = ([
    { mode: 'community', funding_mode: 'customer_unbilled' },
    { mode: 'ee', funding_mode: 'customer_funded_flat', flat_tariff: { version: 'v1', currency: 'USD', microusd_per_million: 0, accounting_version: 'v1' } },
  ] as AIExecutionPolicySnapshot[]).find(policy => aiUsagePricingText(policy))!;
  const expected = aiUsagePricingText(pricing)!;
  const policy = { allowed: true, pricing };
  vi.mocked(aiConnectionService.list).mockResolvedValue({ data: { enabled: true, connections: [
    { id: 'priced', name: 'My provider', provider: 'openai', scope: 'personal', user_id: 'me', status: 'connected', funding: 'customer', policy },
  ], models: [] }, error: null });
  vi.mocked(aiProfileService.list).mockResolvedValue({ data: [
    { id: 'priced-model', workspace_id: 'ws', user_id: 'me', scope: 'personal', name: 'My model', revision: 1, primary: { connection_id: 'priced', model: { provider: 'openai', model: 'gpt-model', controls: {} } }, fallback: null, primary_policy: policy },
  ], error: null });
  await render('personal');
  expect(document.body.textContent).toContain(expected);
  expect(document.querySelector('[aria-label="Usage details for My model"]')).toBeNull();
  await act(async () => { Array.from(document.querySelectorAll('button')).find(button => button.textContent === 'Manage connections')?.click(); });
  expect(document.querySelector('[aria-label="Usage details"]')).toBeNull();
});
