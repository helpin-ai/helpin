// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { AIFollowUpSettings } from './AIFollowUpSettings';

const preview = vi.hoisted(() => vi.fn());
vi.mock('@/lib/services/supportService', () => ({ supportService: { previewConversationFollowUps: preview } }));
let container: HTMLDivElement;
let root: Root;
let client: QueryClient;

beforeEach(() => {
  vi.stubGlobal('IS_REACT_ACT_ENVIRONMENT', true);
  preview.mockReset();
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
  client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
});
afterEach(() => { act(() => root.unmount()); client.clear(); container.remove(); vi.unstubAllGlobals(); });

async function setup(enabled = false) {
  const props = { workspaceId: 'ws', enabled, delayHours: 24, closeHours: 48, maxPerConversation: 2, onEnabledChange: vi.fn(), onDelayChange: vi.fn(), onCloseChange: vi.fn(), onMaxChange: vi.fn() };
  await act(async () => root.render(<QueryClientProvider client={client}><AIFollowUpSettings {...props} /></QueryClientProvider>));
  return props;
}

describe('AI follow-up settings', () => {
  it('keeps follow-ups off until explicitly enabled', async () => {
    const props = await setup();
    const toggle = container.querySelector<HTMLButtonElement>('[role="switch"]')!;
    expect(toggle.getAttribute('aria-checked')).toBe('false');
    expect(container.querySelector('#ai-follow-up-delay')).toBeNull();
    await act(async () => toggle.click());
    expect(props.onEnabledChange).toHaveBeenCalledWith(true);
    expect(preview).not.toHaveBeenCalled();
  });
  it('previews backlog without enabling the policy', async () => {
    preview.mockResolvedValue({ data: { candidates: 12, daily_limit: 25, lookback_days: 30, sample: [{ id: 'conv', display_id: 42, subject: 'Gmail reconnect' }] }, error: null });
    const props = await setup();
    const button = Array.from(container.querySelectorAll('button')).find(item => item.textContent === 'Preview existing conversations')!;
    await act(async () => { button.click(); });
    await vi.waitFor(() => expect(container.textContent).toContain('12 conversations could be assessed'));
    expect(container.textContent).toContain('#42 Gmail reconnect');
    expect(props.onEnabledChange).not.toHaveBeenCalled();
  });
  it('shows accessible timing inputs and the closure notice', async () => {
    await setup(true);
    const close = container.querySelector<HTMLInputElement>('#ai-follow-up-close')!;
    expect(close.value).toBe('48');
    expect(close.min).toBe('1');
    expect(close.max).toBe('720');
    expect(container.querySelector('label[for="ai-follow-up-close"]')?.textContent).toBe('Wait after follow-up (hours)');
    expect(container.textContent).toContain('48 hours without a reply');
  });
});
