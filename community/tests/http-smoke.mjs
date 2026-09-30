// Creates test data through public APIs. Run only against a disposable install.
import assert from 'node:assert/strict';
import { randomUUID } from 'node:crypto';
import { waitForAPI } from './readiness.mjs';

const base = process.env.COMMUNITY_URL || 'http://localhost:8085';
const origin = process.env.COMMUNITY_VISITOR_ORIGIN || 'http://localhost:8098';
let token, workspace;
async function request(path, { method = 'GET', body, visitorOrigin, status = 200, sessionToken } = {}) {
  const headers = { 'Content-Type': 'application/json' };
  if (token && path.startsWith('/api/')) headers.Authorization = `Bearer ${token}`;
  if (visitorOrigin) headers.Origin = visitorOrigin;
  if (sessionToken) headers['X-Session-Token'] = sessionToken;
  if (workspace && path.startsWith('/api/')) path += `${path.includes('?') ? '&' : '?'}workspace_id=${workspace}`;
  const response = await fetch(base + path, { method, headers, body: body && JSON.stringify(body) });
  const failure = response.ok ? '' : (await response.clone().json().catch(() => ({}))).error || 'request failed';
  assert.equal(response.status, status, `${method} ${path.split('?')[0]}: ${failure}`);
  if (status === 403 || status === 404) return;
  return response.json();
}
const config = await waitForAPI(base, process.env.COMMUNITY_TEST_AI === 'yes');
assert.equal(config.email_verification_required, false);
assert.equal(config.app_email_configured, process.env.COMMUNITY_TEST_AI === 'yes');
const suffix = randomUUID().slice(0, 8);
const ownerPassword = randomUUID() + 'Aa1!';
const auth = await request('/api/auth/signup', { method: 'POST', status: 201, body: {
  email: `community-${suffix}@example.test`, password: ownerPassword, full_name: 'Community Test Owner',
} });
token = auth.access_token;
assert.ok(token);
assert.notEqual(auth.user.email_verified, true);
// Signup becomes invite-only once the first account exists, but every
// acceptance step that imports this fixture signs up its own owner. On this
// disposable install the first owner, the server admin, reopens signup.
const adminHeaders = { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' };
const signupPolicy = await fetch(`${base}/api/instance/signup-policy`, { headers: adminHeaders });
if (signupPolicy.ok && (await signupPolicy.json()).mode !== 'open') {
  const opened = await fetch(`${base}/api/instance/signup-policy`, {
    method: 'PUT', headers: adminHeaders, body: JSON.stringify({ mode: 'open', allowed_domains: [] }),
  });
  assert.equal(opened.status, 200, 'PUT /api/instance/signup-policy: open signup for later acceptance steps');
}
const org = await request('/api/organizations', { method: 'POST', status: 201, body: { name: 'Community Test', slug: `community-${suffix}` } });
const ws = await request('/api/workspaces', { method: 'POST', status: 201, body: {
  name: 'Support Test', slug: `support-${suffix}`, workspace_key: 'TEST', organization_id: org.id, timezone: 'UTC', setup_goals: [],
} });
workspace = ws.id;
assert.ok(workspace);
let installation = await request('/api/support/inbox/installations');
assert.equal(installation.identity_verification_mode, 'report_only');
assert.deepEqual(installation.allowed_origins || [], []);
const configPath = `/widget/config?widget_key=${encodeURIComponent(installation.widget_key)}`;
await request(configPath, { visitorOrigin: origin, status: 403 });
installation = await request('/api/support/inbox/installations', { method: 'PATCH', body: {
  allowed_origins: [origin, base], ai_enabled: false, require_email_before_chat: false,
} });
await request(configPath, { visitorOrigin: origin });
await request(configPath, { visitorOrigin: 'https://untrusted.example', status: 403 });
await request(configPath, { status: 403 });
const session = await request('/widget/session', { method: 'POST', visitorOrigin: origin, status: 201,
  body: { widget_key: installation.widget_key, customer_name: 'Test Visitor', customer_email: `visitor-${suffix}@example.test` } });
const message = await request('/widget/messages', { method: 'POST', visitorOrigin: origin, status: 201,
  body: { session_token: session.session_token, content: 'Can you help me get started?' } });
assert.ok(message.conversation_id);
const fileText = 'Private Community support attachment';
const attachment = await request('/widget/support/attachments', { method: 'POST', status: 201, visitorOrigin: origin,
  sessionToken: session.session_token, body: { file_name: 'support.txt', file_size: fileText.length, content_type: 'text/plain' } });
assert.equal(new URL(attachment.upload_url).hostname, 'localhost');
const uploadPreflight = await fetch(attachment.upload_url, { method: 'OPTIONS', headers: { Origin: origin, 'Access-Control-Request-Method': 'PUT', 'Access-Control-Request-Headers': 'content-type' } });
assert.ok(uploadPreflight.ok, 'browser upload CORS preflight failed');
assert.ok(['*', origin].includes(uploadPreflight.headers.get('access-control-allow-origin')), 'browser upload origin was not allowed');
assert.equal((await fetch(attachment.upload_url, { method: 'PUT', headers: { 'Content-Type': 'text/plain' }, body: fileText })).status, 200);
await request(`/widget/support/attachments/${attachment.attachment.id}/confirm`, { method: 'PATCH', visitorOrigin: origin, sessionToken: session.session_token, body: {} });
const signedFile = await fetch(attachment.public_url);
assert.equal(signedFile.status, 200);
assert.equal(await signedFile.text(), fileText);
const unsignedFile = new URL(attachment.public_url); unsignedFile.search = '';
assert.equal((await fetch(unsignedFile)).status, 403, 'customer attachment is anonymously readable');
await request(`/api/support/inbox/conversations/${message.conversation_id}/messages`, { method: 'POST', status: 201,
  body: { content: 'Yes, welcome to the self-hosted inbox!', message_type: 'reply' } });
const history = await request(`/widget/messages?session_token=${encodeURIComponent(session.session_token)}`, { visitorOrigin: origin });
assert.ok(JSON.stringify(history).includes('welcome to the self-hosted inbox'));
await request('/api/internal/agent-runtime/target-context', { status: 404 });
const knowledge = await request('/api/ai-connections');
assert.equal(knowledge.knowledge.embedding_dimensions, 1536);
assert.equal(knowledge.knowledge.embeddings_configured, process.env.COMMUNITY_TEST_AI === 'yes');
const helpSlug = `help-${suffix}`;
await request('/api/docs/helpcenter/config', { method: 'PUT', body: {
  subdomain: helpSlug, brand_name: 'Community Help', is_published: true, chat_widget_enabled: false,
} });
const space = await request('/api/docs/spaces', { method: 'POST', status: 201, body: {
  name: 'Getting Started', slug: 'getting-started', type: 'external_capable', visibility: 'workspace',
} });
const collection = await request(`/api/docs/spaces/${space.id}/collections`, { method: 'POST', status: 201, body: { name: 'Basics', slug: 'basics' } });
const article = await request('/api/docs/documents', { method: 'POST', status: 201, body: { space_id: space.id, collection_id: collection.id, title: 'Your first support conversation' } });
await request(`/api/docs/documents/${article.id}/content/markdown`, { method: 'PUT', body: { markdown: '# Your first support conversation\n\nInstall the widget on your website to start a conversation. Add your website origin in settings, copy the support-only installation snippet, and open the inbox to reply to a visitor. Your data stays in your installation.' } });
const png = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+aZ1cAAAAASUVORK5CYII=', 'base64');
const editorUpload = await request('/api/pm/attachments', { method: 'POST', status: 201, body: {
  entity_type: 'editor_upload', entity_id: article.id, file_name: 'diagram.png', file_size: png.length, content_type: 'image/png', private: true,
} });
assert.equal((await fetch(editorUpload.url, { method: 'PUT', headers: { 'Content-Type': 'image/png' }, body: png })).status, 200);
await request(`/api/pm/attachments/${editorUpload.attachment.id}/confirm`, { method: 'PATCH' });
const imageSrc = `/api/pm/attachments/${editorUpload.attachment.id}/content`;
const contentResponse = await fetch(`${base}${imageSrc}?workspace_id=${ws.id}&proxy=1`, { headers: { Authorization: `Bearer ${token}` } });
assert.equal(contentResponse.status, 200, 'Docs image content must work with PM disabled');
assert.deepEqual(Buffer.from(await contentResponse.arrayBuffer()), png);
const unsigned = new URL(editorUpload.url); unsigned.search = '';
assert.ok([400,403].includes((await fetch(unsigned)).status), 'storage object was anonymously readable');
const imageNodes = [{ type: 'resizableImage', attrs: { src: imageSrc, attachmentId: editorUpload.attachment.id } }];
if (process.env.COMMUNITY_TEST_AI === 'yes') {
  const imported = await request('/api/docs/images/import', { method: 'POST', body: { image_url: 'http://test-providers:8080/image.png' } });
  const privateURL = new URL(imported.url);
  assert.equal(privateURL.pathname, '/api/docs/images/content');
  assert.equal((await fetch(privateURL)).status, 401, 'imported Docs image was publicly readable');
  const privateImage = await fetch(privateURL, { headers: { Authorization: `Bearer ${token}` } });
  assert.equal(privateImage.status, 200);
  assert.deepEqual(Buffer.from(await privateImage.arrayBuffer()), png);
  const publicAttempt = `${base}/api/public/assets/${privateURL.searchParams.get('key')}`;
  assert.equal((await fetch(publicAttempt)).status, 404, 'private import passed the public allowlist');
  imageNodes.push({ type: 'resizableImage', attrs: { src: imported.url } });
}
await request(`/api/docs/documents/${article.id}/publish`, { method: 'POST', body: {
  slug: 'first-conversation', published_content: { type: 'doc', content: [{ type: 'paragraph', content: [{ type: 'text', text: 'Install the widget on your website to start a conversation.' }] }, ...imageNodes] },
} });
const published = await request(`/api/hc/${helpSlug}/spaces/getting-started/articles/first-conversation`);
assert.ok(JSON.stringify(published).includes('Install the widget'));
const publicImages = Array.from((published.content_html || '').matchAll(/<img[^>]*\bsrc="([^"]+)"/g), match => match[1].replaceAll('&amp;', '&'));
assert.ok(publicImages.length >= imageNodes.length, 'publication lost images');
for (const src of publicImages) {
  assert.ok(src.startsWith(`${base}/api/public/assets/helpcenter/`), 'published image still uses a private source');
  const image = await fetch(src);
  assert.equal(image.status, 200);
  assert.deepEqual(Buffer.from(await image.arrayBuffer()), png);
}

const hcBase = process.env.COMMUNITY_HC_URL || 'http://localhost:8086';
const rendered = await fetch(`${hcBase}/getting-started/basics/first-conversation?subdomain=${helpSlug}`);
assert.equal(rendered.status, 200, (await rendered.clone().text()).replace(/<[^>]+>/g, ' ').slice(-2500));
assert.ok((await rendered.text()).includes('Install the widget'));
assert.equal((await fetch(`${hcBase}/api/internal/agent-runtime/target-context`)).status, 404);
console.log('PASS: owner/workspace, report-only identity, origin isolation, visitor message and staff reply, internal route isolation');
console.log('PASS: explicit embedding diagnostics, published help article, help-center SSR and proxy isolation');
export { auth, ws, installation, request, base, origin, helpSlug, article, attachment, session, ownerPassword, fileText };
