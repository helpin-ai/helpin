const { chromium, firefox } = require('playwright');
const { randomUUID } = require('node:crypto');

const site = process.env.SMOKE_SITE || 'https://helpin.ai/';
const metricsURL = process.env.SMOKE_METRICS_URL;
const png = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jRZkAAAAASUVORK5CYII=', 'base64');
async function publish(browser, success, seconds) {
  if (!metricsURL) return;
  const now = Math.floor(Date.now() / 1000);
  const body = `helpin_widget_smoke_success{browser="${browser}"} ${success}\nhelpin_widget_smoke_last_run_seconds{browser="${browser}"} ${now}\nhelpin_widget_smoke_duration_seconds{browser="${browser}"} ${seconds}\n`;
  const response = await fetch(metricsURL, { method: 'POST', body, signal: AbortSignal.timeout(10_000) });
  if (!response.ok) throw new Error('metrics_publish_failed');
}
// Fixed categories only: never emit URLs, response bodies or exception messages.
function failureKind(error) {
  const message = String(error?.message || error || '');
  const code = message.match(/\b(?:ERR_[A-Z_]{1,50}|NS_ERROR_[A-Z_]{1,50}|NS_BINDING_ABORTED|SSL_ERROR_[A-Z_]{1,50}|SEC_ERROR_[A-Z_]{1,50})\b/);
  if (code) return code[0];
  if (error?.name === 'TimeoutError') return 'timeout';
  return 'other';
}
async function check(kind) {
  let browser, page, attachment, session, api, downloadURL, failed = false;
  let metadataFailed = false;
  let stage = 'launch';
  const start = Date.now();
  const name = `helpin-monitor-${randomUUID()}.png`;
  const pending = [];
  const network = [];
  const record = event => { if (network.length < 12) network.push(event); };
  try {
    browser = await ({ chromium, firefox }[kind]).launch({ headless: true });
    page = await browser.newPage({ userAgent: kind === 'firefox'
      ? 'Mozilla/5.0 (X11; Linux x86_64; rv:146.0) Gecko/20100101 Firefox/146.0'
      : 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/145.0.0.0 Safari/537.36' });
    page.setDefaultTimeout(45_000);
    await page.addInitScript(() => Object.defineProperty(navigator, 'webdriver', { get: () => false }));
    page.on('requestfailed', request => {
      const type = request.resourceType();
      if (['document', 'script', 'fetch', 'xhr'].includes(type)) {
        const host = new URL(request.url()).hostname;
        const destination = host === 'cdn.helpin.ai' ? 'widget_cdn' : host === new URL(site).hostname ? 'site' : 'other';
        record({ type, destination, failure: failureKind(request.failure()?.errorText) });
      }
    });
    page.on('websocket', socket => {
      const socketURL = new URL(socket.url());
      if (socketURL.pathname !== '/widget/ws') return;
      socket.on('socketerror', () => record({ type: 'websocket', failure: 'socket_error' }));
      socket.on('framereceived', ({ payload }) => {
        try {
          const frame = JSON.parse(String(payload));
          if (frame.type === 'session:joined' && frame.data?.session_token) {
            session = frame.data.session_token;
            api = socketURL.origin.replace(/^ws/, 'http');
          }
        } catch { /* Ignore non-JSON frames. */ }
      });
    });
    page.on('response', response => {
      const request = response.request();
      const url = new URL(response.url());
      if (response.status() >= 400) record({ type: 'http', status: response.status() });
      if (request.method() !== 'POST' || url.pathname !== '/widget/support/attachments') return;
      pending.push((async () => {
        if (request.postDataJSON()?.file_name !== name || !response.ok()) return;
        const data = await response.json();
        attachment = data.attachment?.id;
        downloadURL = data.public_url;
        session = request.headers()['x-session-token'];
        api = url.origin;
      })().catch(() => { metadataFailed = true; }));
    });
    stage = 'page';
    const navigation = await page.goto(site, { waitUntil: 'domcontentloaded', timeout: 45_000 });
    if (navigation && !navigation.ok()) throw new Error('page_http_error');
    stage = 'launcher';
    await page.locator('.helpin-launcher').click();
    stage = 'session';
    // Widget readiness only means config loaded; wait for the real session acknowledgement.
    const connectionDeadline = Date.now() + 45_000;
    while (!session && Date.now() < connectionDeadline) await new Promise(resolve => setTimeout(resolve, 100));
    if (!session) throw new Error('session_not_connected');
    stage = 'upload';
    await page.locator('.helpin-widget input[type=file]').setInputFiles({ name, mimeType: 'image/png', buffer: png });
    await page.locator('.helpin-widget').getByText('Ready to send', { exact: false }).waitFor();
    await Promise.all(pending);
    if (metadataFailed || !attachment || !session || !api || !downloadURL) throw new Error('missing_confirmation_metadata');
    stage = 'readback';
    const stored = await page.evaluate(async url => {
      const response = await fetch(url, { signal: AbortSignal.timeout(15_000) });
      if (!response.ok) throw new Error('readback_failed');
      return Array.from(new Uint8Array(await response.arrayBuffer()));
    }, downloadURL);
    if (!Buffer.from(stored).equals(png)) throw new Error('readback_mismatch');
  } catch (error) {
    failed = true;
    // Never print browser errors: they can include signed URLs or session credentials.
    console.error(JSON.stringify({ browser: kind, stage, outcome: 'failed', failure: failureKind(error), network }));
  } finally {
    await Promise.allSettled(pending);
    if (session && api && page) {
      try {
        const cleaned = await page.evaluate(async ({ api, attachment, session }) => {
          let deleted = !attachment;
          try {
            if (attachment) {
              const response = await fetch(`${api}/widget/support/attachments/${attachment}`, {
                method: 'DELETE', headers: { 'X-Session-Token': session }, signal: AbortSignal.timeout(15_000),
              });
              deleted = response.ok;
            }
          } finally {
            // Revoke the synthetic session even if attachment cleanup failed.
            const revoke = await fetch(`${api}/widget/session/revoke`, {
              method: 'POST', headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({ session_token: session }), signal: AbortSignal.timeout(15_000),
            });
            if (!revoke.ok) deleted = false;
          }
          return deleted;
        }, { api, attachment: attachment || null, session });
        if (!cleaned) throw new Error('cleanup_failed');
      } catch { failed = true; console.error(JSON.stringify({ browser: kind, stage: 'cleanup', outcome: 'failed' })); }
    }
    if (browser) {
      try { await browser.close(); } catch { failed = true; console.error(JSON.stringify({ browser: kind, stage: 'close', outcome: 'failed' })); }
    }
  }
  const seconds = (Date.now() - start) / 1000;
  await publish(kind, failed ? 0 : 1, seconds);
  console.log(JSON.stringify({ browser: kind, outcome: failed ? 'failed' : 'success', duration_seconds: seconds }));
  return !failed;
}
(async () => {
  let passed = true;
  for (const kind of (process.env.SMOKE_BROWSERS || 'chromium,firefox').split(',')) {
    if (!['chromium', 'firefox'].includes(kind)) throw new Error('invalid_browser');
    if (!await check(kind)) passed = false;
  }
  process.exitCode = passed ? 0 : 1;
})().catch(() => { console.error('smoke_runner_failed'); process.exitCode = 1; });
