// @vitest-environment jsdom
import { act } from 'react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createRoot } from 'react-dom/client';
import { expect, it, vi } from 'vitest';
import { api } from '@/lib/api';
import { useAskAgentDefaults } from '../useAskAgentDefaults';
vi.mock('@/lib/api', () => ({ api: { get: vi.fn() } }));
Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true });

it('loads the dock default without requesting the Automation library and rejects missing responses', async () => {
  vi.mocked(api.get).mockImplementation(async path => path.startsWith('/dock/ai-defaults?')
    ? { data: { ai_profile_id: 'shared-profile' }, error: null, status: 200 } as never
    : { data: null, error: 'module_access_denied', status: 403 } as never);
  let query: ReturnType<typeof useAskAgentDefaults> | undefined;
  function Harness() { query = useAskAgentDefaults('ws-1'); return null; }
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const container = document.createElement('div');
  const root = createRoot(container);
  try {
    await act(async () => { root.render(<QueryClientProvider client={client}><Harness /></QueryClientProvider>); });
    await act(async () => {
      const result = await query!.refetch();
      expect(result.data).toEqual({ ai_profile_id: 'shared-profile' });
    });
    expect(vi.mocked(api.get).mock.calls.every(([path]) => path === '/dock/ai-defaults?workspace_id=ws-1')).toBe(true);
    vi.mocked(api.get).mockResolvedValue({ data: null, error: null, status: 200 } as never);
    await act(async () => { const result = await query!.refetch(); expect(result.isError).toBe(true); });
  } finally { await act(async () => { root.unmount(); }); client.clear(); }
});
