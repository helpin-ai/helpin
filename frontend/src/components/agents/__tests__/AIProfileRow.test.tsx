// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { TooltipProvider } from '@/components/ui/tooltip';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { AIProfileRow } from '../AIProfileRow';
import { aiProfileService, type AIProfile } from '@/lib/services/aiProfileService';
import type { AIConnection } from '@/lib/services/aiConnectionService';

vi.mock('@/lib/services/aiProfileService', () => ({
  aiProfileService: { remove: vi.fn() },
}));
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }));
const confirmResult = { value: true };
vi.mock('@/components/ui/confirm-dialog', () => ({
  useConfirm: () => async () => confirmResult.value,
}));
(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

let root: Root;
let client: QueryClient;

const connection: AIConnection = {
  id: 'c1',
  scope: 'workspace',
  user_id: null,
  name: 'Team key',
  provider: 'openai',
  status: 'connected',
};

const profile: AIProfile = {
  id: 'p1',
  workspace_id: 'ws',
  user_id: null,
  scope: 'workspace',
  name: 'Daily driver',
  revision: 3,
  primary: { connection_id: 'c1', model: { provider: 'openai', model: 'gpt-5.6-luna', controls: {} } },
  fallback: null,
};

beforeEach(() => {
  const container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
  client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  confirmResult.value = true;
  vi.mocked(aiProfileService.remove).mockResolvedValue({ data: undefined, error: null });
});

afterEach(() => {
  act(() => root.unmount());
  client.clear();
  document.body.innerHTML = '';
  vi.clearAllMocks();
});

async function render(overrides: Partial<Parameters<typeof AIProfileRow>[0]> = {}) {
  await act(async () =>
    root.render(
      <QueryClientProvider client={client}>
        <TooltipProvider>
          <table>
            <tbody>
              <AIProfileRow
                workspaceId="ws"
                profile={profile}
                connections={[connection]}
                isDefault={false}
                canManage
                canSetDefault
                onEdit={() => {}}
                onSetDefault={() => {}}
                {...overrides}
              />
            </tbody>
          </table>
        </TooltipProvider>
      </QueryClientProvider>,
    ),
  );
}

it('summarises the route with provider name, model and connection', async () => {
  await render();
  expect(document.body.textContent).toContain('OpenAI API key');
  expect(document.body.textContent).toContain('via Team key');
  expect(document.body.textContent).toContain('No fallback');
  expect(document.querySelector('[data-ai-provider-icon="openai"]')).not.toBeNull();
});

it('marks the workspace default and drops its set-default action', async () => {
  await render({ isDefault: true });
  expect(document.body.textContent).toContain('Default');
  expect(document.querySelector('button[aria-label="Set Daily driver as the workspace default"]')).toBeNull();
});

it('flags a profile whose connection is gone', async () => {
  await render({ connections: [] });
  expect(document.body.textContent).toContain('Connection was removed');
});

it('deletes only after the confirmation is accepted', async () => {
  confirmResult.value = false;
  await render();
  await act(async () => document.querySelector<HTMLButtonElement>('button[aria-label="Delete Daily driver"]')!.click());
  expect(aiProfileService.remove).not.toHaveBeenCalled();

  confirmResult.value = true;
  await act(async () => document.querySelector<HTMLButtonElement>('button[aria-label="Delete Daily driver"]')!.click());
  expect(aiProfileService.remove).toHaveBeenCalledWith('ws', profile);
});
