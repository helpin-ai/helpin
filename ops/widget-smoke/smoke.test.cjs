const { test } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const source = fs.readFileSync(`${__dirname}/smoke.cjs`, 'utf8');

async function run(options = {}) {
  const events = {}, calls = [], logs = [];
  let uploaded;
  const state = { env: { SMOKE_BROWSERS: 'chromium', SMOKE_METRICS_URL: 'https://metrics.test/import' }, exitCode: 0 };
  const page = {
    setDefaultTimeout() {}, async addInitScript() {}, on(name, fn) { events[name] = fn; },
    async goto() {
      events.websocket({ url: () => 'wss://api.test/widget/ws', on: (_name, fn) => fn({ payload: JSON.stringify({ type: 'session:joined', data: { session_token: 'private-session' } }) }) });
    },
    locator(selector) {
      return {
        async click() {},
        async setInputFiles(file) {
          uploaded = file.buffer;
          events.response({
            request: () => ({ method: () => 'POST', postDataJSON: () => ({ file_name: file.name }), headers: () => ({ 'x-session-token': 'private-session' }) }),
            url: () => 'https://api.test/widget/support/attachments', ok: () => true,
            json: async () => {
              if (options.badMetadata) throw new Error('private-response');
              return { attachment: { id: 'owned-test-image' }, public_url: 'https://storage.test/image?signature=private-signature' };
            },
          });
        },
        getByText() { return { async waitFor() { if (options.uploadFails) throw new Error('private-signature'); } }; },
      };
    },
    async evaluate(fn, args) { return fn(args); },
  };
  const browser = { async newPage() { return page; }, async close() { if (options.closeFails) throw new Error('private-browser-error'); } };
  const fetch = async (url, init = {}) => {
    calls.push({ url, ...init });
    if (url.startsWith('https://storage.test')) return new Response(options.badReadback ? Buffer.from('wrong bytes') : uploaded);
    if (init.method === 'DELETE') {
      if (options.deleteThrows) throw new Error('private-cleanup-error');
      return new Response(null, { status: options.deleteFails ? 503 : 204 });
    }
    if (url.endsWith('/revoke')) return new Response(null, { status: 204 });
    if (url.startsWith('https://metrics.test')) return new Response(null, { status: 204 });
    throw new Error('unexpected request');
  };
  await vm.runInNewContext(source, {
    require: name => name === 'playwright' ? { chromium: { launch: async () => browser }, firefox: {} } : require(name),
    process: state, Buffer, URL, Date, Uint8Array, AbortSignal, setTimeout, fetch,
    console: { log: text => logs.push(text), error: text => logs.push(text) },
  });
  assert.equal(logs.some(line => /private-|signature=/.test(line)), false, 'logs exposed credentials or raw errors');
  return { calls, logs, exitCode: state.exitCode };
}

test('checks stored bytes, deletes only its attachment, revokes session and publishes success', async () => {
  const result = await run();
  assert.equal(result.exitCode, 0);
  assert.ok(result.calls.some(c => c.url.startsWith('https://storage.test')));
  assert.ok(result.calls.some(c => c.method === 'DELETE' && c.url.endsWith('/owned-test-image')));
  assert.ok(result.calls.some(c => c.url.endsWith('/revoke')));
  assert.match(result.calls.at(-1).body, /smoke_success\{browser="chromium"\} 1/);
});
for (const option of ['badReadback', 'uploadFails', 'deleteFails', 'deleteThrows', 'closeFails', 'badMetadata']) {
  test(`${option} reports failure without skipping session revocation or leaking secrets`, async () => {
    const result = await run({ [option]: true });
    assert.equal(result.exitCode, 1);
    assert.ok(result.calls.some(c => c.url.endsWith('/revoke')));
    assert.match(result.calls.at(-1).body, /smoke_success\{browser="chromium"\} 0/);
  });
}
