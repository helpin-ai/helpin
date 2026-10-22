import { expect, test, type Page } from '@playwright/test'

const slug = 'example'
const api = `/api/public/portal/${slug}`
const storage = 'https://storage.test'
const png = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==', 'base64')
const config = {
  enabled: true, intake_enabled: true, anonymous_intake_enabled: false, file_uploads_enabled: true,
  attachments: { max_bytes: 5_000_000, max_files: 10, content_types: ['image/png', 'image/heic', 'video/mp4', 'application/pdf'] },
  branding: { name: 'Example support' },
}
const request = { reference: 'REQ-1', subject: 'Broken export', status: 'active', last_activity_at: '2026-01-01T12:00:00Z' }
const question = { id: 'm1', content: 'The export fails.', sender_type: 'customer', created_at: '2026-01-01T12:00:00Z' }

async function mockPortal(page: Page) {
  let detail: Record<string, unknown> = { ...request, can_reply: true, messages: [question] }
  const uploaded: { id: string; name: string; type: string; size: number }[] = []
  let replyBody: { content: string; attachment_ids: string[] } | null = null
  // Keep the live-update socket open so the polling fallback never refetches mid-test.
  await page.routeWebSocket(/\/public\/portal\/example\/ws$/, () => {})
  await page.route(`${storage}/**`, (route) => {
    if (route.request().method() === 'PUT') return route.fulfill({ status: 200 })
    return route.fulfill({ status: 200, contentType: 'image/png', body: png })
  })
  await page.route(`**${api}**`, async (route) => {
    const path = new URL(route.request().url()).pathname.slice(api.length)
    const method = route.request().method()
    const json = (body: unknown, status = 200) => route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(body) })
    if (path === '' && method === 'GET') return json(config)
    if (path === '/session') return json({ customer: { id: 'c1', email: 'customer@example.com' } })
    if (path === '/requests' && method === 'GET') return json([request])
    if (path === '/requests/REQ-1' && method === 'GET') return json(detail)
    if (path === '/requests/REQ-1/attachments' && method === 'POST') {
      const body = route.request().postDataJSON() as { file_name: string; content_type: string; file_size: number }
      if (body.file_name === 'blocked.pdf') return json({ error: 'blocked.pdf could not be stored' }, 400)
      const id = `att-${uploaded.length + 1}`
      uploaded.push({ id, name: body.file_name, type: body.content_type, size: body.file_size })
      return json({ attachment: { id }, upload_url: `${storage}/put/${id}` }, 201)
    }
    if (/^\/requests\/REQ-1\/attachments\/att-\d+\/confirm$/.test(path)) return route.fulfill({ status: 204 })
    if (path === '/requests/REQ-1/replies' && method === 'POST') {
      replyBody = route.request().postDataJSON()
      const attachments = uploaded.filter((file) => replyBody!.attachment_ids.includes(file.id)).map((file) => ({
        id: file.id, file_name: file.name, file_type: file.type, file_size: file.size, url: `${storage}/files/${file.name}`,
      }))
      detail = { ...detail, messages: [question, { id: 'm2', content: replyBody!.content, sender_type: 'customer', created_at: '2026-01-02T12:00:00Z', attachments }] }
      return json(detail)
    }
    return json({ error: 'unexpected portal request' }, 500)
  })
  return { reply: () => replyBody }
}

async function dropFile(page: Page, selector: string, name: string, type: string) {
  const transfer = await page.evaluateHandle(({ name, type }) => {
    const data = new DataTransfer()
    data.items.add(new File(['dropped'], name, { type }))
    return data
  }, { name, type })
  await page.dispatchEvent(selector, 'dragenter', { dataTransfer: transfer })
  await page.dispatchEvent(selector, 'drop', { dataTransfer: transfer })
}

test('customers attach files by picker, drop, and send them without text', async ({ page }) => {
  test.setTimeout(60_000)
  const portal = await mockPortal(page)
  await page.goto(`/portal/${slug}/requests/REQ-1`)
  await expect(page.getByText('The export fails.')).toBeVisible()

  const picker = page.getByLabel('Attach files')
  await picker.setInputFiles([
    { name: 'shot.png', mimeType: 'image/png', buffer: png },
    { name: 'IMG_0001.HEIC', mimeType: '', buffer: Buffer.from('heic') },
  ])
  await dropFile(page, '#portal-reply', 'notes.pdf', 'application/pdf')
  const attached = page.getByRole('list', { name: 'Attached files' })
  await expect(attached.getByText('Ready to send')).toHaveCount(3)

  // Refused before upload by the server's rules, and refused by the server.
  await picker.setInputFiles([{ name: 'tool.exe', mimeType: 'application/x-msdownload', buffer: Buffer.from('x') }])
  await expect(page.getByRole('alert').filter({ hasText: 'tool.exe isn’t a supported file type' })).toBeVisible()
  await picker.setInputFiles([{ name: 'blocked.pdf', mimeType: 'application/pdf', buffer: Buffer.from('x') }])
  await expect(attached.getByText('blocked.pdf could not be stored')).toBeVisible()
  const send = page.getByRole('button', { name: 'Send reply' })
  await expect(send).toBeDisabled()
  await page.getByRole('button', { name: 'Remove blocked.pdf' }).click()
  await expect(send).toBeEnabled()

  await send.click()
  await expect.poll(() => portal.reply()?.attachment_ids.length).toBe(3)
  expect(portal.reply()?.content).toBe('')

  // The thread shows the image inline, the HEIC photo and PDF as downloads.
  const preview = page.getByRole('button', { name: 'Preview shot.png' })
  await expect(preview).toBeVisible()
  await expect(page.getByRole('link', { name: /IMG_0001\.HEIC/ })).toBeVisible()
  await expect(page.getByRole('link', { name: /notes\.pdf/ })).toBeVisible()
  await preview.click()
  await expect(page.locator('img[alt="shot.png"]')).toHaveCount(2)
  await page.keyboard.press('Escape')
  // Move off the thumbnail so its hover preview does not count as an image.
  await page.mouse.move(0, 0)
  await expect(page.locator('img[alt="shot.png"]')).toHaveCount(1)
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
})
