// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { queryKeys } from '@/lib/queryKeys';
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
  aiProfileService: { list: vi.fn(), save: vi.fn(), remove: vi.fn(), settings: vi.fn(), setDefault: vi.fn(), setPersonalDefault: vi.fn(), setVisibility: vi.fn(), setAgentSharing: vi.fn(), enableModel: vi.fn() },
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

async function openConnectionList() {
  expect(document.querySelector('[aria-label="AI model settings actions"]')).toBeNull();
  await act(async () => Array.from(document.querySelectorAll('button')).find(b => b.textContent === 'Add models')!.click());
  expect(document.querySelector('[role="dialog"]')?.textContent).not.toContain('Manage connections');
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
  expect(document.querySelector('[role="dialog"]')?.textContent).toContain("Choose models");
  await act(async () => { document.querySelector<HTMLButtonElement>('[aria-label="Models for Team key"]')?.click(); });
  for (let i=0;i<5;i++) await act(async () => {await new Promise(r => setTimeout(r,0));});
  const add=document.querySelector<HTMLButtonElement>('[aria-label="Add New model"]');
  expect(add).not.toBeNull();
  vi.mocked(aiProfileService.list).mockResolvedValue({ data: [{
    id:'saved',workspace_id:'ws',scope:'workspace',user_id:null,name:'New model',revision:1,
    primary:{connection_id:'c1',model:{provider:'openai',model:'gpt-new',controls:{}}},fallback:null,
  }],error:null });
  expect(aiConnectionService.refreshModels).toHaveBeenCalledWith('ws', 'c1');
  await act(async () => add!.click());
  await vi.waitFor(() => expect(document.querySelector('[role="dialog"] [aria-label="Added New model"]')).not.toBeNull());
  expect(aiProfileService.enableModel).toHaveBeenCalledWith('ws', {name:'New model',connection_id:'c1',model:'gpt-new'});
  expect(document.body.textContent).not.toContain('Profile name');
});

it('keeps existing configurations reachable when their connection is unavailable', async () => {
  vi.mocked(aiConnectionService.list).mockResolvedValue({data:{enabled:true,connections:[],models:[]},error:null});
  vi.mocked(aiProfileService.list).mockResolvedValue({data:[{id:'old',workspace_id:'ws',user_id:null,scope:'workspace',name:'Legacy variant',revision:1,primary:{connection_id:'missing',model:{provider:'openai',model:'gpt-old',controls:{}}},fallback:null}],error:null});
  await render('workspace');
  expect(document.body.textContent).toContain('Legacy variant');
  expect(document.body.textContent).toContain('Connection missing');
  const actions = document.querySelector<HTMLButtonElement>('[aria-label="Actions for Legacy variant"]')!;
  await act(async () => actions.dispatchEvent(new KeyboardEvent('keydown', {key:'Enter',bubbles:true})));
  await act(async () => Array.from(document.querySelectorAll<HTMLElement>('[role="menuitem"]')).find(item => item.textContent === 'Model settings')!.click());
  expect(document.querySelector('[role="dialog"]')?.textContent).toContain('Edit “Legacy variant”');
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
  await openConnectionList();
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
  await openConnectionList();
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

it.each([
  ['openai', 'Replace API key'],
  ['openai_chatgpt', 'Reconnect'],
])('opens credential settings for a connected %s account', async (provider, action) => {
  vi.mocked(aiConnectionService.list).mockResolvedValue({ data: { enabled: true, connections: [
    { id: 'mine', name: 'My account', provider, scope: 'personal', user_id: 'me', status: 'connected' },
  ], models: [] }, error: null });
  await render('personal');
  await openConnectionList();
  const menu = document.querySelector<HTMLButtonElement>('[aria-label="Connection actions for My account"]')!;
  await act(async () => menu.dispatchEvent(new KeyboardEvent('keydown', {key:'Enter',bubbles:true})));
  await act(async () => Array.from(document.querySelectorAll<HTMLElement>('[role="menuitem"]')).find(item => item.textContent === action)!.click());
  expect(document.querySelector('[role="dialog"]')?.textContent).toContain('Reconnect “My account”');
});

it.each([false, true])('labels workspace and personal defaults for members, including overlapping defaults (%s)', async (sameModel) => {
  frame.permissions = new Set();
  vi.mocked(aiConnectionService.list).mockResolvedValue({data:{enabled:true,connections:[
    {id:'team',name:'Team key',provider:'openai',scope:'workspace',user_id:null,status:'connected'},
    {id:'mine',name:'My key',provider:'anthropic',scope:'personal',user_id:'me',status:'disconnected'},
  ],models:[]},error:null});
  vi.mocked(aiProfileService.list).mockResolvedValue({data:[
    {id:'team-model',workspace_id:'ws',scope:'workspace',user_id:null,name:'Shared model',revision:1,primary:{connection_id:'team',model:{provider:'openai',model:'gpt-test',controls:{}}},fallback:null},
    {id:'my-model',workspace_id:'ws',scope:'personal',user_id:'me',name:'My model',revision:1,primary:{connection_id:'mine',model:{provider:'anthropic',model:'claude-test',controls:{}}},fallback:null},
    {id:'hidden',workspace_id:'ws',scope:'personal',user_id:'me',name:'Hidden model',hidden_from_ask_agent:true,revision:1,primary:{connection_id:'mine',model:{provider:'anthropic',model:'old',controls:{}}},fallback:null},
  ],error:null});
  vi.mocked(aiProfileService.settings).mockResolvedValue({data:{default_profile_id:'team-model',personal_default_profile_id:sameModel ? 'team-model' : 'my-model'},error:null});
  await render('workspace');
  expect(Array.from(document.querySelectorAll('thead th')).map(el=>el.textContent)).toEqual(['Model','Connection','Access','Defaults','Status','Actions']);
  expect(document.body.textContent).toContain('Shared model');
  expect(document.body.textContent).toContain('My model');
  expect(document.body.textContent).toContain('Hidden model');
  const hiddenRow = Array.from(document.querySelectorAll('tbody tr')).find(el => el.textContent?.includes('Hidden model'));
  expect(hiddenRow?.textContent).toContain('Hidden from pickers');
  expect(document.body.textContent).toContain('Workspace-wide');
  const row=Array.from(document.querySelectorAll('tbody tr')).find(el=>el.textContent?.includes('My model'));
  const teamRow = Array.from(document.querySelectorAll('tbody tr')).find(el => el.textContent?.includes('Shared model'));
  expect(teamRow?.querySelector('td:nth-child(4)')?.textContent).toContain('Workspace default');
  expect(teamRow?.querySelector('td:nth-child(4)')?.textContent?.includes('Your default')).toBe(sameModel);
  expect(row?.querySelector('td:nth-child(4)')?.textContent?.includes('Your default')).toBe(!sameModel);
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
  await openConnectionList();
  await act(async () => document.querySelector<HTMLButtonElement>('[aria-label="Models for Discovery key"]')?.click());
  for (let i = 0; i < 5; i++) await act(async () => { await new Promise(resolve => setTimeout(resolve, 0)); });
  const dialog = Array.from(document.querySelectorAll('[role="dialog"]')).find(el => el.textContent?.includes('Discovery key models'))!;
  const toggles = Array.from(dialog.querySelectorAll('[aria-label^="Add "],[aria-label^="Added "]'));
  expect(toggles.map(el => el.getAttribute('aria-label'))).toEqual([
    'Add Latest discovery', 'Add Recent discovery', 'Added Saved model', 'Add Baseline model', 'Add Older discovery',
  ]);
  expect(Array.from(dialog.querySelectorAll('span')).filter(el => el.textContent === 'New')).toHaveLength(2);
});

it('shows common pricing below the model table without repeating usage tooltips on models or connections', async () => {
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
  const usage = document.querySelector('[aria-label="Model usage"]')!;
  expect(usage.textContent).toContain(expected);
  expect(document.querySelector('table')!.compareDocumentPosition(usage) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  expect(document.body.textContent?.split(expected)).toHaveLength(2);
  expect(document.querySelector('[aria-label="Usage details for My model"]')).toBeNull();
  await openConnectionList();
  expect(document.querySelector('[aria-label="Usage details"]')).toBeNull();
});

it('requires explicit owner confirmation to share one ChatGPT model', async () => {
  frame.permissions = new Set();
  const connection = {id:'mine', name:'My subscription', provider:'openai_chatgpt', scope:'personal', user_id:'me', status:'connected'} as const;
  const profile = {id:'model', workspace_id:'ws', scope:'personal', user_id:'me', name:'GPT', revision:2, primary:{connection_id:'mine', model:{provider:'openai_chatgpt',model:'gpt-test',controls:{}}}, fallback:null} as const;
  vi.mocked(aiConnectionService.list).mockResolvedValue({data:{enabled:true, connections:[connection], models:[]}, error:null});
  vi.mocked(aiProfileService.list).mockResolvedValue({data:[profile], error:null});
  vi.mocked(aiProfileService.setAgentSharing).mockResolvedValue({data:{...profile,agent_grant_id:'grant'}, error:null});
  await render('personal');
  const actions = document.querySelector<HTMLButtonElement>('[aria-label="Access for GPT: Personal"]')!;
  await act(async () => { actions.dispatchEvent(new KeyboardEvent('keydown', {key:'Enter',bubbles:true})); });
  const share = Array.from(document.querySelectorAll<HTMLElement>('[role="menuitem"]')).find(item => item.textContent === 'Workspace-wide');
  expect(share).toBeDefined();
  await act(async () => share!.click());
  expect(document.querySelector('[role="dialog"]')?.textContent).toContain('My subscription');
  expect(aiProfileService.setAgentSharing).not.toHaveBeenCalled();
  await act(async () => Array.from(document.querySelectorAll('button')).find(button => button.textContent === 'Share model')!.click());
  expect(aiProfileService.setAgentSharing).toHaveBeenCalledWith('ws',profile,true);
});

it('shows the status and permitted actions for each connection independently', async () => {
  frame.permissions = new Set();
  vi.mocked(aiConnectionService.list).mockResolvedValue({data:{enabled:true,models:[],connections:[
    {id:'managed',name:'openai (managed)',scope:'workspace',provider:'openai',funding:'managed',user_id:null,status:'unconfigured'},
    {id:'shared',name:'Team key',scope:'workspace',provider:'openai',user_id:null,status:'connected'},
    {id:'personal',name:'My key',scope:'personal',provider:'openai',user_id:'me',status:'connected',last_verification_error:'The last test failed.'},
    {id:'subscription',name:'Subscription',scope:'personal',provider:'openai_chatgpt',user_id:'me',status:'pending',account_id:'private-prefix-12345678'},
  ]},error:null});
  await render('personal');
  await act(async () => Array.from(document.querySelectorAll('button')).find(b => b.textContent === 'Add models')!.click());
  const dialog = document.querySelector('[role="dialog"]')!;
  expect(dialog.querySelector('[aria-label="Models for OpenAI"]')?.textContent).toContain('Not configured');
  expect(dialog.querySelector('[aria-label="Models for Team key"]')?.textContent).toContain('Connected');
  expect(dialog.querySelector('[aria-label="Models for Team key"]')?.textContent).toContain('View models');
  expect(dialog.querySelector('[aria-label="Models for My key"]')?.textContent).toContain('Last test failed');
  expect(dialog.querySelector('[aria-label="Continue login for Subscription"]')?.textContent).toContain('Awaiting login');
  expect(dialog.textContent).toContain('···· 12345678');
  expect(dialog.textContent).not.toContain('private-prefix');
  expect(dialog.querySelectorAll('[aria-label^="Connection actions for"]')).toHaveLength(2);
});

it('confirms the affected shared model before disconnecting the selected subscription', async () => {
  const connection = {id:'sub',name:'Personal account',scope:'personal',provider:'openai_chatgpt',status:'connected',user_id:'me'} as const;
  vi.mocked(aiConnectionService.list).mockResolvedValue({data:{enabled:true,connections:[connection],models:[]},error:null});
  vi.mocked(aiProfileService.list).mockResolvedValue({data:[{
    id:'selected', workspace_id:'ws',user_id:'me',scope:'personal',name:'Planner model',revision:1,agent_grant_id:'grant',
    primary:{connection_id:'sub',model:{provider:'openai_chatgpt',model:'gpt-test',controls:{}}},fallback:null,
  }],error:null});
  const confirm = vi.fn().mockReturnValueOnce(false).mockReturnValueOnce(true);
  vi.stubGlobal('confirm',confirm);
  vi.mocked(aiConnectionService.disconnect).mockResolvedValue({data:undefined,error:null});
  await render('personal');
  await act(async () => Array.from(document.querySelectorAll('button')).find(b => b.textContent === 'Add models')!.click());
  async function disconnect() {
    const menu=document.querySelector<HTMLButtonElement>('[aria-label="Connection actions for Personal account"]')!;
    await act(async () => menu.dispatchEvent(new KeyboardEvent('keydown',{key:'Enter',bubbles:true})));
    await act(async () => Array.from(document.querySelectorAll<HTMLElement>('[role="menuitem"]')).find(item => item.textContent === 'Disconnect')!.click());
  }
  await disconnect();
  expect(confirm).toHaveBeenCalledWith(expect.stringContaining('Planner model (Workspace-wide)'));
  expect(aiConnectionService.disconnect).not.toHaveBeenCalled();
  await disconnect();
  expect(aiConnectionService.disconnect).toHaveBeenCalledWith('ws','sub');
});

it.each(['main table', 'model chooser', 'another saved variant', 'agent dependency'])('handles Remove model from the %s', async (entry) => {
  const connection = { id:'team', name:'Team key', provider:'openai', scope:'workspace', user_id:null, status:'connected' } as const;
  const profile = { id:'p', name:'Planning model', workspace_id:'ws', scope:'workspace', user_id:null, revision:3,
    primary:{connection_id:'team',model:{provider:'openai',model:'gpt-test',controls:{}}}, fallback:null } as const;
  const second = { ...profile, id:'second', name:'Another configuration' };
  const inChooser = entry === 'model chooser' || entry === 'another saved variant';
  let saved: (typeof second)[] = entry === 'another saved variant' ? [profile, second] : [profile];
  vi.mocked(aiProfileService.list).mockImplementation(async () => ({data:saved,error:null}));
  vi.mocked(aiProfileService.remove).mockImplementation(async (_, removed) => { saved=saved.filter(p => p.id !== removed.id); return {data:undefined,error:null}; });
  vi.mocked(aiConnectionService.list).mockResolvedValue({data:{enabled:true,connections:[connection],models:[]},error:null});
  vi.mocked(aiConnectionService.models).mockResolvedValue({data:{source:'provider',models:[{id:'gpt-test',name:'Test model'}],stale:false},error:null});
  client.setQueryData(queryKeys.automation.agents('ws'), entry === 'agent dependency' ? [{id:'a',name:'Review assistant',ai_profile_id:'p'}] : []);
  await render('workspace');
  if (inChooser) {
    await act(async () => Array.from(document.querySelectorAll('button')).find(b => b.textContent === 'Add models')!.click());
    await act(async () => document.querySelector<HTMLButtonElement>('[aria-label="Models for Team key"]')!.click());
  }
  const surface = document.querySelector('[role="dialog"]') ?? document;
  const trigger = surface.querySelector<HTMLButtonElement>('[aria-label="Actions for Planning model"]')!;
  await act(async () => trigger.dispatchEvent(new KeyboardEvent('keydown',{key:'Enter',bubbles:true})));
  const remove = Array.from(document.querySelectorAll<HTMLElement>('[role="menuitem"]')).find(el => el.textContent === 'Remove model');
  expect(remove).toBeDefined();
  await act(async () => remove!.click());
  const confirmation=document.querySelector('[role="alertdialog"]')!;
  expect(confirmation.textContent).toContain('Planning model');
  expect(aiProfileService.remove).not.toHaveBeenCalled();
  if (entry === 'agent dependency') {
    expect(confirmation.textContent).toContain('Review assistant');
    expect(Array.from(confirmation.querySelectorAll('button')).some(b => b.textContent === 'Remove model' && !b.disabled)).toBe(false);
    return;
  }
  await act(async () => Array.from(confirmation.querySelectorAll('button')).find(b => b.textContent === 'Remove model')!.click());
  expect(aiProfileService.remove).toHaveBeenCalledWith('ws',profile);
  await vi.waitFor(() => expect(document.querySelector('[aria-label="Actions for Planning model"]')).toBeNull());
  if (inChooser) {
    const add = surface.querySelector('[aria-label="Add Test model"]');
    if (entry === 'another saved variant') {
      expect(surface.querySelector('[aria-label="Added Another configuration"]')).not.toBeNull();
      expect(add).toBeNull();
    } else {
      expect(add).not.toBeNull();
    }
  }
});
