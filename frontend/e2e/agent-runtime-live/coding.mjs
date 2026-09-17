const apiBase = 'https://helpin-dev.tryunhide.com/api';
const email = process.env.HELPIN_E2E_EMAIL;
const password = process.env.HELPIN_E2E_PASSWORD;
let chatID = process.env.HELPIN_E2E_CHAT_ID;
const resumeExistingChat = Boolean(chatID);
const verifyExistingChat = process.env.HELPIN_E2E_VERIFY_EXISTING === 'true';
if (!email || !password) throw new Error('credentials required');
if (!verifyExistingChat && process.env.HELPIN_E2E_ALLOW_GITHUB_WRITES !== 'true') {
  throw new Error('HELPIN_E2E_ALLOW_GITHUB_WRITES=true is required because this test pushes a branch and opens a pull request');
}
if (verifyExistingChat && !chatID) throw new Error('HELPIN_E2E_CHAT_ID is required with HELPIN_E2E_VERIFY_EXISTING=true');
const stamp = new Date().toISOString().replace(/[-:.TZ]/g, '').slice(0, 14);
const branch = `e2e/ask-agent-${stamp}`;
const filePath = `docs/e2e-agent-runtime-${stamp}.md`;
const validationCommand = 'cargo test --lib enrichment::ua_resolver';
const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
const log = (name, value) => console.log(`E2E ${name}:`, typeof value === 'string' ? value : JSON.stringify(value));

const authResponse = await fetch(`${apiBase}/auth/signin`, { method: 'POST', headers: { 'content-type': 'application/json' }, body: JSON.stringify({ email, password, remember_me: false }) });
if (!authResponse.ok) throw new Error(`signin ${authResponse.status}`);
const auth = await authResponse.json();
async function request(method, path, body) {
  const response = await fetch(`${apiBase}${path}`, { method, headers: { authorization: `Bearer ${auth.access_token}`, ...(body === undefined ? {} : { 'content-type': 'application/json' }) }, body: body === undefined ? undefined : JSON.stringify(body) });
  const raw = await response.text(); let value; try { value = raw ? JSON.parse(raw) : null; } catch { value = { raw }; }
  if (!response.ok) throw new Error(`${method} ${path}: ${response.status} ${JSON.stringify(value)}`);
  return value;
}
const get = (path) => request('GET', path);
const post = (path, body) => request('POST', path, body);
const patch = (path, body) => request('PATCH', path, body);
const workspace = (await get('/workspaces')).find((item) => item.slug === 'usermaven');
if (!workspace) throw new Error('workspace missing');
const q = `workspace_id=${workspace.id}`;
const repository = (await get(`/git/repositories?${q}`)).find((repo) => repo.full_name === 'usermaven/events-pipeline');
if (!repository) throw new Error('repository missing');
const pageContext = { entity_type: 'repository', entity_id: repository.id, display_title: repository.full_name, metadata: { full_name: repository.full_name, default_branch: repository.default_branch } };
if (!chatID) {
  const chat = await post(`/dock/chats?${q}`, { title: `E2E direct coding ${stamp}` });
  if (chat.execution_enabled) throw new Error('execution unexpectedly enabled by default');
  const enabled = await patch(`/dock/chats/${chat.id}?${q}`, { execution_enabled: true });
  if (!enabled.execution_enabled) throw new Error('could not enable execution');
  chatID = chat.id;
  log('new coding chat', { chat_id: chatID, default_off: true, enabled: true });
}

async function send(content) { return post(`/dock/chats/${chatID}/messages?${q}`, { client_message_id: crypto.randomUUID(), content, page_context: pageContext }); }
async function current() {
  const [detail, messages, interactions] = await Promise.all([
    get(`/dock/chats/${chatID}?${q}`),
    get(`/dock/chats/${chatID}/messages?${q}&limit=100`),
    get(`/dock/chats/${chatID}/run/interactions?${q}`),
  ]);
  return { detail, messages: messages.messages ?? [], interactions: interactions.interactions ?? [] };
}
function messageText(message) { return typeof message.content === 'string' ? message.content : JSON.stringify(message.content ?? ''); }
async function resolvePending(state, expectedHuman) {
  for (const interaction of state.interactions.filter((item) => item.status === 'pending')) {
    const review = interaction.request_payload?.approval_review;
    log('approval', { tool: interaction.request_payload?.tool_name, summary: interaction.summary, decision: review?.decision, scores: review?.scores });
    if (!expectedHuman) throw new Error(`unexpected approval prompt for ${interaction.request_payload?.tool_name}`);
    await post(`/dock/chats/${chatID}/interactions/${interaction.id}/resolve?${q}`, { response_payload: { decision: 'approve' } });
  }
}
async function waitTurn(label, { expectedHuman = false, timeout = 420_000 } = {}) {
  const deadline = Date.now() + timeout;
  let last = ''; let approvalCount = 0;
  while (Date.now() < deadline) {
    const state = await current();
    const run = state.detail.run; const pending = state.interactions.filter((item) => item.status === 'pending');
    const signature = `${run?.id}:${run?.status}:${run?.pause_reason ?? ''}:pending=${pending.length}:messages=${state.messages.length}`;
    if (signature !== last) { log(`${label} state`, signature); last = signature; }
    if (pending.length) { approvalCount += pending.length; await resolvePending(state, expectedHuman); await sleep(1_000); continue; }
    if (run?.status === 'paused' && run.pause_reason === 'awaiting_user_message') return { state, approvalCount };
    if (run && ['completed', 'failed', 'cancelled'].includes(run.status)) return { state, approvalCount };
    await sleep(2_000);
  }
  throw new Error(`${label} timed out`);
}

if (resumeExistingChat) {
  const initial = await current();
  if (initial.interactions.some((item) => item.status === 'pending')) {
    await resolvePending(initial, true);
    await waitTurn('resume prior exact command', { expectedHuman: true });
  }
}

if (verifyExistingChat) {
  await send(`Run exactly \`${validationCommand}\` with run_command in the attached repository's \`rust-capture\` directory. Make no file changes. Report E2E_CODING_TEST_OK only after the command passes, and include the exact command. Do not push or open another pull request.`);
  const verification = await waitTurn('existing coding branch tests', { expectedHuman: true, timeout: 600_000 });
  if (verification.state.detail.run?.status === 'failed' || verification.state.detail.run?.status === 'cancelled') throw new Error(`coding test run ended ${verification.state.detail.run.status}`);
  const verificationText = verification.state.messages.filter((message) => message.role === 'assistant').map(messageText).join('\n');
  if (!verificationText.includes('E2E_CODING_TEST_OK')) throw new Error(`coding test marker missing: ${verificationText.slice(-1500)}`);
  console.log(JSON.stringify({ chat_id: chatID, run_id: verification.state.detail.run?.id, repository: repository.full_name, repository_validation_passed: true, marker: 'E2E_CODING_TEST_OK', approval_count: verification.approvalCount }, null, 2));
  process.exit(0);
}

await send('Run exactly `ls -la` in the attached repository, then report E2E_LOCAL_COMMAND_OK. Do not perform any other operation.');
const local = await waitTurn('exact local command', { expectedHuman: true });
if (local.state.detail.run?.status === 'failed' || local.state.detail.run?.status === 'cancelled') throw new Error(`local command ended ${local.state.detail.run.status}`);
const localText = local.state.messages.filter((message) => message.role === 'assistant').map(messageText).join('\n');
if (!localText.includes('E2E_LOCAL_COMMAND_OK')) throw new Error(`local marker missing: ${localText.slice(-1200)}`);
log('exact local command', { auto_approved: local.approvalCount === 0, run_id: local.state.detail.run?.id });

await send(`Authorized end-to-end coding validation. Work only in the attached ${repository.full_name} repository. Create branch ${branch} from ${repository.default_branch}. Add exactly one new file ${filePath} containing a heading and the marker E2E_DIRECT_CODING_OK. Run git status --short to verify only that file changed. Run exactly \`${validationCommand}\` with run_command in the \`rust-capture\` directory and require it to pass. Commit the file with message "test: validate Ask Agent direct coding ${stamp}", push the branch, and open a pull request to ${repository.default_branch} titled "E2E: validate Ask Agent direct coding ${stamp}". The pull request body must say this is an authorized disposable E2E validation. Do not modify any other file. Push and PR creation are explicitly authorized; use the direct tools and do not delegate to Forge.`);
const coding = await waitTurn('direct coding', { expectedHuman: true });
const codingText = coding.state.messages.filter((message) => message.role === 'assistant').map(messageText).join('\n');
log('direct coding final', { status: coding.state.detail.run?.status, approval_count: coding.approvalCount, tail: codingText.slice(-2500) });
if (coding.state.detail.run?.status === 'failed' || coding.state.detail.run?.status === 'cancelled') throw new Error(`coding run ended ${coding.state.detail.run.status}`);
if (!codingText.includes('E2E_DIRECT_CODING_OK') || !/https:\/\/github\.com\/[^\s]+\/pull\/\d+/i.test(codingText)) throw new Error('coding result did not report the marker and pull-request URL');

console.log(JSON.stringify({ chat_id: chatID, run_id: coding.state.detail.run?.id, repository: repository.full_name, branch, file_path: filePath, approval_count: coding.approvalCount }, null, 2));
