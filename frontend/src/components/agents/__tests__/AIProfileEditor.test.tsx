// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { AIProfileEditor } from '../AIProfileEditor';
import { aiProfileService } from '@/lib/services/aiProfileService';
import type { AIConnection } from '@/lib/services/aiConnectionService';
import { TooltipProvider } from '@/components/ui/tooltip';
import { toast } from 'sonner';

vi.mock('@/lib/services/aiProfileService', () => ({
  aiProfileService: { save: vi.fn(), list: vi.fn(), remove: vi.fn(), settings: vi.fn(), setDefault: vi.fn() },
}));
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }));
(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

let root: Root;
let client: QueryClient;

const connection = (over: Partial<AIConnection> & { id: string }): AIConnection => ({
  scope: 'workspace',
  user_id: null,
  name: over.id,
  provider: 'openai',
  status: 'connected',
  ...over,
});

beforeEach(() => {
  vi.stubGlobal('ResizeObserver', class { observe() {} unobserve() {} disconnect() {} });
  HTMLElement.prototype.scrollIntoView = vi.fn();
  const container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
  client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  vi.mocked(aiProfileService.save).mockResolvedValue({ data: null, error: 'test response' });
});

afterEach(() => {
  act(() => root.unmount());
  client.clear();
  document.body.innerHTML = '';
  vi.unstubAllGlobals();
  vi.clearAllMocks();
});

async function render(node: React.ReactNode) {
  await act(async () => root.render(<QueryClientProvider client={client}><TooltipProvider>{node}</TooltipProvider></QueryClientProvider>));
}

function openConnectionPicker(index = 0) {
  const triggers = document.querySelectorAll<HTMLButtonElement>('button[id$="-connection"]');
  return act(async () => triggers[index].click());
}

it('keeps the fallback list inside the profile scope and off the primary connection', async () => {
  const connections = [
    connection({ id: 'shared-a', name: 'Shared A' }),
    connection({ id: 'shared-b', name: 'Shared B' }),
    connection({ id: 'mine', name: 'Mine', scope: 'personal', user_id: 'u1' }),
  ];
  await render(
    <AIProfileEditor
      workspaceId="ws"
      scope="workspace"
      connections={connections}
      onClose={() => {}}
    />,
  );
  await openConnectionPicker(0);
  expect(document.querySelector('[cmdk-item][data-value="mine"]')).toBeNull();
  await act(async () => document.querySelector<HTMLElement>('[cmdk-item][data-value="shared-a"]')!.click());

  await act(async () => {
    const buttons = Array.from(document.querySelectorAll('button'));
    buttons.find((button) => button.textContent === 'Add fallback')!.click();
  });
  await openConnectionPicker(1);
  expect(document.querySelector('[cmdk-item][data-value="shared-b"]')).not.toBeNull();
  expect(document.querySelector('[cmdk-item][data-value="shared-a"]')).toBeNull();
  expect(document.querySelector('[cmdk-item][data-value="mine"]')).toBeNull();
});

it('explains why a policy-blocked connection cannot be chosen', async () => {
  const connections = [
    connection({
      id: 'blocked',
      name: 'Blocked key',
      policy: { allowed: false, message: 'Customer keys are disabled for this workspace.' },
    }),
  ];
  await render(
    <AIProfileEditor workspaceId="ws" scope="workspace" connections={connections} onClose={() => {}} />,
  );
  await openConnectionPicker(0);
  const item = document.querySelector<HTMLElement>('[cmdk-item][data-value="blocked"]')!;
  expect(item.getAttribute('aria-disabled')).toBe('true');
  expect(document.body.textContent).toContain('Customer keys are disabled for this workspace.');
});

it('names the missing field instead of disabling the submit button', async () => {
  await render(
    <AIProfileEditor
      workspaceId="ws"
      scope="workspace"
      connections={[connection({ id: 'shared-a', name: 'Shared A' })]}
      onClose={() => {}}
    />,
  );
  const name = document.querySelector<HTMLInputElement>('input[id$="-name"]')!;
  await act(async () => {
    Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!.call(name, 'Daily driver');
    name.dispatchEvent(new Event('input', { bubbles: true }));
  });
  await act(async () => document.querySelector('form')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true })));
  expect(toast.error).toHaveBeenCalledWith('Choose a primary connection');

  await openConnectionPicker(0);
  await act(async () => document.querySelector<HTMLElement>('[cmdk-item][data-value="shared-a"]')!.click());
  await act(async () => document.querySelector('form')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true })));
  expect(toast.error).toHaveBeenCalledWith('Enter a primary model');
  expect(aiProfileService.save).not.toHaveBeenCalled();
});

it('saves the resolved route for a catalog model', async () => {
  vi.mocked(aiProfileService.save).mockResolvedValue({
    data: {
      id: 'p1',
      workspace_id: 'ws',
      user_id: null,
      scope: 'workspace',
      name: 'Daily driver',
      revision: 1,
      primary: { connection_id: 'shared-a', model: { provider: 'openai', model: 'gpt-5.6-luna', controls: {} } },
      fallback: null,
    },
    error: null,
  });
  await render(
    <AIProfileEditor
      workspaceId="ws"
      scope="workspace"
      connections={[connection({ id: 'shared-a', name: 'Shared A' })]}
      onClose={() => {}}
    />,
  );
  const name = document.querySelector<HTMLInputElement>('input[id$="-name"]')!;
  await act(async () => {
    Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!.call(name, 'Daily driver');
    name.dispatchEvent(new Event('input', { bubbles: true }));
  });
  await openConnectionPicker(0);
  await act(async () => document.querySelector<HTMLElement>('[cmdk-item][data-value="shared-a"]')!.click());
  await act(async () => document.querySelector<HTMLButtonElement>('button[id$="-model"]')!.click());
  const option = document.querySelector<HTMLElement>('[cmdk-item]')!;
  const chosen = option.getAttribute('data-value')!;
  await act(async () => option.click());
  await act(async () => document.querySelector('form')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true })));
  expect(aiProfileService.save).toHaveBeenCalledWith(
    'ws',
    expect.objectContaining({
      name: 'Daily driver',
      scope: 'workspace',
      primary: expect.objectContaining({
        connection_id: 'shared-a',
        model: expect.objectContaining({ provider: 'openai', model: chosen }),
      }),
    }),
    undefined,
  );
});
