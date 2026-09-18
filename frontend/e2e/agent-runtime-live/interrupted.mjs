const apiBase = 'https://helpin-dev.tryunhide.com/api';
const email = process.env.HELPIN_E2E_EMAIL;
const password = process.env.HELPIN_E2E_PASSWORD;
if (!email || !password) throw new Error('credentials required');
const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
const signin = await fetch(`${apiBase}/auth/signin`, { method: 'POST', headers: { 'content-type': 'application/json' }, body: JSON.stringify({ email, password, remember_me: false }) });
if (!signin.ok) throw new Error(`signin ${signin.status}`);
const auth = await signin.json();
async function request(method, path, body) {
  const response = await fetch(`${apiBase}${path}`, { method, headers: { authorization: `Bearer ${auth.access_token}`, ...(body === undefined ? {} : { 'content-type': 'application/json' }) }, body: body === undefined ? undefined : JSON.stringify(body) });
  const raw = await response.text();
  let value; try { value = raw ? JSON.parse(raw) : null; } catch { value = { raw }; }
  if (!response.ok) throw new Error(`${method} ${path}: ${response.status} ${JSON.stringify(value)}`);
  return value;
}
const get = (path) => request('GET', path);
const post = (path, body) => request('POST', path, body);
const patch = (path, body) => request('PATCH', path, body);
const workspace = (await get('/workspaces')).find((item) => item.slug === 'usermaven');
if (!workspace) throw new Error('workspace missing');
const q = `workspace_id=${workspace.id}`;
const chat = await post(`/dock/chats?${q}`, { title: 'E2E interrupted command marker', execution_enabled: true });
if (!chat.execution_enabled) throw new Error('initial execution choice did not persist');
async function state() {
  const [detail, messages, interactions] = await Promise.all([
    get(`/dock/chats/${chat.id}?${q}`),
    get(`/dock/chats/${chat.id}/messages?${q}&limit=100`),
    get(`/dock/chats/${chat.id}/run/interactions?${q}`).catch(() => ({ interactions: [] })),
  ]);
  return { detail, messages: messages.messages ?? [], interactions: interactions.interactions ?? [] };
}
await post(`/dock/chats/${chat.id}/messages?${q}`, {
  client_message_id: crypto.randomUUID(),
  content: 'Use run_command exactly once with program python3 and args ["-c", "import time; time.sleep(30); print(\'finished\')"]. Do not use run_python. After it returns, report E2E_LONG_COMMAND_DONE.',
});
let pending;
let oldRunID;
for (let attempt = 0; attempt < 120; attempt += 1) {
  const current = await state();
  oldRunID = current.detail.run?.id;
  pending = current.interactions.find((item) => item.status === 'pending');
  if (pending) break;
  if (current.detail.run?.status === 'failed') throw new Error('command run failed before approval');
  await sleep(1000);
}
if (!pending || pending.request_payload?.tool_name !== 'run_command') throw new Error(`run_command approval missing: ${JSON.stringify(pending)}`);
if (pending.summary !== 'Approve run_command for this agent run.' || /TypeSafe review|external_send=/.test(pending.summary)) {
  throw new Error(`approval prompt exposed reviewer details: ${pending.summary}`);
}
await post(`/dock/chats/${chat.id}/interactions/${pending.id}/resolve?${q}`, { response_payload: { decision: 'approve' } });
for (let attempt = 0; attempt < 30; attempt += 1) {
  const current = await state();
  if (current.detail.run?.status === 'running') break;
  await sleep(200);
}
await sleep(700);
await patch(`/dock/chats/${chat.id}?${q}`, { execution_enabled: false });
let oldRun;
for (let attempt = 0; attempt < 40; attempt += 1) {
  oldRun = await get(`/pm/agent-runs/${oldRunID}?${q}`);
  if (oldRun.status === 'cancelled') break;
  await sleep(500);
}
if (oldRun?.status !== 'cancelled') throw new Error(`interrupted run ended ${oldRun?.status}`);
if (!JSON.stringify(oldRun.output_summary ?? {}).includes('started_outcome_unknown')) throw new Error(`unknown-outcome marker missing: ${JSON.stringify(oldRun.output_summary)}`);

await post(`/dock/chats/${chat.id}/messages?${q}`, {
  client_message_id: crypto.randomUUID(),
  content: 'Do not rerun or reconstruct the previous command. Report E2E_INTERRUPTED_OK and accurately state what is known about its outcome.',
});
let successor;
for (let attempt = 0; attempt < 120; attempt += 1) {
  successor = await state();
  const run = successor.detail.run;
  if (run?.status === 'paused' && run.pause_reason === 'awaiting_user_message') break;
  if (run && ['failed', 'cancelled'].includes(run.status)) throw new Error(`successor ended ${run.status}`);
  await sleep(1000);
}
const assistant = successor.messages.filter((item) => item.role === 'assistant').map((item) => item.content).join('\n');
if (successor.detail.run?.id === oldRunID) throw new Error('disable did not create a successor');
if (successor.detail.chat.execution_enabled) throw new Error('successor re-enabled execution');
if (!assistant.includes('E2E_INTERRUPTED_OK') || !/outcome.{0,20}unknown|unknown.{0,20}outcome/i.test(assistant)) throw new Error(`successor did not report uncertainty: ${assistant.slice(-1500)}`);
console.log(JSON.stringify({ chat_id: chat.id, interrupted_run_id: oldRunID, successor_run_id: successor.detail.run?.id, marker: 'started_outcome_unknown', successor_reported_uncertainty: true }, null, 2));
