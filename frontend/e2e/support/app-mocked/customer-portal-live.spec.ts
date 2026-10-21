import { expect, test, type Page, type WebSocketRoute } from '@playwright/test'

const slug = 'example'
const api = `/api/public/portal/${slug}`
const config = { enabled: true, intake_enabled: true, file_uploads_enabled: false, branding: { name: 'Example Support', assistant_name: 'Nova', brand_color: '#0f766e' } }
const now = new Date().toISOString()
const listed = [
  { reference: 'REQ-1', number: 42, subject: 'Broken export', status: 'active', last_activity_at: now, last_message_preview: 'We found the cause and are deploying a fix.', last_message_from: 'support', unread: true },
  { reference: 'REQ-2', number: 43, subject: 'Billing question', status: 'resolved', last_activity_at: now, last_message_preview: 'Thanks!', last_message_from: 'customer', unread: false },
]
const baseMessages = [
  { id: 'm1', sender_type: 'customer', content: 'The export fails.', created_at: now },
  { id: 'm2', sender_type: 'ai', sender_name: 'Nova', content: '**What to check**\n\n- The date range\n- Your plan\n\nSee [the guide](https://example.com/guide).', created_at: now },
]

async function mockPortal(page: Page) {
  let messages = [...baseMessages]
  let socket: WebSocketRoute | null = null
  await page.routeWebSocket(/\/public\/portal\/example\/ws$/, (ws) => { socket = ws })
  await page.route(`**${api}**`, (route) => {
    const path = new URL(route.request().url()).pathname.slice(api.length)
    const json = (body: unknown, status = 200) => route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(body) })
    if (path === '') return json(config)
    if (path === '/session') return json({ customer: { id: 'c1', email: 'customer@example.com' } })
    if (path === '/requests') return json(listed)
    if (path === '/requests/REQ-1') return json({ ...listed[0], can_reply: true, messages })
    return json({ error: 'unexpected' }, 500)
  })
  return {
    socket: () => socket,
    addAgentReply: () => { messages = [...messages, { id: 'm3', sender_type: 'user', sender_name: 'Priya', content: 'Fixed — please try again.', created_at: new Date().toISOString() }] },
  }
}

test.setTimeout(60_000)

test('the request table shows ticket numbers, latest activity, and new replies', async ({ page }) => {
  await mockPortal(page)
  await page.goto(`/portal/${slug}`)
  const rowFor = (name: string) => page.getByRole('listitem').filter({ has: page.getByRole('link', { name }) })
  const row = rowFor('Broken export, new reply')
  await expect(row).toContainText('#42')
  await expect(row).toContainText('We found the cause and are deploying a fix.')
  await expect(row).toContainText('Example Support replied')
  await expect(row).toContainText(/new reply/i)
  await expect(row).toContainText('just now')
  await expect(page.getByText('REQ-1')).toHaveCount(0)

  // Resolved requests start collapsed below the open ones, as completed work does in My Work.
  await expect(page.getByRole('link', { name: 'Billing question' })).toHaveCount(0)
  await page.getByRole('button', { name: 'Resolved (1)', expanded: false }).click()
  await expect(rowFor('Billing question')).toContainText('You replied')

  // Filters count locally and search narrows every group.
  const filters = page.getByRole('group', { name: 'Filter requests by status' })
  await expect(filters.getByRole('button', { name: 'All (2)' })).toBeVisible()
  await filters.getByRole('button', { name: 'Active (1)' }).click()
  await expect(page.getByRole('link', { name: 'Billing question' })).toHaveCount(0)
  await filters.getByRole('button', { name: 'All (2)' }).click()
  await page.getByLabel('Search requests').fill('#43')
  await expect(page.getByRole('link', { name: 'Billing question' })).toBeVisible()
  await expect(page.getByRole('link', { name: 'Broken export, new reply' })).toHaveCount(0)
  await page.getByLabel('Search requests').fill('nothing like this')
  await page.getByRole('button', { name: 'Clear filters' }).click()
  await expect(page.getByRole('link', { name: 'Broken export, new reply' })).toBeVisible()
})

test('replies render as formatted text and arrive live', async ({ page }) => {
  const portal = await mockPortal(page)
  await page.goto(`/portal/${slug}/requests/REQ-1`)
  await expect(page.getByText('What to check', { exact: true })).toHaveJSProperty('tagName', 'STRONG')
  await expect(page.getByRole('link', { name: 'the guide' })).toHaveAttribute('target', '_blank')
  await expect(page.getByText('**What to check**')).toHaveCount(0)
  await expect(page.getByText('#42')).toBeVisible()

  await expect.poll(() => portal.socket() !== null).toBe(true)
  portal.socket()!.send(JSON.stringify({ type: 'request:typing', reference: 'REQ-1', typing: true }))
  await expect(page.getByText('Example Support is replying…')).toBeVisible()

  portal.addAgentReply()
  portal.socket()!.send(JSON.stringify({ type: 'request:changed', reference: 'REQ-1' }))
  await expect(page.getByText('Fixed — please try again.')).toBeVisible()
  portal.socket()!.send(JSON.stringify({ type: 'request:typing', reference: 'REQ-1', typing: false }))
  await expect(page.getByText('Example Support is replying…')).toHaveCount(0)
})

test('a live refresh keeps the image viewer usable', async ({ page }) => {
  baseMessages.push({ id: 'img', sender_type: 'user', sender_name: 'Priya', content: 'Screenshot attached', created_at: now, attachments: [{ id: 'a1', file_name: 'shot.png', file_type: 'image/png', file_size: 70, url: 'https://storage.test/shot.png' }] } as never)
  try {
    const portal = await mockPortal(page)
    await page.route('https://storage.test/**', (route) => route.fulfill({ status: 200, contentType: 'image/png', body: Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==', 'base64') }))
    await page.goto(`/portal/${slug}/requests/REQ-1`)
    await page.getByRole('button', { name: 'Preview shot.png' }).click()
    await page.mouse.move(0, 0)
    await expect(page.locator('img[alt="shot.png"]')).toHaveCount(2)
    await expect.poll(() => portal.socket() !== null).toBe(true)
    portal.addAgentReply()
    portal.socket()!.send(JSON.stringify({ type: 'request:changed', reference: 'REQ-1' }))
    await expect(page.getByText('Fixed — please try again.')).toBeVisible()
    // The viewer stays open through the refresh and still closes with Escape.
    await expect(page.locator('img[alt="shot.png"]')).toHaveCount(2)
    await page.keyboard.press('Escape')
    await expect(page.locator('img[alt="shot.png"]')).toHaveCount(1)
  } finally {
    baseMessages.pop()
  }
})
