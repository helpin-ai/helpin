// A private temporary proof file is created by ai-mail-smoke, never archived.
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { randomUUID } from 'node:crypto';
const proof = JSON.parse(await readFile(process.env.COMMUNITY_TEST_PROOF_FILE, 'utf8'));
const base = process.env.COMMUNITY_URL || 'http://localhost:8085';
async function request(path, body) {
  const response = await fetch(`${base}${path}?workspace_id=${proof.workspace_id}`, {
    method: body ? 'POST' : 'GET',
    headers: { Authorization: `Bearer ${proof.access_token}`, 'Content-Type': 'application/json' },
    body: body && JSON.stringify(body),
  });
  assert.ok(response.ok, `restored profile request failed: ${response.status}`);
  return response.json();
}
const chat = await request('/api/dock/chats', { title: 'Restored credential verification' });
await request(`/api/dock/chats/${chat.id}/messages`, {
  client_message_id: randomUUID(), content: 'Verify the restored connection.', ai_profile_id: proof.profile_id,
});
let answered = false;
for (let attempt = 0; attempt < 60; attempt++) {
  const messages = await request(`/api/dock/chats/${chat.id}/messages`);
  if (JSON.stringify(messages).includes('Community fixture answer.')) { answered = true; break; }
  await new Promise(resolve => setTimeout(resolve, 1000));
}
assert.ok(answered, 'the existing encrypted connection did not work after restore');
console.log('PASS: restored existing profile decrypted its saved connection and ran an agent');

assert.ok(proof.attachment_url, 'restore proof must include a pre-backup attachment');
const restoredAttachment = await fetch(proof.attachment_url);
assert.equal(restoredAttachment.status, 200, 'pre-backup private attachment was not restored');
assert.equal(await restoredAttachment.text(), proof.attachment_text);
const unsignedAttachment = new URL(proof.attachment_url); unsignedAttachment.search = '';
assert.equal((await fetch(unsignedAttachment)).status, 403, 'restored attachment became public');
console.log('PASS: pre-backup private attachment bytes and access policy survived Garage restore');
