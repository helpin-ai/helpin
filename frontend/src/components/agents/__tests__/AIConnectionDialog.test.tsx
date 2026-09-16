// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { AIConnectionDialog } from '../AIConnectionDialog';
import { aiConnectionService, type AIConnection } from '@/lib/services/aiConnectionService';
import { TooltipProvider } from '@/components/ui/tooltip';
import { helpinClient } from '@/lib/helpin';
import { toast } from 'sonner';

vi.mock('@/lib/services/aiConnectionService', () => ({
  aiConnectionService: { endpoints: vi.fn(), create: vi.fn(), reconnect: vi.fn(), poll: vi.fn() },
}));
vi.mock('@/lib/helpin', () => ({ helpinClient: { show: vi.fn(), open: vi.fn() } }));
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }));
(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

let root: Root;
let client: QueryClient;

beforeEach(() => {
  vi.stubGlobal('ResizeObserver', class { observe() {} unobserve() {} disconnect() {} });
  HTMLElement.prototype.scrollIntoView = vi.fn();
  const container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
  client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  vi.mocked(aiConnectionService.endpoints).mockResolvedValue({
    data: [{ id: 'local', base_url: 'http://localhost:8181/v1', auth_mode: 'none' }],
    error: null,
  });
  vi.mocked(aiConnectionService.create).mockResolvedValue({ data: null, error: 'test response' });
  vi.mocked(aiConnectionService.reconnect).mockResolvedValue({ data: null, error: 'test response' });
});

afterEach(() => {
  act(() => root.unmount());
  client.clear();
  document.body.innerHTML = '';
  vi.unstubAllGlobals();
  vi.clearAllMocks();
});

async function flush() {
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 0)); });
}

function render(ui: React.ReactNode) {
  return act(async () => root.render(<QueryClientProvider client={client}><TooltipProvider>{ui}</TooltipProvider></QueryClientProvider>));
}

it('saves an approved no-auth endpoint without a key or browser-supplied URL', async () => {
  await render(
    <AIConnectionDialog workspaceId="ws" scope="workspace" open mode={{ kind: 'add' }} models={[]} onOpenChange={() => {}} />,
  );
  await act(async () => document.querySelector<HTMLButtonElement>('[aria-label="Provider"]')!.click());
  await act(async () => document.querySelector<HTMLElement>('[cmdk-item][data-value="openai_compatible"]')!.click());
  for (let i = 0; i < 20 && !document.querySelector<HTMLButtonElement>('button[id$="-endpoint"]:not(:disabled)'); i++) await flush();
  await act(async () => document.querySelector<HTMLButtonElement>('button[id$="-endpoint"]')!.click());
  await act(async () => document.querySelector<HTMLElement>('[cmdk-item][data-value="local"]')!.click());
  expect(document.querySelector('input[type="password"]')).toBeNull();
  expect(document.body.textContent).toContain('No API key is sent.');
  const name = document.querySelector<HTMLInputElement>('input[id$="-name"]')!;
  await act(async () => {
    Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!.call(name, 'Local model');
    name.dispatchEvent(new Event('input', { bubbles: true }));
  });
  await act(async () => document.querySelector('form')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true })));
  expect(aiConnectionService.create).toHaveBeenCalledWith('ws', {
    name: 'Local model',
    scope: 'workspace',
    provider: 'openai_compatible',
    endpoint_id: 'local',
    api_key: undefined,
  });
});

it('reports the missing name instead of silently disabling submit', async () => {
  await render(
    <AIConnectionDialog
      workspaceId="ws"
      scope="workspace"
      open
      mode={{ kind: 'add' }}
      models={[{ provider: 'openai', selection_model: 'gpt-5.5', label: 'GPT', tier: 'large' }]}
      onOpenChange={() => {}}
    />,
  );
  const name = document.querySelector<HTMLInputElement>('input[id$="-name"]')!;
  await act(async () => {
    Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!.call(name, '   ');
    name.dispatchEvent(new Event('input', { bubbles: true }));
  });
  await act(async () => document.querySelector('form')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true })));
  expect(toast.error).toHaveBeenCalledWith('Name is required');
  expect(aiConnectionService.create).not.toHaveBeenCalled();
});

// The server duplicates OpenAI models under openai_chatgpt only when the
// ChatGPT feature flag is on, so an entry here means the flag is enabled.
const models = [
  { provider: 'openai', selection_model: 'gpt-5.5', label: 'GPT', tier: 'large' },
  { provider: 'openai_chatgpt', selection_model: 'gpt-5.5', label: 'GPT', tier: 'large' },
];

it('keeps ChatGPT out of workspace connections', async () => {
  await render(
    <AIConnectionDialog workspaceId="ws" scope="workspace" open mode={{ kind: 'add' }} models={models} onOpenChange={() => {}} />,
  );
  await act(async () => document.querySelector<HTMLButtonElement>('[aria-label="Provider"]')!.click());
  expect(document.querySelector('[cmdk-item][data-value="openai"]')).not.toBeNull();
  expect(document.querySelector('[cmdk-item][data-value="openai_chatgpt"]')).toBeNull();
});

it('offers ChatGPT for a personal connection', async () => {
  await render(
    <AIConnectionDialog workspaceId="ws" scope="personal" open mode={{ kind: 'add' }} models={models} onOpenChange={() => {}} />,
  );
  await act(async () => document.querySelector<HTMLButtonElement>('[aria-label="Provider"]')!.click());
  expect(document.querySelector('[cmdk-item][data-value="openai_chatgpt"]')).not.toBeNull();
});

it('reconnects an existing connection with a replacement key only', async () => {
  const connection: AIConnection = {
    id: 'c1',
    scope: 'workspace',
    user_id: null,
    name: 'Team key',
    provider: 'openai',
    status: 'reauthorization_required',
  };
  await render(
    <AIConnectionDialog
      workspaceId="ws"
      scope="workspace"
      open
      mode={{ kind: 'reconnect', connection }}
      models={[]}
      onOpenChange={() => {}}
    />,
  );
  expect(document.querySelector('input[id$="-name"]')).toBeNull();
  expect(document.body.textContent).toContain('Reconnect “Team key”');
  const key = document.querySelector<HTMLInputElement>('input[type="password"]')!;
  await act(async () => {
    Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!.call(key, 'sk-new');
    key.dispatchEvent(new Event('input', { bubbles: true }));
  });
  await act(async () => document.querySelector('form')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true })));
  expect(aiConnectionService.reconnect).toHaveBeenCalledWith('ws', 'c1', 'sk-new');
});

it('offers Helpin support when Cloud has no compatible endpoints', async () => {
  vi.mocked(aiConnectionService.endpoints).mockResolvedValue({ data: [], error: null });
  const close = vi.fn();
  await render(<AIConnectionDialog workspaceId="ws" scope="personal" open mode={{ kind: 'add' }} models={[]} onOpenChange={close} />);
  for (let i = 0; i < 20 && !document.body.textContent?.includes('No compatible endpoints'); i++) await flush();
  expect(document.body.textContent).toContain('No compatible endpoints are available.');
  expect(document.body.textContent).not.toContain('administrator must approve');
  const contact = [...document.querySelectorAll('button')].find(button => button.textContent === 'Contact Helpin support to request one')!;
  await act(async () => contact.click());
  expect(close).toHaveBeenCalledWith(false);
  expect(helpinClient.show).toHaveBeenCalledOnce();
  expect(helpinClient.open).toHaveBeenCalledOnce();
  expect(aiConnectionService.create).not.toHaveBeenCalled();
});
