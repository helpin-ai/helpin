// @vitest-environment jsdom
import { act } from 'react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { api } from '@/lib/api';
import { button, click, flush, renderWithQuery, type, type Rendered } from '@/components/setup/__tests__/setupTestUtils';
import { ConnectAIStep } from '../ConnectAIStep';

vi.mock('@/lib/api', () => ({ api: { get: vi.fn(), post: vi.fn() } }));

let rendered: Rendered | undefined;
afterEach(async () => {
  await rendered?.unmount();
  rendered = undefined;
  vi.mocked(api.get).mockReset();
  vi.mocked(api.post).mockReset();
});

const LIST = '/ai-connections/?workspace_id=ws-1';
const CREATE = '/ai-connections/?workspace_id=ws-1';

function connection(id: string, provider: string, status: string) {
  return { id, scope: 'workspace', name: provider, provider, status, user_id: null };
}

function listConnections(connections: unknown[]) {
  vi.mocked(api.get).mockImplementation(async (path: string) => path === LIST
    ? { data: { enabled: true, connections, models: [] }, error: null, status: 200 } as never
    : { data: null, error: 'unexpected', status: 404 } as never);
}

function respond() {
  vi.mocked(api.post).mockImplementation(async (path: string) => {
    if (path === CREATE) return { data: { connection: connection('new-1', 'anthropic', 'connected') }, error: null, status: 201 } as never;
    const reconnect = path.match(/^\/ai-connections\/([^/]+)\/reconnect/);
    if (reconnect) return { data: { connection: connection(reconnect[1], 'openrouter', 'connected') }, error: null, status: 200 } as never;
    if (path.includes('/test')) return { data: { ok: true, model: 'model-x', latency_ms: 640 }, error: null, status: 200 } as never;
    return { data: null, error: 'unexpected', status: 404 } as never;
  });
}

async function renderStep(canManage = true) {
  const onContinue = vi.fn();
  rendered = await renderWithQuery(<ConnectAIStep workspaceId="ws-1" canManage={canManage} onContinue={onContinue} />);
  return { ...rendered, onContinue };
}

function row(container: HTMLElement, provider: string) {
  return container.querySelector(`[data-provider="${provider}"]`) as HTMLElement;
}

async function connect(container: HTMLElement, provider: string, key: string) {
  const r = row(container, provider);
  await click(button(r, 'Add key'));
  await type(r.querySelector('input[type="password"]') as HTMLInputElement, key);
  await act(async () => { r.querySelector('form')!.requestSubmit(); });
  await flush();
}

describe('ConnectAIStep', () => {
  it('offers OpenRouter, OpenAI and Anthropic on one page and says keys can change later', async () => {
    listConnections([]);
    const { container } = await renderStep();

    for (const provider of ['openrouter', 'openai', 'anthropic']) {
      expect(row(container, provider)).not.toBeNull();
      expect(button(row(container, provider), 'Add key')).toBeDefined();
    }
    expect(container.textContent).toContain('You can add, replace or remove them later in Settings → AI & knowledge → AI setup.');
    expect(button(container, 'Skip for now')).toBeDefined();
  });

  it('keeps each provider collapsed until Add key opens its focused key field', async () => {
    listConnections([]);
    const { container } = await renderStep();

    expect(container.querySelector('input[type="password"]')).toBeNull();
    const openai = row(container, 'openai');
    expect(openai.textContent).toContain('GPT models, direct from OpenAI.');

    await click(button(openai, 'Add key'));
    const field = openai.querySelector('input[type="password"]') as HTMLInputElement;
    expect(field).not.toBeNull();
    expect(document.activeElement).toBe(field);
    expect(button(openai, 'Connect and test')).toBeDefined();
    expect(button(openai, 'Add key')).toBeUndefined();
    expect(row(container, 'anthropic').querySelector('input')).toBeNull();

    await click(button(openai, 'Cancel'));
    expect(openai.querySelector('input')).toBeNull();
    expect(button(openai, 'Add key')).toBeDefined();
  });

  it('fills the keyless placeholder connection instead of creating a duplicate, then tests it', async () => {
    listConnections([connection('std-or', 'openrouter', 'disconnected')]);
    respond();
    const { container } = await renderStep();

    await connect(container, 'openrouter', 'sk-or-1');

    expect(api.post).toHaveBeenCalledWith('/ai-connections/std-or/reconnect?workspace_id=ws-1', { api_key: 'sk-or-1' });
    expect(api.post).not.toHaveBeenCalledWith(CREATE, expect.anything());
    expect(row(container, 'openrouter').textContent).toContain('Connected. model-x answered in 640 ms.');
  });

  it('creates a connection for a provider without one', async () => {
    listConnections([]);
    respond();
    const { container } = await renderStep();

    await connect(container, 'anthropic', 'sk-ant-1');

    expect(api.post).toHaveBeenCalledWith(CREATE, { name: 'Anthropic', provider: 'anthropic', api_key: 'sk-ant-1', scope: 'workspace' });
  });

  it('shows connected providers with Replace key and lets the person continue', async () => {
    listConnections([connection('c-1', 'openai', 'connected')]);
    const { container, onContinue } = await renderStep();

    expect(row(container, 'openai').textContent).toContain('Connected');
    expect(row(container, 'openai').querySelector('input')).toBeNull();
    await click(button(row(container, 'openai'), 'Replace key'));
    expect(row(container, 'openai').querySelector('input[type="password"]')).not.toBeNull();
    await click(button(container, 'Continue'));
    expect(onContinue).toHaveBeenCalled();
  });

  it('shows members a hint instead of key fields', async () => {
    listConnections([]);
    const { container } = await renderStep(false);

    expect(container.querySelector('input')).toBeNull();
  });
});
