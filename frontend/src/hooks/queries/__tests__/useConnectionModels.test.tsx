// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { useConnectionModels } from '../useAIConnections';
import { aiConnectionService } from '@/lib/services/aiConnectionService';
import { queryKeys } from '@/lib/queryKeys';

vi.mock('@/lib/services/aiConnectionService', () => ({
  aiConnectionService: { models: vi.fn(), refreshModels: vi.fn() },
}));
Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true });
let root: Root;
let client: QueryClient;
const cached = { models: [{ id: 'old', name: 'Saved list' }], source: 'provider' as const, stale: false };
const fresh = { ...cached, models: [{ id: 'new', name: 'New model' }] };

function Chooser({ refreshOnOpen }: { refreshOnOpen: boolean }) {
  const query = useConnectionModels('ws', 'connection', true, { refreshOnOpen });
  return <div>{query.data?.models.map(model => model.name).join(', ')}{query.isFetching && ' Refreshing'}{query.isError && ' Refresh failed'}</div>;
}
async function render(open = true, refreshOnOpen = true) {
  await act(async () => root.render(<QueryClientProvider client={client}>
    {open && <Chooser refreshOnOpen={refreshOnOpen} />}
  </QueryClientProvider>));
  await act(async () => { await new Promise(resolve => setTimeout(resolve, 0)); });
}
beforeEach(() => {
  const container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
  client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  client.setQueryData([...queryKeys.ai.root('ws'), 'models', 'connection'], cached);
  vi.mocked(aiConnectionService.models).mockResolvedValue({ data: fresh, error: null });
  vi.mocked(aiConnectionService.refreshModels).mockResolvedValue({ data: fresh, error: null });
});
afterEach(() => {
  act(() => root.unmount());
  client.clear();
  document.body.innerHTML = '';
  vi.resetAllMocks();
});

it('refreshes on each opening even with a fresh client cache, but not after model edits', async () => {
  await render();
  expect(aiConnectionService.refreshModels).toHaveBeenCalledTimes(1);
  expect(document.body.textContent).toContain('New model');
  await act(async () => { await client.invalidateQueries({ queryKey: queryKeys.ai.root('ws') }); });
  expect(aiConnectionService.refreshModels).toHaveBeenCalledTimes(1);
  expect(aiConnectionService.models).toHaveBeenCalledTimes(1);
  await render(false);
  await render();
  expect(aiConnectionService.refreshModels).toHaveBeenCalledTimes(2);
});

it('keeps cached choices visible while refreshing and if the request fails', async () => {
  let finish!: (value: { data: null; error: string }) => void;
  vi.mocked(aiConnectionService.refreshModels).mockReturnValue(new Promise(resolve => { finish = resolve; }));
  await render();
  expect(document.body.textContent).toContain('Saved list Refreshing');
  await act(async () => finish({ data: null, error: 'Temporarily unavailable' }));
  await act(async () => { await new Promise(resolve => setTimeout(resolve, 0)); });
  expect(document.body.textContent).toContain('Saved list Refresh failed');
});

it('does not force provider requests for read-only or curated connections', async () => {
  await render(true, false);
  expect(document.body.textContent).toContain('Saved list');
  expect(aiConnectionService.refreshModels).not.toHaveBeenCalled();
});
