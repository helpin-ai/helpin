const apiBase = 'https://helpin-dev.tryunhide.com/api';
const email = process.env.HELPIN_E2E_EMAIL;
const password = process.env.HELPIN_E2E_PASSWORD;
if (!email || !password) throw new Error('credentials required');
const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
const authResponse = await fetch(`${apiBase}/auth/signin`, { method: 'POST', headers: { 'content-type': 'application/json' }, body: JSON.stringify({ email, password, remember_me: false }) });
if (!authResponse.ok) throw new Error(`signin ${authResponse.status}`);
const auth = await authResponse.json();
async function request(method, path, body) {
  const response = await fetch(`${apiBase}${path}`, { method, headers: { authorization: `Bearer ${auth.access_token}`, ...(body === undefined ? {} : { 'content-type': 'application/json' }) }, body: body === undefined ? undefined : JSON.stringify(body) });
  const text = await response.text();
  let value; try { value = text ? JSON.parse(text) : null; } catch { value = { raw: text }; }
  if (!response.ok) throw new Error(`${method} ${path}: ${response.status} ${JSON.stringify(value)}`);
  return value;
}
const get = (path) => request('GET', path);
const post = (path, body) => request('POST', path, body);
const patch = (path, body) => request('PATCH', path, body);
const workspace = (await get('/workspaces')).find((item) => item.slug === 'usermaven');
if (!workspace) throw new Error('workspace missing');
const q = `workspace_id=${workspace.id}`;
const repository = (await get(`/git/repositories?${q}`)).find((item) => item.full_name === 'usermaven/events-pipeline');
const agents = await get(`/pm/agents?${q}`);
const mira = agents.find((item) => item.name === 'Mira');
if (!repository || !mira) throw new Error('repository or saved agent missing');

async function dockState(chatID) {
  const [detail, messages, interactions] = await Promise.all([
    get(`/dock/chats/${chatID}?${q}`),
    get(`/dock/chats/${chatID}/messages?${q}&limit=100`),
    get(`/dock/chats/${chatID}/run/interactions?${q}`).catch(() => ({ interactions: [] })),
  ]);
  return { detail, messages: messages.messages ?? [], interactions: interactions.interactions ?? [] };
}
async function waitDock(chatID, timeout = 300_000) {
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) {
    const state = await dockState(chatID);
    const run = state.detail.run;
    const pending = state.interactions.filter((item) => item.status === 'pending');
    if (pending.length) return state;
    if (run?.status === 'paused' && run.pause_reason === 'awaiting_user_message') return state;
    if (run && ['completed', 'failed', 'cancelled'].includes(run.status)) return state;
    await sleep(1500);
  }
  throw new Error('dock timeout');
}

// A minimal, explicitly requested local command should be eligible for TypeSafe auto-approval.
const typeSafeChat = await post(`/dock/chats?${q}`, { title: 'E2E TypeSafe local autoapproval' });
await patch(`/dock/chats/${typeSafeChat.id}?${q}`, { execution_enabled: true });
await post(`/dock/chats/${typeSafeChat.id}/messages?${q}`, {
  client_message_id: crypto.randomUUID(),
  content: 'Use run_command exactly once with program ls and args ["-la"] in the attached repository. This is a local directory listing only. Then report E2E_TYPESAFE_AUTO_OK. Do nothing else.',
  page_context: { entity_type: 'repository', entity_id: repository.id, display_title: repository.full_name, metadata: { full_name: repository.full_name, default_branch: repository.default_branch } },
});
const typeSafeState = await waitDock(typeSafeChat.id);
if (typeSafeState.detail.run?.status === 'failed' || typeSafeState.detail.run?.status === 'cancelled') throw new Error(`TypeSafe probe ended ${typeSafeState.detail.run.status}`);
const typeSafePending = typeSafeState.interactions.filter((item) => item.status === 'pending');
const typeSafeText = typeSafeState.messages.filter((item) => item.role === 'assistant').map((item) => item.content).join('\n');
if (typeSafePending.length) {
  const review = typeSafePending[0].request_payload?.approval_review;
  throw new Error(`local TypeSafe call prompted: ${JSON.stringify({ summary: typeSafePending[0].summary, scores: review?.scores })}`);
}
if (!typeSafeText.includes('E2E_TYPESAFE_AUTO_OK')) throw new Error(`TypeSafe marker missing: ${typeSafeText.slice(-1000)}`);
await patch(`/dock/chats/${typeSafeChat.id}?${q}`, { execution_enabled: false });

// Saved non-Ask agent should continue to run through the ordinary shared path.
const context = 'Use the repository listing tool and report E2E_SAVED_AGENT_OK plus the exact number of enabled repositories. Do not mutate anything.';
const savedRun = await post(`/pm/agent-runs?${q}`, { agent_id: mira.id, target_type: 'workspace', target_id: workspace.id, additional_context: context, delivery_mode: 'preview' });
let savedState;
for (let attempt = 0; attempt < 200; attempt += 1) {
  savedState = await get(`/pm/agent-runs/${savedRun.id}?${q}`);
  if (['completed', 'failed', 'cancelled', 'paused'].includes(savedState.status)) break;
  await sleep(1500);
}
const savedMessages = await get(`/pm/agent-runs/${savedRun.id}/messages?${q}`);
const savedText = savedMessages.filter((item) => item.role === 'assistant').map((item) => item.content).join('\n');
if (savedState?.status === 'failed' || savedState?.status === 'cancelled') throw new Error(`saved agent ended ${savedState.status}`);
if (!savedText.includes('E2E_SAVED_AGENT_OK')) throw new Error(`saved agent marker missing: ${savedText.slice(-1200)}`);

console.log(JSON.stringify({
  typesafe: { chat_id: typeSafeChat.id, run_id: typeSafeState.detail.run?.id, auto_approved: true, marker: true },
  saved_agent: { agent_id: mira.id, name: mira.name, run_id: savedRun.id, status: savedState.status, marker: true },
}, null, 2));
