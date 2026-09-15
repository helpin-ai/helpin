import { afterEach, describe, expect, it, vi } from 'vitest';
import { HelpinClient, type HelpinWidgetController } from '../../../src/core/client';
import { helpinClient } from '../../../src/esm-entry';
import { widgetURL, widgetSocketURL } from '../../../src/core/urls';

afterEach(() => { vi.useRealTimers(); vi.clearAllMocks(); document.head.innerHTML = ''; delete (window as any).helpin; delete (window as any).helpinQ; });

describe('support-only distribution', () => {
  it('keeps the widget on first identification and resets/reboots on a known-user switch', async () => {
    const controller = { boot: vi.fn(), shutdown: vi.fn() } as unknown as HelpinWidgetController;
    const client = new HelpinClient({ widgetKey: 'identity-transition', host: 'http://widget.example:8080', supportOnly: true }, controller);
    client.boot();
    await client.id({ id: 'first', email: 'first@example.test' });
    expect(controller.shutdown).not.toHaveBeenCalled();
    expect(controller.boot).toHaveBeenCalledTimes(1);
    await client.id({ id: 'second', email: 'second@example.test' });
    expect(controller.shutdown).toHaveBeenCalledOnce();
    expect(controller.boot).toHaveBeenCalledTimes(2);
    expect(controller.boot).toHaveBeenLastCalledWith(expect.objectContaining({ supportOnly: true, user: expect.objectContaining({ email: 'second@example.test' }) }));
  });
  it('keeps identity working without collector transport, event persistence, capture or retries', async () => {
    vi.useFakeTimers();
    const client = new HelpinClient({ widgetKey: 'community-key', host: 'http://widget.example:8080', supportOnly: true, autoPageview: true, gaHook: true, segmentHook: true });
    client.track('custom_event', { secret: 'must-not-send' }, true);
    client.pageview();
    client.rawTrack({ event: 'raw' });
    window.dispatchEvent(new Event('pagehide'));
    window.dispatchEvent(new Event('online'));
    await vi.advanceTimersByTimeAsync(10000);
    expect(fetch).not.toHaveBeenCalled();
    expect(XMLHttpRequest).not.toHaveBeenCalled();
    expect(navigator.sendBeacon).not.toHaveBeenCalled();
    expect(vi.getTimerCount()).toBe(0);
    const keys = vi.mocked(localStorage.getItem).mock.calls.map(([key]) => key);
    expect(keys.some(key => key.includes('offline_queue'))).toBe(false);
    await client.id({ email: 'visitor@example.com', id: 'visitor-id' });
    expect(fetch).toHaveBeenCalledOnce();
    expect(vi.mocked(fetch).mock.calls[0][0]).toBe('http://widget.example:8080/widget/identify');
    expect(() => client.init({ widgetKey: 'community-key', host: 'http://widget.example:8080', supportOnly: false })).toThrow('Create a new client');
  });

  it('loads the operator-owned hosted runtime and carries support mode through its loader', () => {
    const client = helpinClient({ widgetKey: 'community-key', host: 'http://widget.example:8080', supportOnly: true });
    expect(client).not.toBeNull();
    const script = document.querySelector<HTMLScriptElement>('script[data-helpin-runtime="hosted"]');
    expect(script?.src).toBe('http://widget.example:8080/sdk/lib.js');
    expect(script?.getAttribute('data-support-only')).toBe('true');
    expect((window as any).helpinQ[0][1].supportOnly).toBe(true);
  });

  it('preserves HTTP and WS for local installs and HTTPS/WSS for public hosts', () => {
    expect(widgetURL('http://localhost:8080', '/widget/config')).toBe('http://localhost:8080/widget/config');
    expect(widgetSocketURL('http://localhost:8080', 'a b')).toBe('ws://localhost:8080/widget/ws?key=a%20b');
    expect(widgetSocketURL('widget.example', 'key')).toBe('wss://widget.example/widget/ws?key=key');
    expect(() => widgetURL('https://user:secret@widget.example', '/widget/config')).toThrow();
    expect(() => widgetURL('https://widget.example/wrong-path', '/widget/config')).toThrow();
  });
});
