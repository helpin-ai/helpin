const apiBase = 'https://helpin-dev.tryunhide.com/api';
const email = process.env.HELPIN_E2E_EMAIL;
const password = process.env.HELPIN_E2E_PASSWORD;
if (!email || !password) throw new Error('credentials required');

const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
const signin = await fetch(`${apiBase}/auth/signin`, {
  method: 'POST',
  headers: { 'content-type': 'application/json' },
  body: JSON.stringify({ email, password, remember_me: false }),
});
if (!signin.ok) throw new Error(`signin ${signin.status}`);
const auth = await signin.json();
const headers = { authorization: `Bearer ${auth.access_token}` };

async function json(response) {
  const text = await response.text();
  let body;
  try { body = text ? JSON.parse(text) : null; } catch { body = { raw: text }; }
  return { status: response.status, ok: response.ok, body };
}

const workspaces = await json(await fetch(`${apiBase}/workspaces`, { headers }));
if (!workspaces.ok) throw new Error(`list workspaces ${workspaces.status}`);
const workspace = workspaces.body.find((item) => item.slug === 'usermaven');
if (!workspace) throw new Error('workspace missing');
const q = `workspace_id=${workspace.id}`;
const chatResponse = await json(await fetch(`${apiBase}/dock/chats?${q}`, {
  method: 'POST',
  headers: { ...headers, 'content-type': 'application/json' },
  body: JSON.stringify({ title: 'E2E Dock admission race' }),
}));
if (!chatResponse.ok) throw new Error(`create chat ${chatResponse.status}`);
const chat = chatResponse.body;

const send = (label) => fetch(`${apiBase}/dock/chats/${chat.id}/messages?${q}`, {
  method: 'POST',
  headers: { ...headers, 'content-type': 'application/json' },
  body: JSON.stringify({
    client_message_id: crypto.randomUUID(),
    content: `Reply exactly E2E_ADMISSION_LOCK_OK_${label}. Do not call tools.`,
  }),
}).then(json);

const attempts = await Promise.all([send('A'), send('B')]);
const accepted = attempts.filter((item) => item.ok);
const rejected = attempts.filter((item) => !item.ok);
if (accepted.length !== 1 || rejected.length !== 1) {
  throw new Error(`expected one admitted message and one busy rejection: ${JSON.stringify(attempts)}`);
}

let detail;
for (let attempt = 0; attempt < 120; attempt += 1) {
  const response = await json(await fetch(`${apiBase}/dock/chats/${chat.id}?${q}`, { headers }));
  if (!response.ok) throw new Error(`get chat ${response.status}`);
  detail = response.body;
  if (detail.run?.status === 'paused' && detail.run.pause_reason === 'awaiting_user_message') break;
  if (detail.run && ['completed', 'failed', 'cancelled'].includes(detail.run.status)) break;
  await sleep(1000);
}
if (!detail?.run || detail.run.status !== 'paused' || detail.run.pause_reason !== 'awaiting_user_message') {
  throw new Error(`admitted run did not finish normally: ${JSON.stringify(detail?.run)}`);
}
const messages = await json(await fetch(`${apiBase}/dock/chats/${chat.id}/messages?${q}&limit=100`, { headers }));
if (!messages.ok) throw new Error(`list messages ${messages.status}`);
const userTurns = (messages.body.messages ?? []).filter((item) => item.role === 'user' && item.content.includes('E2E_ADMISSION_LOCK_OK_'));
if (userTurns.length !== 1) throw new Error(`expected one durable user turn, got ${userTurns.length}`);

console.log(JSON.stringify({
  chat_id: chat.id,
  run_id: detail.run.id,
  admitted_status: accepted[0].status,
  rejected_status: rejected[0].status,
  durable_user_turns: userTurns.length,
}, null, 2));
