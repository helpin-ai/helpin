// @vitest-environment jsdom
import { act, StrictMode } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { ExternalMCPCallbackPage } from '../ExternalMCPCallbackPage';
import { externalMCPService } from '@/lib/services/externalMCPService';

vi.mock('@/lib/services/externalMCPService', () => ({ externalMCPService: { completeOAuth: vi.fn() } }));
(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
let root: Root;
let container: HTMLDivElement;
const replace = vi.fn();
beforeEach(() => {
  vi.clearAllMocks();
  const original = window;
  vi.stubGlobal('window', new Proxy(original, { get(target, key) {
    return key === 'location' ? { origin: 'https://app.helpin.ai', replace } : Reflect.get(target, key, target);
  } }));
  container = document.createElement('div'); document.body.append(container); root = createRoot(container);
});
afterEach(() => { act(() => root.unmount()); container.remove(); vi.unstubAllGlobals(); });

it('exchanges once in Strict Mode and returns to the workspace', async () => {
  vi.mocked(externalMCPService.completeOAuth).mockResolvedValue({ data: { redirect_url: 'https://app.helpin.ai/w/acme/settings/external-tools?external_mcp_oauth=connected' }, error: null } as never);
  await act(async () => root.render(<StrictMode><ExternalMCPCallbackPage query={{ state: 'state', code: 'code', error: '' }} /></StrictMode>));
  expect(externalMCPService.completeOAuth).toHaveBeenCalledTimes(1);
  expect(externalMCPService.completeOAuth).toHaveBeenCalledWith({ state: 'state', code: 'code', error: '' });
  expect(replace).toHaveBeenCalledWith('https://app.helpin.ai/w/acme/settings/external-tools?external_mcp_oauth=connected');
});
it('passes provider cancellation through the authenticated completion', async () => {
  vi.mocked(externalMCPService.completeOAuth).mockResolvedValue({ data: { redirect_url: 'https://app.helpin.ai/workspaces?external_mcp_oauth=failed' }, error: null } as never);
  await act(async () => root.render(<ExternalMCPCallbackPage query={{ state: 'state', code: '', error: 'access_denied' }} />));
  expect(externalMCPService.completeOAuth).toHaveBeenCalledWith({ state: 'state', code: '', error: 'access_denied' });
  expect(replace).toHaveBeenCalled();
});
it('rejects missing state without exchanging a code', async () => {
  await act(async () => root.render(<ExternalMCPCallbackPage query={{ state: '', code: 'code', error: '' }} />));
  expect(externalMCPService.completeOAuth).not.toHaveBeenCalled();
  expect(container.textContent).toContain('Could not finish connecting');
});
it.each(['error', 'external redirect'])('shows recovery for %s', async (kind) => {
  vi.mocked(externalMCPService.completeOAuth).mockResolvedValue(kind === 'error' ? { data: null, error: { message: 'Failed' } } as never : { data: { redirect_url: 'https://untrusted.example' }, error: null } as never);
  await act(async () => root.render(<ExternalMCPCallbackPage query={{ state: 'state', code: 'code', error: '' }} />));
  expect(replace).not.toHaveBeenCalled();
  expect(container.textContent).toContain('Could not finish connecting');
});
