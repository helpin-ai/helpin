import { expect, test } from '@playwright/test'

for (const width of [1440, 390]) {
  test(`visitor feedback leaves content and timestamps unobstructed (${width}px)`, async ({ page, baseURL }) => {
    test.setTimeout(90_000)
    await page.setViewportSize({ width, height: 900 })
    await page.route('**/api/**', route => route.fulfill({ json: [] }))
    await page.goto(`${baseURL}/e2e/support/harness/support-answer-feedback-harness.html`, { waitUntil: 'domcontentloaded' })
    const rows = page.locator('[data-slot="support-answer-feedback"]:visible')
    await expect(rows).toHaveCount(2, { timeout: 60_000 })
    for (let index = 0; index < 2; index++) {
      const row = rows.nth(index)
      const wrapper = row.locator('..')
      const frame = wrapper.locator('[data-slot="support-message-bubble-frame"]')
      const frameBox = (await frame.boundingBox())!
      const badgeBox = (await row.boundingBox())!
      expect(badgeBox.y).toBeGreaterThanOrEqual(frameBox.y + frameBox.height)
      expect(Math.abs(badgeBox.x - frameBox.x)).toBeLessThan(2)
      const timeBox = (await frame.locator('time').boundingBox())!
      expect(timeBox.y + timeBox.height).toBeLessThanOrEqual(badgeBox.y)
    }
    const badge = page.getByRole('img', { name: 'Visitor marked unhelpful', exact: true })
    await badge.focus()
    await expect(page.getByRole('tooltip')).toHaveText('Visitor marked unhelpful')
    await badge.blur()
    await expect(page.getByRole('tooltip')).toBeHidden()
    await page.screenshot({ path: `/tmp/helpin-feedback-${width}-light.png`, fullPage: true })
    await page.locator('html').evaluate(element => element.classList.add('dark'))
    await page.screenshot({ path: `/tmp/helpin-feedback-${width}-dark.png`, fullPage: true })
  })
}
