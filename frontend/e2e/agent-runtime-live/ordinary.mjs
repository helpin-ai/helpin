const apiBase = 'https://helpin-dev.tryunhide.com/api';
const email = process.env.HELPIN_E2E_EMAIL;
const password = process.env.HELPIN_E2E_PASSWORD;
if (!email || !password) throw new Error('HELPIN_E2E_EMAIL and HELPIN_E2E_PASSWORD are required');

const stamp = new Date().toISOString().replace(/[-:.TZ]/g, '').slice(0, 14);
const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
const log = (name, value) => console.log(`E2E ${name}:`, typeof value === 'string' ? value : JSON.stringify(value));

const signin = await fetch(`${apiBase}/auth/signin`, {
  method: 'POST', headers: { 'content-type': 'application/json' },
  body: JSON.stringify({ email, password, remember_me: false }),
});
if (!signin.ok) throw new Error(`signin failed: ${signin.status} ${await signin.text()}`);
const session = await signin.json();
if (!session.access_token) throw new Error('signin returned no access token');

async function request(method, path, body) {
  const response = await fetch(`${apiBase}${path}`, {
    method,
    headers: { authorization: `Bearer ${session.access_token}`, ...(body === undefined ? {} : { 'content-type': 'application/json' }) },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  const text = await response.text();
  let value;
  try { value = text ? JSON.parse(text) : null; } catch { value = { raw: text }; }
  if (!response.ok) throw new Error(`${method} ${path}: ${response.status} ${JSON.stringify(value)}`);
  return value;
}
const get = (path) => request('GET', path);
const post = (path, body) => request('POST', path, body);
const patch = (path, body) => request('PATCH', path, body);

const workspaces = await get('/workspaces');
const workspace = workspaces.find((item) => item.slug === 'usermaven');
if (!workspace) throw new Error('Usermaven workspace not found');
const q = `workspace_id=${encodeURIComponent(workspace.id)}`;
const repositories = await get(`/git/repositories?${q}`);
const repository = repositories.find((repo) => repo.full_name === 'usermaven/events-pipeline') ?? repositories[0];
if (!repository) throw new Error('No enabled repository found');
log('workspace', { id: workspace.id, repository: repository.full_name, repository_id: repository.id });

async function createChat(executionEnabled = false) {
  const chat = await post(`/dock/chats?${q}`, { title: `E2E validation ${stamp}` });
  if (Boolean(chat.execution_enabled)) throw new Error('new chat unexpectedly enabled execution');
  if (executionEnabled) {
    const updated = await patch(`/dock/chats/${chat.id}?${q}`, { execution_enabled: true });
    if (!updated.execution_enabled) throw new Error('execution toggle did not persist');
    return updated;
  }
  return chat;
}

async function send(chat, content, pageContext) {
  return post(`/dock/chats/${chat.id}/messages?${q}`, {
    client_message_id: crypto.randomUUID(),
    content,
    ...(pageContext ? { page_context: pageContext } : {}),
  });
}

async function chatState(chat) {
  const [detail, messages, interactions] = await Promise.all([
    get(`/dock/chats/${chat.id}?${q}`),
    get(`/dock/chats/${chat.id}/messages?${q}&limit=100`),
    get(`/dock/chats/${chat.id}/run/interactions?${q}`).catch(() => ({ interactions: [] })),
  ]);
  return { detail, messages: messages.messages ?? [], interactions: interactions.interactions ?? [] };
}

function messageText(message) {
  if (typeof message.content === 'string') return message.content;
  return JSON.stringify(message.content ?? '');
}

async function waitForChat(chat, { timeout = 240_000, approve = false, label } = {}) {
  const deadline = Date.now() + timeout;
  const resolved = new Set();
  let last = '';
  while (Date.now() < deadline) {
    const state = await chatState(chat);
    const run = state.detail.run;
    const pending = state.interactions.filter((item) => item.status === 'pending');
    const signature = `${run?.status}:${run?.pause_reason ?? ''}:${pending.length}:${state.messages.length}`;
    if (signature !== last) { log(`${label ?? chat.id} state`, signature); last = signature; }
    if (approve) {
      for (const interaction of pending) {
        if (resolved.has(interaction.id)) continue;
        log(`${label ?? chat.id} approving`, { id: interaction.id, kind: interaction.interaction_kind, summary: interaction.summary });
        await post(`/dock/chats/${chat.id}/interactions/${interaction.id}/resolve?${q}`, { response_payload: { decision: 'approve' } });
        resolved.add(interaction.id);
      }
    }
    if (run && ['completed', 'failed', 'cancelled'].includes(run.status)) return state;
    if (run?.status === 'paused' && run.pause_reason === 'awaiting_user_message') return state;
    if (run?.status === 'paused' && !approve && pending.length > 0) return state;
    await sleep(2_000);
  }
  throw new Error(`${label ?? chat.id} timed out`);
}

async function events(chat) {
  const response = await get(`/dock/chats/${chat.id}/run/events?${q}&after=0&include_snapshot=false`);
  return response.events ?? [];
}

// Phase 1: ordinary Ask Agent stays non-execution and exercises shared host tools.
const ordinary = await createChat(false);
await send(ordinary, 'End-to-end validation: use the repository listing tool and answer with E2E_GENERAL_OK plus the exact number of enabled repositories. Do not enable execution or delegate.');
const ordinaryDone = await waitForChat(ordinary, { label: 'ordinary' });
if (ordinaryDone.detail.run?.status === 'failed' || ordinaryDone.detail.run?.status === 'cancelled') {
  throw new Error(`ordinary Ask Agent ended ${ordinaryDone.detail.run.status}`);
}
const ordinaryText = ordinaryDone.messages.filter((message) => message.role === 'assistant').map(messageText).join('\n');
if (!ordinaryText.includes('E2E_GENERAL_OK')) throw new Error(`ordinary Ask Agent marker missing: ${ordinaryText.slice(-1000)}`);
const ordinaryEvents = await events(ordinary);
log('ordinary result', {
  chat_id: ordinary.id,
  run_id: ordinaryDone.detail.run?.id,
  status: ordinaryDone.detail.run?.status,
  execution_enabled: ordinaryDone.detail.chat.execution_enabled,
  event_types: [...new Set(ordinaryEvents.map((event) => event.event_type ?? event.type))],
  marker: ordinaryText.includes('E2E_GENERAL_OK'),
});

console.log(JSON.stringify({ stamp, workspace_id: workspace.id, repository, ordinary_chat_id: ordinary.id }, null, 2));
