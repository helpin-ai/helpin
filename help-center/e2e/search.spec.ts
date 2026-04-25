import { expect, test } from '@playwright/test'

function apiURL(baseURL: string, path: string) {
  return new URL(path, baseURL.endsWith('/') ? baseURL : `${baseURL}/`).toString()
}

test('Ctrl+K opens command search', async ({ baseURL, page }) => {
  const pageErrors: string[] = []
  page.on('pageerror', (error) => {
    pageErrors.push(error.message)
  })

  await page.goto(baseURL!, { waitUntil: 'domcontentloaded' })
  await page.waitForLoadState('networkidle')
  await expect(page.getByText(/Search for articles|Search documentation/i).first()).toBeVisible()

  await page.locator('body').click({ position: { x: 20, y: 20 } })
  await page.keyboard.press('Control+K')
  await expect(page.getByRole('dialog')).toBeVisible()

  expect(pageErrors.filter((message) => message.includes('Minified React error'))).toEqual([])
})

test('search result links are canonical and remain responsive after navigation', async ({
  baseURL,
  page,
}) => {
  const pageErrors: string[] = []
  page.on('pageerror', (error) => {
    pageErrors.push(error.message)
  })

  await page.goto(baseURL!, { waitUntil: 'domcontentloaded' })
  await page.waitForLoadState('networkidle')
  await page.getByText(/Search for articles|Search documentation/i).first().click()
  await expect(page.getByRole('dialog')).toBeVisible()

  const input = page.getByRole('dialog').locator('input').first()
  await input.fill('api')

  const firstResult = page.getByRole('dialog').locator('a').first()
  await expect(firstResult).toBeVisible()

  const href = await firstResult.getAttribute('href')
  expect(href).toBeTruthy()

  const configResponse = await page.request.get(apiURL(baseURL!, 'api/hc/usermaven/config'))
  if (configResponse.ok()) {
    const config = await configResponse.json()
    const enabledLocales = Array.isArray(config.enabled_locales)
      ? config.enabled_locales.filter((locale: unknown) => String(locale || '').trim())
      : []
    if (enabledLocales.length <= 1) {
      expect(href).not.toMatch(/\/[a-z]{2}(?:-[a-z0-9]+)?\/articles\//i)
    }
  }

  await firstResult.evaluate((element) => {
    ;(element as HTMLAnchorElement).click()
  })

  await expect
    .poll(
      async () =>
        page
          .evaluate(() => ({
            href: window.location.href,
            readyState: document.readyState,
            nodeCount: document.querySelectorAll('*').length,
          }))
          .catch((error) => ({
            href: `evaluate failed: ${String(error)}`,
            readyState: 'unknown',
            nodeCount: 0,
          })),
      { timeout: 10_000 },
    )
    .toMatchObject({
      readyState: expect.stringMatching(/interactive|complete/),
    })

  await expect(page).toHaveURL(/\/articles\//)
  await expect(page.locator('article, main, body')).toContainText(/\w+/)
  expect(pageErrors.filter((message) => message.includes('Minified React error'))).toEqual([])
})
