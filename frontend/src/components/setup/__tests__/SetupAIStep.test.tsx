// @vitest-environment jsdom
import { act } from 'react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { api } from '@/lib/api';
import { queryKeys } from '@/lib/queryKeys';
import { SetupAIStep } from '../SetupAIStep';
import { button, capability, click, flush, renderWithQuery, type, type Rendered } from './setupTestUtils';

vi.mock('@/lib/api', () => ({ api: { get: vi.fn(), post: vi.fn() } }));
vi.mock('@tanstack/react-router', () => ({
  Link: ({ to, children }: { to: string; children: React.ReactNode }) => <a href={to}>{children}</a>,
}));

let rendered: Rendered | undefined;
afterEach(async () => {
  await rendered?.unmount();
  rendered = undefined;
  vi.mocked(api.get).mockReset();
  vi.mocked(api.post).mockReset();
});

const LIST = '/ai-connections/?workspace_id=ws-1';
const CREATE = '/ai-connections/?workspace_id=ws-1';
const TEST = '/ai-connections/conn-9/test?workspace_id=ws-1';

const notConnected = capability({ key: 'ai_chat', detail: 'No workspace AI default is set.', action: { kind: 'open_settings', label: 'Connect an AI provider', path: 'settings/ai' } });

function listConnections(connections: unknown[] = []) {
  vi.mocked(api.get).mockImplementation(async (path: string) => path === LIST
    ? { data: { enabled: true, connections, models: [] }, error: null, status: 200 } as never
    : { data: null, error: 'unexpected', status: 404 } as never);
}

function respond(test: { ok: boolean; model: string; latency_ms: number; error?: string }) {
  vi.mocked(api.post).mockImplementation(async (path: string) => {
    if (path === CREATE) {
      return { data: { connection: { id: 'conn-9', scope: 'workspace', name: 'OpenRouter', provider: 'openrouter', status: 'connected', user_id: null } }, error: null, status: 201 } as never;
    }
    if (path === TEST) return { data: test, error: null, status: 200 } as never;
    return { data: null, error: 'unexpected', status: 404 } as never;
  });
}

async function renderStep(cap = notConnected, canManage = true) {
  rendered = await renderWithQuery(<SetupAIStep capability={cap} workspaceId="ws-1" slug="acme" canManage={canManage} />);
  return rendered;
}

async function connectWithKey(container: HTMLElement, key: string) {
  const input = container.querySelector('input[type="password"]') as HTMLInputElement;
  await type(input, key);
  const form = container.querySelector('form')!;
  await act(async () => { form.requestSubmit(); });
  await flush();
}

function status(container: HTMLElement) {
  return container.querySelector('[role="status"]')?.textContent ?? '';
}

describe('SetupAIStep', () => {
  it('creates a workspace connection, tests it straight away and reports the model and latency', async () => {
    listConnections();
    respond({ ok: true, model: 'openai/gpt-5.6-mini', latency_ms: 420 });
    const { container, client } = await renderStep();
    const invalidate = vi.spyOn(client, 'invalidateQueries');

    expect(container.textContent).toContain('Provider');
    await connectWithKey(container, 'sk-or-1');

    expect(api.post).toHaveBeenNthCalledWith(1, CREATE, { name: 'OpenRouter', provider: 'openrouter', api_key: 'sk-or-1', scope: 'workspace' });
    expect(api.post).toHaveBeenNthCalledWith(2, TEST, {});
    expect(status(container)).toBe('Connected. openai/gpt-5.6-mini answered in 420 ms.');
    expect(invalidate).toHaveBeenCalledWith({ queryKey: queryKeys.workspaces.capabilities('ws-1') });
    expect(invalidate).toHaveBeenCalledWith({ queryKey: queryKeys.ai.root('ws-1') });
  });

  it('shows the provider error when the first test fails', async () => {
    listConnections();
    respond({ ok: false, model: 'claude-sonnet', latency_ms: 0, error: 'The provider rejected the API key.' });
    const { container } = await renderStep();

    await connectWithKey(container, 'bad-key');

    expect(status(container)).toBe('The connection test failed: The provider rejected the API key.');
  });

  it('asks for a key before creating anything', async () => {
    listConnections();
    const { container } = await renderStep();

    await connectWithKey(container, '   ');

    expect(api.post).not.toHaveBeenCalled();
    expect(status(container)).toBe('Enter an API key to connect.');
  });

  it('tests an existing, unverified connection', async () => {
    respond({ ok: true, model: 'gpt-5.6', latency_ms: 812.4 });
    const { container } = await renderStep(capability({
      key: 'ai_chat',
      status: 'unable_to_verify',
      action: { kind: 'test_ai_connection', label: 'Test the connection', path: 'settings/ai', connection_id: 'conn-9' },
    }));

    expect(container.querySelector('form')).toBeNull();
    await click(button(container, 'Test connection'));

    expect(api.post).toHaveBeenCalledWith(TEST, {});
    expect(status(container)).toBe('Connected. gpt-5.6 answered in 812 ms.');
  });

  it('links to AI settings when a connected workspace connection exists but AI still needs setup', async () => {
    listConnections([{ id: 'conn-1', scope: 'workspace', name: 'OpenAI', provider: 'openai', status: 'connected', user_id: null }]);
    const { container } = await renderStep(capability({ key: 'ai_chat', action: { kind: 'open_settings', label: 'Connect an AI provider', path: 'settings/ai' } }));

    expect(container.querySelector('form')).toBeNull();
    expect(container.querySelector('a')?.getAttribute('href')).toBe('/w/acme/settings/ai');
  });

  it('shows the key form when only disconnected standard connections exist', async () => {
    // Community provisions standard connections without keys; they must not hide the form.
    listConnections([
      { id: 'std-1', scope: 'workspace', name: 'OpenRouter', provider: 'openrouter', status: 'disconnected', user_id: null },
      { id: 'std-2', scope: 'workspace', name: 'OpenAI', provider: 'openai', status: 'reauthorization_required', user_id: null },
    ]);
    const { container } = await renderStep(capability({ key: 'ai_chat', action: { kind: 'open_settings', label: 'Connect an AI provider', path: 'settings/ai' } }));

    expect(container.querySelector('form')).not.toBeNull();
    expect(container.querySelector('input[type="password"]')).not.toBeNull();
  });

  it('fills the keyless placeholder for the chosen provider instead of creating a duplicate', async () => {
    listConnections([{ id: 'std-1', scope: 'workspace', name: 'OpenRouter', provider: 'openrouter', status: 'disconnected', user_id: null }]);
    vi.mocked(api.post).mockImplementation(async (path: string) => {
      if (path === '/ai-connections/std-1/reconnect?workspace_id=ws-1') {
        return { data: { connection: { id: 'std-1', scope: 'workspace', name: 'OpenRouter', provider: 'openrouter', status: 'connected', user_id: null } }, error: null, status: 200 } as never;
      }
      if (path === '/ai-connections/std-1/test?workspace_id=ws-1') return { data: { ok: true, model: 'glm', latency_ms: 300 }, error: null, status: 200 } as never;
      return { data: null, error: 'unexpected', status: 404 } as never;
    });
    const { container } = await renderStep(capability({ key: 'ai_chat', action: { kind: 'open_settings', label: 'Connect an AI provider', path: 'settings/ai' } }));

    await connectWithKey(container, 'sk-or-2');

    expect(api.post).toHaveBeenCalledWith('/ai-connections/std-1/reconnect?workspace_id=ws-1', { api_key: 'sk-or-2' });
    expect(api.post).not.toHaveBeenCalledWith(CREATE, expect.anything());
    expect(status(container)).toBe('Connected. glm answered in 300 ms.');
  });

  it('renders nothing actionable once AI is ready or for members', async () => {
    const ready = await renderStep(capability({ key: 'ai_chat', status: 'ready' }));
    expect(ready.container.querySelector('button, form, a')).toBeNull();
    await ready.unmount();

    rendered = undefined;
    const member = await renderStep(notConnected, false);
    expect(member.container.querySelector('form')).toBeNull();
    expect(member.container.textContent).toContain('A workspace admin can finish this step.');
  });
});
