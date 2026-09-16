import assert from 'node:assert/strict';
import { randomUUID } from 'node:crypto';
import { writeFile } from 'node:fs/promises';
process.env.COMMUNITY_TEST_AI = 'yes';
const { request, auth, ws } = await import('./http-smoke.mjs');
const fixture = process.env.COMMUNITY_PROVIDER_URL || 'http://localhost:8097';
const before = await (await fetch(fixture + '/metrics')).json();
const created = await request('/api/ai-connections', { method: 'POST', status: 201, body: {
  name: 'Local acceptance provider', provider: 'openai_compatible', scope: 'workspace', endpoint_id: 'community-test', api_key: 'community-fixture-key',
} });
const profile = await request('/api/ai-profiles', { method: 'POST', status: 201, body: {
  name: 'Local acceptance profile', scope: 'workspace', primary: {
    connection_id: created.connection.id, model: { provider: 'openai_compatible', model: 'community-test-model', controls: {} },
  },
} });
await request('/api/ai-settings', { method: 'PUT', body: { default_profile_id: profile.id } });
const chat = await request('/api/dock/chats', { method: 'POST', status: 201, body: { title: 'Community acceptance' } });
await request(`/api/dock/chats/${chat.id}/messages`, { method: 'POST', body: {
  client_message_id: randomUUID(), content: 'Please reply with a short welcome.', ai_profile_id: profile.id,
} });
let answer = false;
for (let attempt = 0; attempt < 60; attempt++) {
  const messages = await request(`/api/dock/chats/${chat.id}/messages`);
  if (JSON.stringify(messages).includes('Community fixture answer.')) { answer = true; break; }
  await new Promise(resolve => setTimeout(resolve, 1000));
}
assert.ok(answer, 'durable agent did not return the fixture answer');
let finished = false;
for (let attempt = 0; attempt < 30; attempt++) {
  const detail = await request(`/api/dock/chats/${chat.id}`);
  assert.notEqual(detail.run?.status, 'failed', 'agent response did not successfully finish its turn');
  if (detail.run?.status === 'completed' ||
      (detail.run?.status === 'paused' && detail.run?.pause_reason === 'awaiting_user_message')) {
    finished = true; break;
  }
  await new Promise(resolve => setTimeout(resolve, 1000));
}
assert.ok(finished, 'agent returned text but never completed its turn');
await request('/api/auth/forgot-password', { method: 'POST', body: { email: auth.user.email } });
const invitee = `invite-${randomUUID()}@example.test`;
await request('/api/invitations', { method: 'POST', status: 201, body: { workspace_id: ws.id, email: invitee, role: 'member' } });
let metrics;
for (let attempt = 0; attempt < 30; attempt++) {
  metrics = await (await fetch(fixture + '/metrics')).json();
  if (metrics.mail > before.mail && metrics.embeddings > 0) break;
  await new Promise(resolve => setTimeout(resolve, 1000));
}
assert.ok(metrics.completions > before.completions, 'agent did not call the configured provider');
assert.equal(metrics.credential_rejections, 0, 'a provider call lacked its expected credential');
assert.ok(metrics.embeddings > 0, 'publishing did not call the separately configured embedding provider');
assert.ok(metrics.mail > before.mail, 'password-reset email was not delivered through SMTP');
const mails = await (await fetch(fixture + '/mail')).json();
assert.ok(mails.some(mail => mail.includes(auth.user.email)));
assert.ok(mails.some(mail => mail.includes(invitee)), 'invitation was not delivered through SMTP');
assert.ok(mails.every(mail => !/https?:\/\/(?:[^/]+\.)?helpin\.(?:ai|email)\b/.test(mail)), 'mail contains a hosted Helpin URL');
const resetMail = mails.findLast(mail => mail.includes(auth.user.email) && mail.includes('reset-password'));
const decoded = resetMail?.replace(/=\r\n/g, '').replace(/=3D/g, '=');
const resetToken = decoded?.match(/reset-password\?token=([a-zA-Z0-9_-]+)/)?.[1];
assert.ok(resetToken, 'password reset mail has no usable link');
const password = randomUUID() + 'Aa1!';
await request('/api/auth/reset-password', { method: 'POST', body: { token: resetToken, password } });
const signedIn = await request('/api/auth/signin', { method: 'POST', body: { email: auth.user.email, password } });
if (process.env.COMMUNITY_TEST_PROOF_FILE) {
  await writeFile(process.env.COMMUNITY_TEST_PROOF_FILE, JSON.stringify({
    access_token: signedIn.access_token, workspace_id: ws.id, profile_id: profile.id,
  }), { mode: 0o600 });
}
console.log('PASS: shared profile → durable Runtime worker → authenticated compatible provider; embeddings; SMTP password reset; no hosted mail links');
