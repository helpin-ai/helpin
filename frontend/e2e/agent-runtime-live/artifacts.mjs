const apiBase = 'https://helpin-dev.tryunhide.com/api';
const email = process.env.HELPIN_E2E_EMAIL;
const password = process.env.HELPIN_E2E_PASSWORD;
const chatID = process.env.HELPIN_E2E_CHAT_ID;
const runID = process.env.HELPIN_E2E_RUN_ID;
if (!email || !password || !chatID || !runID) throw new Error('credentials, chat id, and run id required');

const signin = await fetch(`${apiBase}/auth/signin`, { method: 'POST', headers: { 'content-type': 'application/json' }, body: JSON.stringify({ email, password, remember_me: false }) });
if (!signin.ok) throw new Error(`signin ${signin.status}`);
const auth = await signin.json();
const headers = { authorization: `Bearer ${auth.access_token}` };
async function request(method, path, body) {
  const response = await fetch(`${apiBase}${path}`, { method, headers: { ...headers, ...(body === undefined ? {} : { 'content-type': 'application/json' }) }, body: body === undefined ? undefined : JSON.stringify(body) });
  const text = await response.text();
  let value; try { value = text ? JSON.parse(text) : null; } catch { value = { raw: text }; }
  if (!response.ok) throw new Error(`${method} ${path}: ${response.status} ${JSON.stringify(value)}`);
  return value;
}
const workspaces = await request('GET', '/workspaces');
const workspace = workspaces.find((item) => item.slug === 'usermaven');
if (!workspace) throw new Error('workspace missing');
const q = `workspace_id=${workspace.id}`;

const artifacts = await request('GET', `/pm/agent-runs/${runID}/artifacts?${q}`);
const outputs = artifacts.filter((item) => item.artifact_type === 'analysis_output');
if (outputs.length < 2) throw new Error(`expected at least two analysis outputs, got ${outputs.length}`);
const chat = await request('GET', `/dock/chats/${chatID}?${q}`);
const visibleIDs = new Set((chat.artifacts ?? []).map((item) => item.id));
for (const output of outputs) {
  if (!visibleIDs.has(output.id)) throw new Error(`published artifact ${output.id} is missing from the Dock chat detail`);
}

async function verifyOutput(artifact) {
  const endpoint = `/agent-artifacts/${artifact.id}/content-url?${q}`;
  const unauthenticated = await fetch(`${apiBase}${endpoint}`);
  if (unauthenticated.status !== 401) throw new Error(`content URL without auth returned ${unauthenticated.status}`);
  const content = await request('GET', endpoint);
  if (!content.url) throw new Error(`artifact ${artifact.id} returned no private URL`);
  const download = await fetch(content.url);
  if (!download.ok) throw new Error(`artifact download ${download.status}`);
  const bytes = new Uint8Array(await download.arrayBuffer());
  if (bytes.length === 0) throw new Error(`artifact ${artifact.id} is empty`);
  return { id: artifact.id, format: artifact.format, file_name: artifact.metadata?.file_name, bytes: bytes.length };
}

const before = await Promise.all(outputs.slice(0, 2).map(verifyOutput));
const disabled = await request('PATCH', `/dock/chats/${chatID}?${q}`, { execution_enabled: false });
if (disabled.execution_enabled) throw new Error('execution did not disable');

let oldRun;
for (let attempt = 0; attempt < 30; attempt += 1) {
  oldRun = await request('GET', `/pm/agent-runs/${runID}?${q}`);
  if (['cancelled', 'completed', 'failed'].includes(oldRun.status)) break;
  await new Promise((resolve) => setTimeout(resolve, 1000));
}
if (!['cancelled', 'completed', 'failed'].includes(oldRun?.status)) throw new Error(`old run remained ${oldRun?.status}`);
const after = await Promise.all(outputs.slice(0, 2).map(verifyOutput));
console.log(JSON.stringify({ chat_id: chatID, run_id: runID, terminal_status: oldRun.status, dock_visible_output_count: outputs.length, authenticated_outputs_before_disable: before, authenticated_outputs_after_terminal_cleanup: after, unauthenticated_content_url_status: 401 }, null, 2));
