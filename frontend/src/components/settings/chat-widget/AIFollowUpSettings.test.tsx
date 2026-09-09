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

async function setup(enabled = true) {
  const props = { enabled, delayHours: 24, closeHours: 1, secondDelayHours: 24, onEnabledChange: vi.fn(), onDelayChange: vi.fn(), onCloseChange: vi.fn(), onSecondDelayChange: vi.fn() };
  await act(async () => root.render(<QueryClientProvider client={client}><AIFollowUpSettings {...props} /></QueryClientProvider>));
  return props;
}

describe('AI follow-up settings', () => {
  it('allows follow-ups to be disabled', async () => {
    const props = await setup(false);
    const toggle = container.querySelector<HTMLButtonElement>('[role="switch"]')!;
    expect(toggle.getAttribute('aria-checked')).toBe('false');
    expect(container.querySelector('#ai-follow-up-delay')).toBeNull();
    await act(async () => toggle.click());
    expect(props.onEnabledChange).toHaveBeenCalledWith(true);
    expect(preview).not.toHaveBeenCalled();
  });
  it('removes the existing conversation preview and lifetime limit', async () => {
    await setup();
    expect(container.textContent).not.toContain('Preview existing conversations');
    expect(container.querySelector('#ai-follow-up-max')).toBeNull();
  });
  it('shows accessible timing inputs and the closure notice', async () => {
    await setup(true);
    const close = container.querySelector<HTMLInputElement>('#ai-follow-up-close')!;
    expect(close.value).toBe('1');
    expect(close.min).toBe('1');
    expect(close.max).toBe('720');
    expect(container.querySelector('label[for="ai-follow-up-close"]')?.textContent).toBe('Close after final follow-up (hours)');
    expect(container.querySelector<HTMLInputElement>('#ai-follow-up-second-delay')?.value).toBe('24');
    expect(container.textContent).toContain('closing shortly');
    expect(container.textContent).toContain('reply anytime');
    expect(container.textContent).toContain('Existing follow-up sequences keep their saved timings');
  });
});
