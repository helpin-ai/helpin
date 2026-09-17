/**
 * Helpin Widget Loader — tiny inline stub (~2KB minified).
 *
 * Built to dist/lib.js (stable URL, short cache).
 * The full SDK filename (with content hash) is injected at build time
 * via the __SDK_FILENAME__ placeholder — no runtime manifest fetch needed.
 *
 * On page load this does:
 *   1. Skips bots (minimal inline check)
 *   2. Injects <link rel="preconnect"> for the API/WS host
 *   3. Queues any API calls via window[namespace]()
 *   4. Async-loads the full SDK bundle which drains the queue on init
 */

(function (document: Document, window: Window & typeof globalThis) {
  // ─── Bot check (inline, minimal) ──────────────────────────────
  const ua = navigator?.userAgent || '';
  if (/bot|crawl|spider|headlesschrome|phantomjs|selenium|lighthouse/i.test(ua)) return;
  if ((navigator as any)?.webdriver) return;

  // ─── Capture script element ───────────────────────────────────
  const currentScript = document.currentScript as HTMLScriptElement | null;
  if (!currentScript) return;

  const widgetKey = currentScript.getAttribute('data-widget-key') || currentScript.getAttribute('data-key');
  if (!widgetKey) return;

  const host = currentScript.getAttribute('data-host') || currentScript.getAttribute('data-tracking-host') || new URL(currentScript.src).origin;
  const namespace = currentScript.getAttribute('data-namespace') || 'helpin';
  const noAutoInit = currentScript.getAttribute('data-no-auto-init') === 'true';

  // ─── DNS preconnect ───────────────────────────────────────────
  const hostUrl = host.startsWith('http') ? host : `https://${host}`;
  if (!document.querySelector(`link[rel="preconnect"][href="${hostUrl}"]`)) {
    const link = document.createElement('link');
    link.rel = 'preconnect';
    link.href = hostUrl;
    link.crossOrigin = 'anonymous';
    document.head.appendChild(link);
  }

  // ─── Queue stub ───────────────────────────────────────────────
  const queueName = `${namespace}Q`;
  (window as any)[queueName] = (window as any)[queueName] || [];

  if (!(window as any)[namespace]) {
    (window as any)[namespace] = function (...args: any[]) {
      (window as any)[queueName].push(args);
    };
  }

  // ─── Load full SDK bundle ─────────────────────────────────────
  // __SDK_FILENAME__ is replaced at build time with the hashed filename
  const sdkFile = '__SDK_FILENAME__';
  const baseUrl = currentScript.src.substring(0, currentScript.src.lastIndexOf('/') + 1);

  const script = document.createElement('script');
  script.type = 'module';
  script.src = baseUrl + sdkFile;
  script.async = true;

  // Pass through data attributes so the full SDK can read them
  script.setAttribute('data-widget-key', widgetKey);
  script.setAttribute('data-host', host);
  script.setAttribute('data-namespace', namespace);
  if (noAutoInit) script.setAttribute('data-no-auto-init', 'true');

  // Copy any extra data attributes
  const attrs = currentScript.attributes;
  for (let i = 0; i < attrs.length; i++) {
    const attr = attrs[i];
    if (attr.name.startsWith('data-') && !script.hasAttribute(attr.name)) {
      script.setAttribute(attr.name, attr.value);
    }
  }

  document.head.appendChild(script);
})(document, window);
