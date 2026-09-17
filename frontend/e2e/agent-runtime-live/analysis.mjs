const apiBase = 'https://helpin-dev.tryunhide.com/api';
const email = process.env.HELPIN_E2E_EMAIL;
const password = process.env.HELPIN_E2E_PASSWORD;
if (!email || !password) throw new Error('credentials required');
const stamp = new Date().toISOString().replace(/[-:.TZ]/g, '').slice(0, 14);
const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
const log = (name, value) => console.log(`E2E ${name}:`, typeof value === 'string' ? value : JSON.stringify(value));

const authResponse = await fetch(`${apiBase}/auth/signin`, { method: 'POST', headers: { 'content-type': 'application/json' }, body: JSON.stringify({ email, password, remember_me: false }) });
if (!authResponse.ok) throw new Error(`signin ${authResponse.status}`);
const auth = await authResponse.json();
async function request(method, path, body, authenticated = true) {
  const response = await fetch(`${apiBase}${path}`, { method, headers: { ...(authenticated ? { authorization: `Bearer ${auth.access_token}` } : {}), ...(body === undefined ? {} : { 'content-type': 'application/json' }) }, body: body === undefined ? undefined : JSON.stringify(body), redirect: 'manual' });
  const text = await response.text();
  let value; try { value = text ? JSON.parse(text) : null; } catch { value = { raw: text }; }
  if (!response.ok) throw new Error(`${method} ${path}: ${response.status} ${JSON.stringify(value)}`);
  return { value, response };
}
const get = async (path) => (await request('GET', path)).value;
const post = async (path, body) => (await request('POST', path, body)).value;
const patch = async (path, body) => (await request('PATCH', path, body)).value;

const workspace = (await get('/workspaces')).find((item) => item.slug === 'usermaven');
if (!workspace) throw new Error('workspace missing');
const q = `workspace_id=${workspace.id}`;
const repository = (await get(`/git/repositories?${q}`)).find((repo) => repo.full_name === 'usermaven/events-pipeline');
if (!repository) throw new Error('events-pipeline missing');

const chat = await post(`/dock/chats?${q}`, { title: `E2E execution ${stamp}` });
if (chat.execution_enabled) throw new Error('execution must default off');
const enabled = await patch(`/dock/chats/${chat.id}?${q}`, { execution_enabled: true });
if (!enabled.execution_enabled) throw new Error('execution setting did not persist');
log('execution chat', { chat_id: chat.id, default_off: true, enabled: enabled.execution_enabled });

async function send(content, pageContext) {
  return post(`/dock/chats/${chat.id}/messages?${q}`, { client_message_id: crypto.randomUUID(), content, ...(pageContext ? { page_context: pageContext } : {}) });
}
async function state() {
  const detail = await get(`/dock/chats/${chat.id}?${q}`);
  const messages = await get(`/dock/chats/${chat.id}/messages?${q}&limit=100`);
  const interactions = await get(`/dock/chats/${chat.id}/run/interactions?${q}`).catch(() => ({ interactions: [] }));
  return { detail, messages: messages.messages ?? [], interactions: interactions.interactions ?? [] };
}
function text(message) { return typeof message.content === 'string' ? message.content : JSON.stringify(message.content ?? ''); }
async function resolvePending(current) {
  for (const interaction of current.interactions.filter((item) => item.status === 'pending')) {
    log('human approval', { id: interaction.id, kind: interaction.interaction_kind, summary: interaction.summary, request_payload: interaction.request_payload });
    await post(`/dock/chats/${chat.id}/interactions/${interaction.id}/resolve?${q}`, { response_payload: { decision: 'approve' } });
  }
}
async function waitTurn(label, { approve = false, timeout = 300_000 } = {}) {
  const deadline = Date.now() + timeout;
  let last = '';
  while (Date.now() < deadline) {
    const current = await state();
    const run = current.detail.run;
    const pending = current.interactions.filter((item) => item.status === 'pending');
    const signature = `${run?.id}:${run?.status}:${run?.pause_reason ?? ''}:pending=${pending.length}:messages=${current.messages.length}`;
    if (signature !== last) { log(`${label} state`, signature); last = signature; }
    if (approve && pending.length) { await resolvePending(current); await sleep(1_000); continue; }
    if (run?.status === 'paused' && run.pause_reason === 'awaiting_user_message') return current;
    if (run && ['completed', 'failed', 'cancelled'].includes(run.status)) return current;
    if (run?.status === 'paused' && pending.length) return current;
    await sleep(2_000);
  }
  throw new Error(`${label} timed out`);
}
async function runEvents() {
  const data = await get(`/dock/chats/${chat.id}/run/events?${q}&after=0&include_snapshot=false`);
  return data.events ?? [];
}
function summarizeEvents(events) {
  return events.map((event) => ({ seq: event.sequence_no, type: event.event_type ?? event.type, tool: event.payload?.tool_name ?? event.data?.tool_name, status: event.payload?.status ?? event.data?.status })).filter((event) => event.tool || event.type === 'approval_review' || event.type?.includes('artifact'));
}

const base = `e2e-${stamp}`;
await send(`This is an authorized local Python E2E test. Use run_python, without a repository, to create ${base}.json containing {"marker":"E2E_PYTHON_OK","phase":1} and ${base}.csv containing two rows. Explicitly publish both files with publish_outputs. Then use run_python a second time in this same turn to read ${base}.json and report E2E_PYTHON_OK. Do not use the network or delegate.`);
let pythonState = await waitTurn('python auto-review');
let pythonPending = pythonState.interactions.filter((item) => item.status === 'pending');
const pythonAutoApproved = pythonPending.length === 0;
if (pythonPending.length) {
  log('python unexpected prompt', pythonPending.map((item) => ({ kind: item.interaction_kind, summary: item.summary, request_payload: item.request_payload })));
  await resolvePending(pythonState);
  pythonState = await waitTurn('python after approval', { approve: true });
}
if (pythonState.detail.run?.status === 'failed' || pythonState.detail.run?.status === 'cancelled') throw new Error(`python run ended ${pythonState.detail.run.status}`);
const pythonText = pythonState.messages.filter((message) => message.role === 'assistant').map(text).join('\n');
if (!pythonText.includes('E2E_PYTHON_OK')) throw new Error(`python marker missing: ${pythonText.slice(-1500)}`);
const firstRunID = pythonState.detail.run?.id;
const firstEvents = await runEvents();
log('python result', { run_id: firstRunID, auto_approved: pythonAutoApproved, event_summary: summarizeEvents(firstEvents) });

await send(`Follow-up retention check: use run_python to read ${base}.json from the existing execution workspace. Do not recreate it. Report E2E_REUSE_OK and its marker value. Do not use the network.`);
let reuseState = await waitTurn('python reuse');
if (reuseState.interactions.some((item) => item.status === 'pending')) { await resolvePending(reuseState); reuseState = await waitTurn('python reuse approved', { approve: true }); }
if (reuseState.detail.run?.status === 'failed' || reuseState.detail.run?.status === 'cancelled') throw new Error(`python reuse ended ${reuseState.detail.run.status}`);
const reuseText = reuseState.messages.filter((message) => message.role === 'assistant').map(text).join('\n');
if (!reuseText.includes('E2E_REUSE_OK') || !reuseText.includes('E2E_PYTHON_OK')) throw new Error(`reuse marker missing: ${reuseText.slice(-1500)}`);
if (reuseState.detail.run?.id !== firstRunID) throw new Error(`follow-up changed run: ${firstRunID} -> ${reuseState.detail.run?.id}`);
log('python reuse', { same_run: true, run_id: firstRunID });

const artifacts = await get(`/pm/agent-runs/${firstRunID}/artifacts?${q}`);
log('artifacts', (artifacts ?? []).map((artifact) => ({ id: artifact.id, type: artifact.artifact_type, content_type: artifact.content_type, file_name: artifact.file_name, visibility: artifact.visibility, download_url: artifact.download_url })));

await send(`Attach the repository in the supplied page context. Inspect its root with repository tools and report E2E_REPO_ATTACHED plus the default branch. Do not modify anything yet.`, { entity_type: 'repository', entity_id: repository.id, display_title: repository.full_name, metadata: { full_name: repository.full_name, default_branch: repository.default_branch } });
let repoState = await waitTurn('repository attach');
if (repoState.interactions.some((item) => item.status === 'pending')) { await resolvePending(repoState); repoState = await waitTurn('repository attach approved', { approve: true }); }
if (repoState.detail.run?.status === 'failed' || repoState.detail.run?.status === 'cancelled') throw new Error(`repository attachment ended ${repoState.detail.run.status}`);
const repoText = repoState.messages.filter((message) => message.role === 'assistant').map(text).join('\n');
if (!repoText.includes('E2E_REPO_ATTACHED')) throw new Error(`repo marker missing: ${repoText.slice(-1500)}`);
log('repository attachment', { run_id: repoState.detail.run?.id, prior_run_id: firstRunID, response_marker: true });

console.log(JSON.stringify({ stamp, chat_id: chat.id, run_id: repoState.detail.run?.id, scratch_run_id: firstRunID, python_auto_approved: pythonAutoApproved, repository_id: repository.id, repository: repository.full_name }, null, 2));
