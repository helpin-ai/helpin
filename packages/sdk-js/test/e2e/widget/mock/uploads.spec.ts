import { expect, test, type Page } from '@playwright/test'
import { installWidgetMocks, WIDGET_HOST, WIDGET_KEY } from './widgetE2E'

const image = { name: '夏 Image With Spaces.png', mimeType: 'image/png', buffer: Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jRZkAAAAASUVORK5CYII=', 'base64') }
const storageURL = `https://${WIDGET_HOST}/uploads/**`
async function boot(page: Page, deferSession = false) {
  await installWidgetMocks(page, { deferSession })
  await page.goto('/test/e2e/widget/mock/test-page.html')
  await page.waitForFunction(() => typeof window.helpin === 'function')
  await page.evaluate(({ key, host }) => window.helpin?.('boot', { key, host }), { key: WIDGET_KEY, host: WIDGET_HOST })
  if (!deferSession) await expect.poll(() => page.evaluate(() => window.__widgetE2E?.isReady)).toBe(true)
  await page.evaluate(() => window.helpin?.('open'))
  await expect(page.locator('.helpin-chat-window')).toBeVisible()
}
async function confirmations(page: Page) {
  return page.evaluate(() => window.__widgetE2E?.getRequests().filter(r => r.url.endsWith('/confirm')) || [])
}

test('uploads real image bytes with only the signed content type and confirms', async ({ page }) => {
  await boot(page)
  let transferred = false
  await page.route(storageURL, async route => {
    const request = route.request()
    expect(request.method()).toBe('PUT')
    expect(request.headers()['content-type']).toBe('image/png')
    expect(request.headers()['x-amz-acl']).toBeUndefined()
    expect(request.headers()['x-session-token']).toBeUndefined()
    expect(request.postDataBuffer()).toEqual(image.buffer)
    transferred = true
    await route.fulfill({ status: 200, headers: { 'Access-Control-Allow-Origin': '*' }, body: '' })
  })
  await page.locator('input[type=file]').setInputFiles(image)
  await expect(page.getByText(/Ready to send/)).toBeVisible()
  expect(transferred).toBe(true)
  expect(await confirmations(page)).toHaveLength(1)
})

for (const failure of ['403', 'network', 'stall']) {
  test(`${failure} preserves draft, blocks sending and recovers on retry`, async ({ page }) => {
    await boot(page)
    let attempts = 0
    await page.route(storageURL, async route => {
      attempts++
      if (attempts > 1) return route.fulfill({ status: 200, headers: { 'Access-Control-Allow-Origin': '*' }, body: '' })
      if (failure === '403') return route.fulfill({ status: 403, body: '<Error>SignatureDoesNotMatch</Error>' })
      if (failure === 'network') return route.abort('failed')
      // Keep the actual browser XHR pending; advance its inactivity deadline below.
    })
    await page.clock.install()
    await page.locator('.helpin-compose-input').fill('Please see this image')
    await page.locator('input[type=file]').setInputFiles(image)
    await expect.poll(() => attempts).toBe(1)
    if (failure === 'stall') await page.clock.runFor(31_000)
    await expect(page.getByText(/Upload failed/)).toBeVisible()
    await expect(page.locator('.helpin-compose-send')).toBeDisabled()
    expect(await confirmations(page)).toHaveLength(0)
    await expect(page.locator('.helpin-compose-input')).toHaveValue('Please see this image')
    await page.getByRole('button', { name: 'Retry', exact: true }).click()
    await expect(page.getByText(/Ready to send/)).toBeVisible()
    expect(attempts).toBe(2)
    expect(await confirmations(page)).toHaveLength(1)
  })
}

test('cancel removes a pending upload without confirming it', async ({ page }) => {
  await boot(page)
  let started = false
  await page.route(storageURL, () => { started = true })
  await page.locator('input[type=file]').setInputFiles(image)
  await expect.poll(() => started).toBe(true)
  await page.getByRole('button', { name: `Cancel upload of ${image.name}`, exact: true }).click()
  await expect(page.getByTitle(image.name)).toHaveCount(0)
  expect(await confirmations(page)).toHaveLength(0)
})


test('upload before session connection retains image and succeeds after connection on retry', async ({ page }) => {
  await boot(page, true)
  await page.locator('input[type=file]').setInputFiles(image)
  await expect(page.getByText('Chat is not connected yet. Please wait and retry the upload.')).toBeVisible()
  expect(await confirmations(page)).toHaveLength(0)
  await page.evaluate(() => window.__widgetE2E?.emit({ type: 'session:joined', data: {
    session_token: 'connected-session', is_anonymous: true, conversations: [], messages: [],
  } }))
  await page.getByRole('button', { name: 'Retry', exact: true }).click()
  await expect(page.getByText(/Ready to send/)).toBeVisible()
  expect(await confirmations(page)).toHaveLength(1)
})

test('confirmation failure never shows ready and supports a fresh retry', async ({ page }) => {
  await boot(page)
  await page.evaluate(() => {
    const fetch = window.fetch.bind(window)
    let fail = true
    window.fetch = async (input, init) => {
      const url = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url
      if (url.endsWith('/confirm') && fail) {
        fail = false
        return new Response(JSON.stringify({ error: 'Storage object is not available. Please retry.' }), { status: 409 })
      }
      return fetch(input, init)
    }
  })
  await page.locator('input[type=file]').setInputFiles(image)
  await expect(page.getByText('Storage object is not available. Please retry.')).toBeVisible()
  await expect(page.getByText(/Ready to send/)).toHaveCount(0)
  await expect(page.locator('.helpin-compose-send')).toBeDisabled()
  await page.getByRole('button', { name: 'Retry', exact: true }).click()
  await expect(page.getByText(/Ready to send/)).toBeVisible()
})

test('uploads a 4.9 MB PDF through the same widget path', async ({ page }) => {
  await boot(page)
  const buffer = Buffer.alloc(5_110_250, 32)
  buffer.write('%PDF-1.4\n% upload regression fixture\n')
  await page.locator('input[type=file]').setInputFiles({ name: 'Large document.pdf', mimeType: 'application/pdf', buffer })
  await expect(page.getByText('4.9 MB · Ready to send')).toBeVisible()
  expect(await confirmations(page)).toHaveLength(1)
})
