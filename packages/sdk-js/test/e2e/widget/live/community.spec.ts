import { test, expect } from '@playwright/test';
import { createServer, type Server } from 'node:http';

test('packaged support-only loader: outside-origin visitor, reply, reload and rejected origin', async ({ page, browser }) => {
  const fixture = await import('../../../../../../community/tests/http-smoke.mjs');
  const { installation, request, base, origin, auth, ws, session, ownerPassword } = fixture;
  const servers: Server[] = [];
  try {
    for (const port of [8098, 8099]) {
      const server = createServer((_req, res) => {
        res.setHeader('Content-Type', 'text/html');
        res.end(`<!doctype html><html><body><h1>Customer website</h1><script>
window.helpin = function () { (window.helpinQ = window.helpinQ || []).push(arguments); };
</script><script defer id="helpin-widget" data-widget-key="${installation.widget_key}" data-host="${base}" data-support-only="true" src="${base}/sdk/lib.js"></script></body></html>`);
      });
      await new Promise<void>(resolve => server.once('error', error => { throw error; }).listen(port, '127.0.0.1', resolve));
      servers.push(server);
    }
    await page.addInitScript(() => {
      Object.defineProperty(Navigator.prototype, 'webdriver', { get: () => false });
      const agent = navigator.userAgent.replace('HeadlessChrome', 'Chrome');
      Object.defineProperty(Navigator.prototype, 'userAgent', { get: () => agent });
    });
    const unexpected: string[] = [];
    page.on('request', req => {
      const url = new URL(req.url());
      if (!['localhost', '127.0.0.1'].includes(url.hostname) || /\/capture|\/track|\/batch|\/analytics/.test(url.pathname)) unexpected.push(url.origin + url.pathname);
    });
    console.log('Browser fixture ready');
    await page.goto(origin, { waitUntil: 'domcontentloaded' });
    await expect(page.locator('.helpin-launcher')).toBeVisible();
    console.log('Widget loaded');
    const upload = await request('/widget/support/attachments', { method: 'POST', status: 201, visitorOrigin: origin,
      sessionToken: session.session_token, body: { file_name: 'browser.txt', file_size: 16, content_type: 'text/plain' } });
    console.log('Upload admitted');
    const uploaded = await page.evaluate(async url => (await fetch(url, {
      method: 'PUT', headers: { 'Content-Type': 'text/plain' }, body: 'Browser upload!!',
    })).status, upload.upload_url);
    expect(uploaded).toBe(200);
    console.log('Browser upload completed');
    await request(`/widget/support/attachments/${upload.attachment.id}/confirm`, { method: 'PATCH', visitorOrigin: origin, sessionToken: session.session_token, body: {} });
    await page.evaluate(() => window.helpin?.('id', { id: 'community-visitor', email: 'browser-visitor@example.test', first_name: 'Visitor' }));
    await page.locator('.helpin-launcher').click();
    await expect(page.locator('.helpin-chat-window')).toBeVisible();
    const start = page.getByRole('button', { name: /send us a message|start a conversation|new conversation/i });
    if (await start.count()) await start.first().click();
    await expect(page.locator('.helpin-compose-input')).toBeVisible();
    const text = `Browser support ${Date.now()}`;
    await page.locator('.helpin-compose-input').fill(text);
    await page.locator('.helpin-compose-send').click();
    await expect(page.locator('.helpin-message-list')).toContainText(text);
    let conversation: any;
    await expect.poll(async () => {
      const list = await request('/api/support/inbox/conversations');
      conversation = list.data.find((item: any) => JSON.stringify(item).includes(text));
      return Boolean(conversation);
    }).toBe(true);
    await request(`/api/support/inbox/conversations/${conversation.id}/messages`, { method: 'POST', status: 201,
      body: { content: 'Packaged inbox reply', message_type: 'reply' } });
    await expect(page.locator('.helpin-message-list')).toContainText('Packaged inbox reply');
    await page.reload();
    await page.locator('.helpin-launcher').click();
    await expect(page.locator('.helpin-chat-window')).toContainText('Packaged inbox reply');
    expect(unexpected).toEqual([]);
    const forbidden = await page.request.get(`${base}/widget/config?widget_key=${installation.widget_key}`, { headers: { Origin: 'http://localhost:8099' } });
    expect(forbidden.status()).toBe(403);
    const otherSite = await browser.newPage();
    await otherSite.goto('http://localhost:8099', { waitUntil: 'domcontentloaded' });
    for (const query of [`key=${installation.widget_key}`, `session_token=${session.session_token}`]) {
      const rejected = await otherSite.evaluate(url => new Promise<boolean>(resolve => {
        const socket = new WebSocket(url);
        const timeout = setTimeout(() => { socket.close(); resolve(false); }, 3000);
        socket.onopen = () => { clearTimeout(timeout); socket.close(); resolve(false); };
        socket.onerror = () => { clearTimeout(timeout); resolve(true); };
      }), `${base.replace(/^http/, 'ws')}/widget/ws?${query}`);
      expect(rejected).toBe(true);
    }
    await otherSite.close();
    console.log('Both unauthorized WebSockets rejected');
    const staff = await browser.newPage();
    await staff.goto(`${base}/login`);
    await staff.getByLabel('Email', { exact: true }).fill(auth.user.email);
    await staff.getByLabel('Password', { exact: true }).fill(ownerPassword);
    await staff.getByRole('button', { name: 'Sign in', exact: true }).click();
    await expect(staff).not.toHaveURL(/\/login/);
    await staff.goto(`${base}/w/${ws.slug}/support/inbox`);
    await expect(staff.getByText('Packaged inbox reply').first()).toBeVisible();
    await expect(staff.getByText('Verify your email address', { exact: true })).toHaveCount(0);
    await expect(staff).toHaveURL(new RegExp(`/w/${ws.slug}/support`));
    await staff.goto(`${base}/w/${ws.slug}/support/${conversation.id}`);
    await staff.locator('[contenteditable="true"]').first().fill('Reply from the staff browser');
    await staff.getByRole('button', { name: 'Send', exact: true }).click();
    await expect(page.locator('.helpin-message-list')).toContainText('Reply from the staff browser');
    await page.evaluate(() => window.helpin?.('id', { id: 'another-visitor', email: 'another-visitor@example.test' }));
    await expect(page.locator('.helpin-launcher')).toBeVisible();
    await page.locator('.helpin-launcher').click();
    await expect(page.locator('.helpin-chat-window')).toBeVisible();
    await expect(page.locator('.helpin-chat-window')).not.toContainText('Packaged inbox reply');
    await expect(page.locator('.helpin-chat-window')).not.toContainText('Reply from the staff browser');
    await staff.close();
  } finally {
    await Promise.all(servers.map(server => new Promise<void>(resolve => { server.closeAllConnections(); server.close(() => resolve()); })));
  }
});
