import { test } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, cp, mkdir, writeFile, readFile, stat, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { execFileSync } from 'node:child_process';
import { waitForAPI } from './readiness.mjs';
const root = new URL('../', import.meta.url);

test('public readiness waits for the expected installation and configuration', async t => {
  const states = [new Response('', { status: 401 }),
    Response.json({ public_widget_url: 'http://test', app_email_configured: false }),
    Response.json({ public_widget_url: 'http://test', app_email_configured: true })];
  t.mock.method(globalThis, 'fetch', async () => states.shift());
  assert.equal((await waitForAPI('http://test', true, 2000)).app_email_configured, true);
});

test('public readiness fails closed on another installation', async t => {
  t.mock.method(globalThis, 'fetch', async () => Response.json({ public_widget_url: 'http://other', app_email_configured: true }));
  await assert.rejects(waitForAPI('http://test', true, 0), /expected configuration/);
});

test('install generates independent stable secrets and never executes dotenv contents', async () => {
  const dir = await mkdtemp(join(tmpdir(), 'community-installer-'));
  try {
    for (const file of ['setup.sh', '.env.example', 'apps.example.json']) await cp(new URL(file, root), join(dir, file));
    await mkdir(join(dir, 'bin'));
    await writeFile(join(dir, 'bin/docker'), '#!/bin/sh\nexit 0\n', { mode: 0o755 });
    const run = command => execFileSync('bash', [join(dir, 'setup.sh'), command], { env: { ...process.env, PATH: `${dir}/bin:${process.env.PATH}` }, stdio: 'pipe' });
    run('install');
    const generated = await readFile(join(dir, '.env'), 'utf8');
    const values = Object.fromEntries(generated.split('\n').filter(line => /^[A-Z_]+=/.test(line)).map(line => [line.slice(0, line.indexOf('=')), line.slice(line.indexOf('=') + 1)]));
    const keys = ['POSTGRES_PASSWORD', 'HELPIN_DB_PASSWORD', 'RUNTIME_DB_PASSWORD', 'TEMPORAL_DB_PASSWORD', 'VISIBILITY_DB_PASSWORD', 'GARAGE_SECRET_KEY', 'GARAGE_RPC_SECRET', 'JWT_SECRET', 'INTERNAL_API_SECRET', 'AI_CONNECTION_ENCRYPTION_KEY', 'AGENT_RUNTIME_MODEL_CREDENTIAL_ENCRYPTION_KEY', 'AGENT_RUNTIME_MCP_CREDENTIAL_ENCRYPTION_KEY', 'CRM_ENCRYPTION_KEY', 'GIT_OAUTH_ENCRYPTION_KEY'];
    assert.equal(new Set(keys.map(key => values[key])).size, keys.length);
    for (const key of keys) assert.ok(values[key].length >= 43, `${key} not initialized`);
    assert.match(values.GARAGE_ACCESS_KEY, /^GK[0-9a-f]{32}$/);
    assert.equal((await stat(join(dir, '.env'))).mode & 0o777, 0o600);
    const malicious = generated + `\nUNUSED=$(touch ${dir}/executed)\n`;
    await writeFile(join(dir, '.env'), malicious);
    run('install'); run('start');
    assert.equal(await readFile(join(dir, '.env'), 'utf8'), malicious);
    await assert.rejects(stat(join(dir, 'executed')), { code: 'ENOENT' });
  } finally { await rm(dir, { recursive: true, force: true }); }
});

test('bundle launch and event contracts agree with the Runtime template', async () => {
  const compose = await readFile(new URL('compose.yaml', root), 'utf8');
  const app = JSON.parse(await readFile(new URL('apps.example.json', root), 'utf8')).apps[0];
  assert.match(compose, /AGENT_RUNTIME_LAUNCH_ENABLED: 'true'/);
  assert.match(compose, new RegExp(`AGENT_RUNTIME_EVENT_PROTOCOL: ${app.event_protocol}`));
  assert.equal(app.require_run_model_credentials, true);
  assert.equal(app.browser.enabled, false);
  assert.equal(app.model_credential_callback.token_env, 'HELPIN_INTERNAL_API_SECRET');
  const fixture = JSON.parse(await readFile(new URL('tests/apps.fixture.json', root), 'utf8')).apps[0];
  fixture.model_endpoints = [];
  assert.deepEqual(fixture, app);
});
