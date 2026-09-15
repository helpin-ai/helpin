// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { TooltipProvider } from '@/components/ui/tooltip';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { AIConnectionRow } from '../AIConnectionRow';
import { aiConnectionService, type AIConnection } from '@/lib/services/aiConnectionService';

vi.mock('@/lib/services/aiConnectionService', () => ({
  aiConnectionService: { disconnect: vi.fn() },
}));
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }));
const confirmResult = { value: true };
vi.mock('@/components/ui/confirm-dialog', () => ({
  useConfirm: () => async () => confirmResult.value,
}));
(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

let root: Root;
let client: QueryClient;

const base: AIConnection = {
  id: 'c1',
  scope: 'workspace',
  user_id: null,
  name: 'Team key',
  provider: 'openai',
  status: 'connected',
};

beforeEach(() => {
  const container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
  client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  confirmResult.value = true;
  vi.mocked(aiConnectionService.disconnect).mockResolvedValue({ data: undefined, error: null });
});

afterEach(() => {
  act(() => root.unmount());
  client.clear();
  document.body.innerHTML = '';
  vi.clearAllMocks();
});

async function render(
  connection: AIConnection,
  handlers: Partial<{ onReconnect: () => void; onContinueLogin: () => void }> = {},
) {
  await act(async () =>
    root.render(
      <QueryClientProvider client={client}>
        <TooltipProvider>
          <table>
            <tbody>
              <AIConnectionRow
                workspaceId="ws"
                connection={connection}
                canManage
                onReconnect={handlers.onReconnect ?? (() => {})}
                onContinueLogin={handlers.onContinueLogin ?? (() => {})}
              />
            </tbody>
          </table>
        </TooltipProvider>
      </QueryClientProvider>,
    ),
  );
}

function action(label: string) {
  return document.querySelector<HTMLButtonElement>(`button[aria-label="${label}"]`);
}

it('labels the status in words and names the provider', async () => {
  await render({ ...base, status: 'reauthorization_required' });
  expect(document.body.textContent).toContain('Reconnect required');
  expect(document.body.textContent).not.toContain('reauthorization_required');
  expect(document.body.textContent).toContain('OpenAI API key');
});

it('shows the provider mark for the connection', async () => {
  await render(base);
  expect(document.querySelector('[data-ai-provider-icon="openai"]')).not.toBeNull();
});

it('offers continue login while a device code is outstanding', async () => {
  const onContinueLogin = vi.fn();
  await render({ ...base, provider: 'openai_chatgpt', status: 'pending' }, { onContinueLogin });
  expect(document.body.textContent).toContain('Awaiting login');
  await act(async () => action('Continue login for Team key')!.click());
  expect(onContinueLogin).toHaveBeenCalled();
  expect(action('Reconnect Team key')).toBeNull();
});

it('hides actions for a managed connection and says who owns it', async () => {
  await render({ ...base, funding: 'managed' });
  expect(document.body.textContent).toContain('Managed by your administrator');
  expect(action('Disconnect Team key')).toBeNull();
  expect(action('Reconnect Team key')).toBeNull();
});

it('disconnects only after the confirmation is accepted', async () => {
  confirmResult.value = false;
  await render(base);
  await act(async () => action('Disconnect Team key')!.click());
  expect(aiConnectionService.disconnect).not.toHaveBeenCalled();

  confirmResult.value = true;
  await act(async () => action('Disconnect Team key')!.click());
  expect(aiConnectionService.disconnect).toHaveBeenCalledWith('ws', 'c1');
});
