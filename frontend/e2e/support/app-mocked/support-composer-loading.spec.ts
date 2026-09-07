import { expect, test } from '@playwright/test'
import { CONVERSATION_ID, installSupportAppMocks, WORKSPACE_SLUG } from '../fixtures/supportE2E'

for (const width of [1440, 390]) {
  test(`composer reserves its space during data and editor loading (${width}px)`, async ({ page, baseURL }) => {
    test.setTimeout(90_000)
    await page.setViewportSize({ width, height: 900 })
    await installSupportAppMocks(page)
    let releaseData!: () => void
    let releaseEditor!: () => void
    const dataGate = new Promise<void>((resolve) => { releaseData = resolve })
    const editorGate = new Promise<void>((resolve) => { releaseEditor = resolve })
    let editorRequested = false
    await page.route(`**/api/support/inbox/conversations/${CONVERSATION_ID}**`, async (route) => {
      await dataGate
      await route.fallback()
    })
    await page.route('**/src/components/support/ReplyComposer.tsx*', async (route) => {
      editorRequested = true
      await editorGate
      await route.continue()
    })
    try {
      await page.goto(`${baseURL}/w/${WORKSPACE_SLUG}/support/${CONVERSATION_ID}`, { waitUntil: 'domcontentloaded' })
      const frame = page.locator('[data-support-reply-composer]:visible')
      await expect(frame).toHaveAttribute('aria-busy', 'true', { timeout: 45_000 })
      await expect(frame.getByRole('button', { name: 'Send', exact: true })).toBeDisabled()
      const loadingBox = await frame.boundingBox()
      expect(loadingBox).not.toBeNull()
      // Preloading must start before the conversation requests finish.
      await expect.poll(() => editorRequested).toBe(true)
      releaseData()
      await expect(page.locator('[data-support-message-list]:visible')).toContainText('Initial message')
      await expect(frame).toHaveAttribute('aria-busy', 'true')
      expect((await frame.boundingBox())?.y).toBeCloseTo(loadingBox!.y, 0)
      releaseEditor()
      await expect(frame.locator('[contenteditable="true"]')).toBeVisible()
      const readyBox = await frame.boundingBox()
      expect(readyBox!.y).toBeCloseTo(loadingBox!.y, 0)
      expect(readyBox!.height).toBeCloseTo(loadingBox!.height, 0)
    } finally {
      releaseData()
      releaseEditor()
    }
  })
}
