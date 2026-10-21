import { expect, test, type Page } from '@playwright/test';
import type { SupportConversation } from '../../../src/lib/pmTypes';
import {
  buildConversation,
  CONVERSATION_ID,
  installSupportAppMocks,
  WORKSPACE_SLUG,
} from '../fixtures/supportE2E';

async function setup(page: Page, canEdit = true) {
  await installSupportAppMocks(page);
  let conversation = {
    ...buildConversation(),
    flow_state: 'ai_handling',
    ai_state: 'pending',
    assigned_agent_id: 'ai-agent',
    last_public_message_id: 'source',
    last_public_sender_type: 'ai',
    last_public_message_at: '2026-10-10T10:15:00Z',
    ai_follow_up: {
      id: 'episode',
      source_message_id: 'source',
      run_id: 'run',
      status: 'scheduled',
      sequence_version: 2,
      due_at: '2026-10-11T10:15:00Z',
      created_at: '2026-10-10T10:15:05Z',
      updated_at: '2026-10-10T10:15:05Z',
    },
  } as SupportConversation;
  const messages = [
    {
      id: 'source',
      workspace_id: 'ws-1',
      conversation_id: CONVERSATION_ID,
      sender_type: 'ai',
      sender_display_name: 'Helpin AI',
      content: 'Could you share a screenshot of the error and the file type?',
      message_type: 'reply',
      is_internal: false,
      created_at: '2026-10-10T10:15:00Z',
      updated_at: '2026-10-10T10:15:00Z',
    },
  ];
  if (!canEdit)
    await page.route(
      (url) => url.pathname === '/api/workspaces/ws-1/me',
      (route) =>
        route.fulfill({
          json: {
            workspace_id: 'ws-1',
            modules: ['support'],
            membership: { role: 'viewer', status: 'active' },
            permissions: ['workspace.read', 'support.read', 'settings.read', 'ws.connect'],
            team_memberships: [],
          },
        }),
    );
  await page.route(
    (url) => url.pathname === `/api/support/inbox/conversations/${CONVERSATION_ID}`,
    (route) => route.fulfill({ json: conversation }),
  );
  await page.route(
    (url) => url.pathname === `/api/support/inbox/conversations/${CONVERSATION_ID}/message-pages`,
    (route) => route.fulfill({ json: { data: messages, has_more: false, next_cursor: null } }),
  );
  await page.route(
    (url) =>
      url.pathname === `/api/support/inbox/conversations/${CONVERSATION_ID}/follow-up/cancel`,
    async (route) => {
      expect(route.request().postDataJSON()).toEqual({ follow_up_id: 'episode' });
      conversation = {
        ...conversation,
        ai_follow_up: {
          ...conversation.ai_follow_up!,
          status: 'cancelled',
          reason: 'cancelled_by_teammate',
          cancelled_by_user_id: 'user-b',
        },
      };
      await route.fulfill({ json: conversation });
    },
  );
  await page.goto(`/w/${WORKSPACE_SLUG}/support/${CONVERSATION_ID}`);
  await expect(page.locator('[data-support-follow-up]:visible')).toBeVisible({ timeout: 120_000 });
  return {
    update: async (patch: Partial<SupportConversation>) => {
      conversation = { ...conversation, ...patch };
      await page.evaluate(() =>
        window.__supportE2E?.emit({
          action: 'updated',
          entity: 'support_conversation',
          entity_id: 'conv-1',
          workspace_id: 'ws-1',
        }),
      );
    },
    get: () => conversation,
  };
}

for (const theme of ['light', 'dark'])
  test(`one live stage with stop action (${theme})`, async ({ page }, info) => {
    test.setTimeout(180_000);
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.addInitScript((theme) => localStorage.setItem('theme', theme), theme);
    const state = await setup(page);
    const row = page.locator('[data-support-follow-up]:visible');
    await expect(row).toHaveCount(1);
    await row.getByRole('button', { name: 'About this AI follow-up' }).click();
    await expect(page.getByRole('tooltip')).toContainText('Visible only to your team');
    await page.keyboard.press('Escape');
    await page
      .locator('[data-support-message-thread]')
      .screenshot({ path: info.outputPath(`follow-up-${theme}.png`) });
    await row.getByRole('button', { name: 'Stop follow-ups', exact: true }).click();
    await expect(row).toHaveAttribute('data-support-follow-up', 'stopped');
    await expect(row).toHaveCount(1);
    await expect(row.getByRole('button', { name: 'Stop follow-ups', exact: true })).toHaveCount(0);
    await row.getByRole('button', { name: 'About this AI follow-up' }).click();
    await expect(page.getByRole('tooltip')).toContainText('Bob Agent');
    await page.keyboard.press('Escape');
    await state.update({
      last_public_message_id: 'customer-reply',
      last_public_sender_type: 'customer',
    });
    await expect(row).toHaveCount(0);
  });

test.describe('viewer timezone and current stage on a phone', () => {
  test.use({ timezoneId: 'America/Los_Angeles', locale: 'en-US' });
  test('updates stages and keeps local years across UTC New Year', async ({ page }, info) => {
    test.setTimeout(180_000);
    await page.clock.setFixedTime(new Date('2026-12-31T20:00:00Z'));
    await page.setViewportSize({ width: 390, height: 844 });
    const state = await setup(page);
    const row = page.locator('[data-support-follow-up]:visible');
    await state.update({
      ai_follow_up: { ...state.get().ai_follow_up!, due_at: '2027-01-01T00:30:00Z' },
    });
    await expect(row.locator('time')).toHaveText('Today, 4:30 PM');
    await expect(row.locator('time')).toHaveAttribute('title', 'Dec 31, 4:30 PM');
    await state.update({
      ai_follow_up: { ...state.get().ai_follow_up!, due_at: '2027-01-01T09:30:00Z' },
    });
    await expect(row.locator('time')).toContainText('2027');
    await expect(row.locator('time')).toContainText('1:30 AM');
    await state.update({ ai_follow_up: { ...state.get().ai_follow_up!, status: 'assessing' } });
    await expect(row).toContainText('AI is checking whether to follow up');
    await expect(row.locator('time')).toHaveCount(0);
    await state.update({
      last_public_message_id: 'first',
      last_public_message_at: '2026-12-31T20:00:00Z',
      ai_follow_up: {
        ...state.get().ai_follow_up!,
        status: 'waiting',
        sent_message_id: 'first',
        sent_at: '2026-12-31T20:00:00Z',
      },
    });
    await expect(row).toContainText('Final AI follow-up scheduled');
    await state.update({
      last_public_message_id: 'second',
      last_public_message_at: '2026-12-31T21:00:00Z',
      ai_follow_up: {
        ...state.get().ai_follow_up!,
        second_message_id: 'second',
        second_sent_at: '2026-12-31T21:00:00Z',
        close_at: '2026-12-31T22:00:00Z',
      },
    });
    await expect(row.getByRole('button', { name: 'Stop auto-close' })).toBeVisible();
    await expect(row).toHaveCount(1);
    const rowBounds = await row.boundingBox();
    expect(rowBounds!.x + rowBounds!.width).toBeLessThanOrEqual(390);
    await page.screenshot({ path: info.outputPath('follow-up-phone.png') });
    await state.update({ human_takeover: true, assigned_user_id: 'user-a' });
    await expect(row).toHaveCount(0);
  });
});

test('viewers see the stage with no stop action', async ({ page }) => {
  test.setTimeout(180_000);
  await setup(page, false);
  await expect(page.locator('[data-support-follow-up]:visible')).toContainText(
    'AI follow-up scheduled',
  );
  await expect(page.getByRole('button', { name: 'Stop follow-ups', exact: true })).toHaveCount(0);
});
