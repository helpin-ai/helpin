// From frontend, after building the app: node design-preview/slack/capture.mjs <output-directory>
import { mkdir, stat } from 'node:fs/promises'
import { resolve, sep } from 'node:path'
import { fileURLToPath } from 'node:url'
import { chromium, expect } from '@playwright/test'
import { CONVERSATION_ID, installSupportAppMocks } from '../../e2e/support/fixtures/supportE2E.ts'
import { now, responseFor, taskId, workspace } from './fixtures.mjs'

const dist = fileURLToPath(new URL('../../dist', import.meta.url))
const output = resolve(process.argv[2])
await stat(resolve(dist, 'index.html'))
await mkdir(output, { recursive: true })
const browser = await chromium.launch()
try {
  const page = await browser.newPage({
    viewport: { width: 1440, height: 900 }, deviceScaleFactor: 1.5, timezoneId: 'UTC',
    locale: 'en-US', colorScheme: 'light', reducedMotion: 'reduce', serviceWorkers: 'block',
  })
  const origin = 'http://localhost:8080'
  await page.route('**/*', async route => {
    const url = new URL(route.request().url())
    if (url.origin !== origin) return route.abort()
    const file = resolve(dist, url.pathname.startsWith('/w/') ? 'index.html' : '.' + decodeURIComponent(url.pathname))
    if (!file.startsWith(dist + sep) || !(await stat(file).catch(() => null))?.isFile()) return route.fulfill({ status: 404, body: '' })
    return route.fulfill({ path: file })
  })
  await installSupportAppMocks(page, { pmAccess: true })
  await page.route('**/api/**', route => {
    const path = new URL(route.request().url()).pathname
    if (process.env.DEBUG_SHOTS) console.log(route.request().method(), path)
    return route.fulfill({ json: responseFor(path) })
  })
  await page.clock.setFixedTime(new Date(now))
  page.setDefaultTimeout(9000)
  if (process.env.DEBUG_SHOTS) page.on('console', msg => { if (msg.type() === 'error') console.error(msg.text()) })
  const errors = []
  page.on('pageerror', e => { errors.push(e.message); console.error(e) })
  async function go(path) {
    await page.goto(origin + '/w/' + workspace.slug + '/' + path)
  }
  async function shot(file, hovered) {
    await page.waitForLoadState('networkidle')
    await page.evaluate(() => document.fonts.ready)
    if (hovered) await hovered.hover()
    else await page.mouse.move(0, 0)
    if (await page.getByText('This part of Helpin couldn’t load').count()) throw new Error('App error boundary on ' + file)
    if (errors.length) throw new Error(errors.join('\n'))
    await page.screenshot({ path: resolve(output, file + '.png'), animations: 'disabled' })
    console.log('Captured', file)
  }
  try {
    if (process.env.INBOX_FEED_REVIEW) {
      await go('settings/external-tools?tab=apps')
      const rule = page.getByRole('button', { name: 'Edit rule for #support-desk' })
      await rule.waitFor()
      await shot('channel-rules')
      await shot('channel-rule-hover', rule)
      await rule.click()
      const dialog = page.getByRole('dialog')
      await dialog.getByText('#support-desk', {exact:true}).waitFor()
      await shot('inbox-rule')
      await dialog.getByRole('button', {name: /^Inboxes:/}).click()
      await shot('inbox-selection')
      await page.keyboard.press('Escape')
      await dialog.getByRole('button', {name:'Cancel'}).click()
      await page.getByRole('button', {name:/#acme-support/}).click()
      await dialog.getByText(/#acme-support · Slack Connect/).waitFor()
      await shot('customer-intake')
      await go('settings/external-tools?tab=apps&slack_inbox=shared')
      await dialog.getByRole('button', {name:'Cancel'}).click()
      await page.getByRole('button', {name:'Add rule'}).click()
      await dialog.getByRole('button', {name:/^Slack channel:/}).click()
      await page.getByText('#customer-support', {exact:true}).waitFor()
      await Promise.all([
        page.waitForResponse(res => new URL(res.url()).pathname === '/api/slack/channels'),
        page.getByRole('button', {name:'Refresh', exact:true}).click(),
      ])
      await shot('channel-picker')
      await page.keyboard.press('Escape')
      await dialog.getByRole('combobox', {name:'Channel purpose'}).click()
      await page.getByRole('option', {name:'Handle Helpin inboxes in Slack'}).click()
      await dialog.getByRole('button', {name:/^Inboxes:/}).click()
      await page.getByText('Main inbox', {exact:true}).last().click()
      await page.keyboard.press('Escape')
      await dialog.getByRole('button', {name:/^Slack channel:/}).click()
      await page.getByText('#customer-support', {exact:true}).click()
      await dialog.getByText(/Everyone with access/).waitFor()
      await shot('public-channel')
      await go('settings/inboxes-routing?tab=inboxes')
      await page.getByRole('button', {name:'Actions for Shared inbox'}).click()
      const connect = page.getByRole('menuitem', {name:'Connect Slack'})
      await connect.waitFor()
      await shot('inboxes')
      await connect.click()
      await dialog.getByRole('button', {name:'Inboxes: Main inbox'}).waitFor()
      await go('settings/inboxes-routing?tab=inboxes')
      const actions = page.getByRole('button', {name:'Actions for Customer support'})
      await actions.click()
      const shortcut = page.getByRole('menuitem', {name:/Manage Slack channel.*#support-desk/})
      await shortcut.waitFor()
      await shot('inbox-actions')
      await page.getByRole('menuitem', {name:'Edit inbox'}).click()
      await dialog.getByRole('button', {name:/Email & Slack/}).click()
      await dialog.getByRole('link', {name:'Manage Slack channel'}).waitFor()
      await shot('inbox-settings')
      await page.keyboard.press('Escape')
      await actions.click()
      await shortcut.click()
      await dialog.getByText('#support-desk', {exact:true}).waitFor()
      await go('support/' + CONVERSATION_ID + '?view=team&team_inbox=support')
      const via = page.getByRole('link', { name:'via Slack' })
      await expect(via).toHaveCount(3)
      await via.last().scrollIntoViewIfNeeded()
      await shot('conversation')
      await page.getByRole('button', {name:'Open conversation actions'}).last().click()
      await page.getByText('Open Slack thread', {exact:true}).waitFor()
      await shot('conversation-actions')
    } else {
    await go('settings/support-channels')
    const slackShortcut = page.getByRole('link', { name: 'Manage Slack' })
    await slackShortcut.waitFor()
    await shot('channels')
    await slackShortcut.click()
    await page.getByRole('button', { name: /#acme-support/ }).waitFor()
    await shot('channel-rules')
    await page.getByRole('button', { name: /#acme-support/ }).click()
    const dialog = page.getByRole('dialog')
    await dialog.getByText('Every new customer thread', { exact: true }).first().waitFor()
    await shot('routing')
    await dialog.getByRole('button', { name: 'Cancel' }).click()
    await page.getByRole('button', { name: /#payments/ }).click()
    await dialog.getByText('Checkout reliability', { exact: true }).waitFor()
    await shot('project-routing')
    await page.setViewportSize({ width: 430, height: 870 })
    await go('support?view=team&team_inbox=support')
    await page.getByText('Mia Chen', { exact: true }).first().waitFor()
    await shot('list')
    await page.setViewportSize({ width: 1440, height: 900 })
    await go('support/' + CONVERSATION_ID + '?view=team&team_inbox=support')
    await page.locator('[contenteditable="true"].tiptap').fill('Lena, we found the issue with FALL20. The fix is in review. I’ll let you know as soon as it’s live.')
    await page.locator('[data-support-delivery-notice]').getByText('#acme-support', { exact: true }).waitFor()
    await page.getByRole('button', { name: 'Sending options: Slack only', exact: true }).waitFor()
    const sources = page.getByRole('link', { name: /Received from Slack/ })
    await expect(sources).toHaveCount(2)
    const replySource = sources.last()
    const replyURL = responseFor('/api/support/inbox/messages/message-2/slack-source').permalink
    await expect(replySource).toHaveAttribute('href', replyURL)
    await shot('inbox')
    const incoming = page.locator('[data-slot="support-message-bubble"]').filter({ hasText: 'Two orders so far' })
    await incoming.screenshot({ path: resolve(output, 'incoming-detail.png'), animations: 'disabled' })
    await page.context().route('https://northstar-example.slack.com/**', route => route.fulfill({ body: 'Fixture message' }))
    const [sourceTab] = await Promise.all([page.waitForEvent('popup'), replySource.click()])
    await expect(sourceTab).toHaveURL(replyURL)
    await sourceTab.close()
    await expect(page.locator('[contenteditable="true"].tiptap')).toContainText('The fix is in review.')
    await page.mouse.move(0, 0)
    await page.locator('[data-support-reply-composer]').screenshot({ path: resolve(output, 'composer-detail.png'), animations: 'disabled' })
    await go('pm/tasks/' + taskId)
    await page.getByText('Replies sync to #payments', { exact: true }).waitFor()
    await shot('task')
    await page.locator('[data-comment-thread-id]').filter({ hasText: '#payments' }).screenshot({ path: resolve(output, 'comment-detail.png'), animations: 'disabled' })
    }
  } catch (error) {
    console.error(page.url(), (await page.locator('body').innerText()).slice(0, 6500))
    throw error
  }
} finally {
  await browser.close()
}
