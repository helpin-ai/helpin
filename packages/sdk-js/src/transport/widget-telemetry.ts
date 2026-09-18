import { widgetURL } from '../core/urls';
declare const __HELPIN_SDK_RELEASE__: string;

export type WidgetTelemetryEvent = {
  stage: 'initialization' | 'storage' | 'confirmation' | 'upload' | 'connection' | 'message';
  outcome: 'success' | 'error' | 'timeout' | 'cancelled' | 'not_connected' | 'closed';
  duration_ms: number;
  close_code?: number;
};

// Per-widget budget prevents reconnect loops from flooding the telemetry endpoint.
// No file names, page URLs, content, session tokens or exception text are transmitted.
export function createWidgetTelemetry(host: () => string, key: () => string | null) {
  let windowStart = 0;
  let sent = 0;
  return (event: WidgetTelemetryEvent): void => {
    if (!host() || !key()) return;
    const now = Date.now();
    if (now - windowStart >= 60_000) { windowStart = now; sent = 0; }
    if (sent >= 30) return;
    sent++;
    try {
      void fetch(widgetURL(host(), '/widget/telemetry'), {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'omit',
        keepalive: true,
        body: JSON.stringify({ widget_key: key(), sdk_release: typeof __HELPIN_SDK_RELEASE__ === 'undefined' ? 'local' : __HELPIN_SDK_RELEASE__, stage: event.stage, outcome: event.outcome, ...(event.close_code !== undefined ? { close_code: event.close_code } : {}), duration_ms: Math.min(600_000, Math.max(0, Math.round(event.duration_ms))) }),
      }).catch(() => { /* Best effort: telemetry must never affect customer actions. */ });
    } catch { /* Some browser policies throw synchronously. */ }
  };
}
