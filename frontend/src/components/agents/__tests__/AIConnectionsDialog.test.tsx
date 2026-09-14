// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { AIConnectionsDialog } from '../AIConnectionsDialog';
import { aiConnectionService } from '@/lib/services/aiConnectionService';

vi.mock('@/lib/services/aiConnectionService', () => ({
  aiConnectionService: { endpoints: vi.fn(), create: vi.fn() },
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
  client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  vi.mocked(aiConnectionService.endpoints).mockResolvedValue({ data: [{ id: 'local', base_url: 'http://localhost:8181/v1', auth_mode: 'none' }], error: null });
  vi.mocked(aiConnectionService.create).mockResolvedValue({ data: null, error: 'test response' });
});
afterEach(() => {
  act(() => root.unmount()); client.clear(); document.body.innerHTML = ''; vi.unstubAllGlobals(); vi.clearAllMocks();
});
async function flush() { await act(async () => { await new Promise(resolve => setTimeout(resolve, 0)); }); }
it('saves an approved no-auth endpoint without a key or browser-supplied URL', async () => {
  await act(async () => root.render(<QueryClientProvider client={client}>
    <AIConnectionsDialog workspaceId="ws" scope="workspace" open onOpenChange={() => {}} connections={[]} models={[]} onChanged={() => {}} />
  </QueryClientProvider>));
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
  expect(aiConnectionService.create).toHaveBeenCalledWith('ws', { name: 'Local model', scope: 'workspace', provider: 'openai_compatible', endpoint_id: 'local', api_key: undefined });
});
