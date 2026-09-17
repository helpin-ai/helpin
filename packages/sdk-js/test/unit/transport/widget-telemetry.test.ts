import { afterEach, describe, expect, it, vi } from 'vitest';
import { createWidgetTelemetry } from '../../../src/transport/widget-telemetry';

afterEach(() => { vi.unstubAllGlobals(); vi.useRealTimers(); });
describe('privacy-safe widget telemetry', () => {
  it('sends only allowlisted fields without session credentials', () => {
    const fetch = vi.fn().mockResolvedValue({ ok: true }); vi.stubGlobal('fetch', fetch);
    const report = createWidgetTelemetry(() => 'api.test', () => 'widget-key');
    report({ stage: 'upload', outcome: 'error', duration_ms: 12, file_name: 'secret', session_token: 'secret' } as never);
    const [url, options] = fetch.mock.calls[0];
    expect(url).toBe('https://api.test/widget/telemetry');
    expect(JSON.parse(options.body)).toEqual({ widget_key: 'widget-key', sdk_release: 'local', stage: 'upload', outcome: 'error', duration_ms: 12 });
    expect(options.credentials).toBe('omit');
    expect(options.headers).toEqual({ 'Content-Type': 'application/json' });
  });
  it('bounds reporting during reconnect loops and resets the budget', () => {
    vi.useFakeTimers(); vi.setSystemTime(new Date('2026-09-17T00:00:00Z'));
    const fetch = vi.fn().mockResolvedValue({ ok: true }); vi.stubGlobal('fetch', fetch);
    const report = createWidgetTelemetry(() => 'api.test', () => 'key');
    for (let i = 0; i < 100; i++) report({ stage: 'connection', outcome: 'error', duration_ms: 0 });
    expect(fetch).toHaveBeenCalledTimes(30);
    vi.advanceTimersByTime(60_000);
    report({ stage: 'connection', outcome: 'success', duration_ms: 0 });
    expect(fetch).toHaveBeenCalledTimes(31);
  });
  it('does not throw for unavailable telemetry or unconfigured widgets', async () => {
    const fetch = vi.fn().mockRejectedValue(new Error('offline')); vi.stubGlobal('fetch', fetch);
    const event = { stage: 'upload', outcome: 'error', duration_ms: 0 } as const;
    expect(() => createWidgetTelemetry(() => '', () => null)(event)).not.toThrow();
    expect(fetch).not.toHaveBeenCalled();
    expect(() => createWidgetTelemetry(() => 'api.test', () => 'key')(event)).not.toThrow();
    await Promise.resolve();
  });
});
