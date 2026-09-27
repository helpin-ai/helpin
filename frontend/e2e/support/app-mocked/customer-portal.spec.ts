import { expect, test, type Page } from '@playwright/test'

const slug = 'example'
const root = `/portal/${slug}`
const api = `/api/public/portal/${slug}`
const customer = { customer: { id: 'customer-1', email: 'customer@example.com' } }
const config = { enabled: true, requests_only: false, intake_enabled: true, anonymous_intake_enabled: true, file_uploads_enabled: false, branding: { name: 'Example support' } }
const request = { reference: 'REQ-1', subject: 'Existing request', status: 'resolved', last_activity_at: '2026-01-01T12:00:00Z' }
const detail = { ...request, can_reply: true, messages: [{ id: 'message-1', content: 'Customer question', sender_type: 'customer', created_at: '2026-01-01T12:00:00Z' }] }

async function mockPortal(page: Page, options: { disabled?: boolean; anonymous?: boolean; expired?: boolean } = {}) {
  let requests = [request]
  let current = detail
  await page.route(`**${api}**`, async (route) => {
    const url = new URL(route.request().url())
    const path = url.pathname.slice(api.length)
    const method = route.request().method()
    const json = (body: unknown, status = 200) => route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(body) })
    if (path === '' && method === 'GET') return json(options.disabled ? { error: 'portal unavailable' } : config, options.disabled ? 404 : 200)
    if (path === '/session' && method === 'GET') return json(options.anonymous ? { error: 'invalid session' } : customer, options.anonymous ? 401 : 200)
    if (path === '/auth/magic-link' && method === 'POST') return route.fulfill({ status: 204 })
    if (path === '/intake/session' && method === 'POST') return json({ intake_token: 'intake-token' }, 201)
    if (path === '/intake/requests' && method === 'POST') {
      expect(route.request().headers().authorization).toBe('Bearer intake-token')
      expect(route.request().postDataJSON()).toMatchObject({ subject: 'A public request', message: 'Please help me' })
      return route.fulfill({ status: 202 })
    }
    if (path === '/auth/exchange' && method === 'POST') return json(options.expired ? { error: 'expired' } : customer, options.expired ? 401 : 200)
    if (path === '/requests' && method === 'GET') return json(requests)
    if (path === '/requests' && method === 'POST') {
      const body = route.request().postDataJSON() as { subject: string; message: string }
      current = { ...detail, reference: 'REQ-2', subject: body.subject, status: 'active', messages: [{ ...detail.messages[0], content: body.message }] }
      requests = [current, ...requests]
      return json(current, 201)
    }
    if (path === '/requests/REQ-1' && method === 'GET' || path === '/requests/REQ-2' && method === 'GET') return json(current)
    if (path === '/requests/REQ-1/replies' && method === 'POST') {
      const body = route.request().postDataJSON() as { content: string }
      current = { ...current, status: 'active', messages: [...current.messages, { ...detail.messages[0], id: 'message-2', content: body.content }] }
      return json(current)
    }
    return json({ error: 'unexpected portal request' }, 500)
  })
}

for (const viewport of [{ width: 1280, height: 800 }, { width: 375, height: 812 }]) {
  test.describe(`${viewport.width}px portal`, () => {
    test.setTimeout(60_000)
    test.use({ viewport })

    test('sign-in, My Requests, detail, reply/reopen and intake stay navigable', async ({ page }) => {
      await mockPortal(page, { anonymous: true })
      await page.goto(`${root}/sign-in`)
      await expect(page.getByRole('heading', { name: 'Sign in to your requests' })).toBeVisible()
      await page.getByLabel('Email address', { exact: true }).fill('customer@example.com')
      await page.getByRole('button', { name: 'Email me a sign-in link' }).click()
      await expect(page.getByText('Check your email')).toBeVisible()
      // Exchange establishes the session; a subsequent navigation uses the authenticated response.
      await page.unrouteAll()
      await mockPortal(page)
      await page.goto(`${root}/callback?token=valid`)
      await expect(page.getByRole('heading', { name: 'Your requests' })).toBeVisible()
      await expect(page.getByText('Existing request')).toBeVisible()
      await page.getByRole('link', { name: 'Existing request' }).click()
      await expect(page.getByText('Customer question')).toBeVisible()
      await page.getByLabel('Reply').fill('Please reopen this request')
      await page.getByRole('button', { name: 'Send reply' }).click()
      await expect(page.getByText('Please reopen this request')).toBeVisible()
      await page.getByRole('link', { name: 'All requests' }).click()
      await page.getByRole('button', { name: 'New request' }).click()
      await page.getByLabel('Subject').fill('New issue')
      await page.getByLabel('How can we help?').fill('A new customer issue')
      await page.getByRole('button', { name: 'Send request' }).click()
      await expect(page.getByRole('link', { name: 'New issue' })).toBeVisible()
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
    })

    test('expired session returns to sign-in without exposing requests', async ({ page }) => {
      await mockPortal(page, { anonymous: true })
      await page.goto(root)
      await expect(page.getByRole('heading', { name: 'Sign in to your requests' })).toBeVisible()
      await expect(page.getByText('Existing request')).toHaveCount(0)
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
    })

    test('anonymous intake sends a request and prompts email verification', async ({ page }) => {
      await mockPortal(page, { anonymous: true })
      await page.goto(`${root}/sign-in`)
      await page.getByLabel('Your email address').fill('public@example.com')
      await page.getByRole('button', { name: 'Continue' }).click()
      await page.getByLabel('Subject').fill('A public request')
      await page.getByLabel('How can we help?').fill('Please help me')
      await page.getByRole('button', { name: 'Send request' }).click()
      await expect(page.getByText('Request received. Check your email for a link to view and reply to it.')).toBeVisible()
      await expect(page.getByText('Existing request')).toHaveCount(0)
    })

    test('disabled portal and expired sign-in link provide recovery without overflow', async ({ page }) => {
      await mockPortal(page, { disabled: true })
      await page.goto(root)
      await expect(page.getByText(/portal.*unavailable|unavailable/i).first()).toBeVisible()
      await page.unrouteAll()
      await mockPortal(page, { anonymous: true, expired: true })
      await page.goto(`${root}/callback?token=expired`)
      await expect(page.getByRole('heading', { name: 'This link is no longer valid' })).toBeVisible()
      await page.getByRole('link', { name: 'Request a new link' }).click()
      await expect(page.getByRole('heading', { name: 'Sign in to your requests' })).toBeVisible()
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
    })
  })
}
