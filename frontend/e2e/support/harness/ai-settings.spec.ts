import { expect, test, type Page } from '@playwright/test';
import { SUPPORT_INSTALLATION } from '../fixtures/supportE2E';

async function mock(
  page: Page,
  options: {
    empty?: boolean;
    loadError?: boolean;
    saveError?: boolean;
    humanRequest?: boolean;
    readOnly?: boolean;
    disabled?: boolean;
    delay?: number;
  } = {},
) {
  const writes: Record<string, unknown>[] = [];
  const controls: Record<string, unknown>[] = [];
  let loadErrors = options.loadError ? 1 : 0;
  let saveErrors = options.saveError ? 1 : 0;
  let settings: Record<string, unknown> = {
    ...SUPPORT_INSTALLATION.settings,
    ai_enabled: !options.empty && !options.disabled,
    ai_agent_id: options.empty ? null : 'echo',
    ai_response_mode: 'ai_first',
    ai_reply_channels: 'chat',
    ai_confidence_threshold: 0.7,
    ai_max_followups: 3,
    ai_follow_up_enabled: true,
    ai_follow_up_delay_hours: 24,
    ai_follow_up_second_delay_hours: 24,
    ai_follow_up_close_hours: 1,
  };
  let conversation = {
    id: 'conv',
    workspace_id: 'ws',
    subject: 'Login help',
    status: 'open',
    source: 'widget',
    channel: 'widget',
    human_takeover: true,
    ai_state: 'escalated',
    ai_control_version: 4,
    customer_requested_human_at: options.humanRequest
      ? '2026-09-18T12:00:00Z'
      : null,
  };
  await page.route('**/api/**', async (route) => {
    const request = route.request();
    const path = new URL(request.url()).pathname;
    const fulfill = (json: unknown, status = 200) =>
      route.fulfill({ json, status });
    if (path.endsWith('/installations')) {
      if (request.method() === 'PATCH') {
        writes.push(request.postDataJSON());
        if (saveErrors-- > 0)
          return fulfill({ error: 'Could not save settings' }, 503);
        settings = { ...settings, ...request.postDataJSON() };
      } else {
        if (options.delay)
          await new Promise((resolve) => setTimeout(resolve, options.delay));
        if (loadErrors-- > 0) return fulfill({ error: 'Unavailable' }, 503);
      }
      return fulfill({ ...SUPPORT_INSTALLATION, workspace_id: 'ws', settings });
    }
    if (path.endsWith('/settings'))
      return fulfill({
        settings: { workspace_id: 'ws' },
        teams: [],
        people: [],
        memberships: [],
        user_memberships: [],
      });
    if (path.endsWith('/me'))
      return fulfill({
        permissions: options.readOnly
          ? ['support.read']
          : ['support.admin', 'support.edit', 'settings.read'],
        modules: ['support'],
        membership: { role: 'admin' },
      });
    if (path.endsWith('/pm/agents'))
      return fulfill(
        options.empty
          ? []
          : [
              { id: 'echo', name: 'Echo', preset_key: 'support_agent' },
              {
                id: 'long',
                name: 'Customer success assistant for billing and account troubleshooting',
                preset_key: 'support_agent',
              },
            ],
      );
    if (path.endsWith('/conversations/conv')) return fulfill(conversation);
    if (path.endsWith('/ai-control')) {
      controls.push(request.postDataJSON());
      conversation = {
        ...conversation,
        human_takeover: false,
        ai_state: 'pending',
        ai_control_version: 5,
        customer_requested_human_at: null,
      };
      return fulfill({ updated: true });
    }
    return fulfill([]);
  });
  return { writes, controls };
}

for (const width of [1440, 390]) {
  test(`AI settings are labelled and responsive (${width}px)`, async ({
    page,
  }) => {
    await page.setViewportSize({ width, height: 1050 });
    const { writes } = await mock(page);
    await page.goto('/e2e/support/harness/ai-settings.html');
    await expect(
      page.getByRole('heading', { name: 'AI responses', exact: true }),
    ).toBeVisible();
    await expect(
      page.getByRole('combobox', { name: 'Support agent', exact: true }),
    ).toHaveText('Echo');
    await expect(
      page.getByRole('combobox', { name: 'Minimum confidence', exact: true }),
    ).toHaveText('70%');
    await expect(
      page.getByLabel('First check-in', { exact: true }),
    ).toHaveValue('24');
    await page.getByText('Human handoff', { exact: true }).click();
    await page.getByRole('tab', { name: 'After hours', exact: true }).click();
    await expect(
      page.getByRole('link', { name: 'Set up business hours' }),
    ).toBeVisible();
    await page
      .getByRole('tab', { name: 'Delayed team reply', exact: true })
      .click();
    await expect(
      page.getByLabel('Wait before sending', { exact: true }),
    ).toBeVisible();
    await page.getByRole('tab', { name: 'Default', exact: true }).click();
    await page.screenshot({
      path: `/tmp/helpin-ai-settings-${width}-light.png`,
      fullPage: true,
    });
    await page.locator('html').evaluate((el) => el.classList.add('dark'));
    await page.screenshot({
      path: `/tmp/helpin-ai-settings-${width}-dark.png`,
      fullPage: true,
    });
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= window.innerWidth,
      ),
    ).toBe(true);
    await page
      .getByRole('combobox', { name: 'Response mode', exact: true })
      .focus();
    await page.keyboard.press('Enter');
    await page
      .getByRole('option', { name: 'Add internal note', exact: true })
      .click();
    await expect(
      page.getByText('Suggestions stay private for your team to review.'),
    ).toBeVisible();
    await expect
      .poll(() => writes.at(-1)?.ai_response_mode)
      .toBe('internal_note');
    await page
      .getByRole('combobox', { name: 'Support agent', exact: true })
      .click();
    await page
      .getByRole('option', {
        name: 'Customer success assistant for billing and account troubleshooting',
        exact: true,
      })
      .click();
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= window.innerWidth,
      ),
    ).toBe(true);
  });
}

test('follow-up timing saves and a failed save preserves changes for retry', async ({
  page,
}) => {
  const { writes } = await mock(page, { saveError: true });
  await page.goto('/e2e/support/harness/ai-settings.html');
  await page.getByLabel('Final follow-up', { exact: true }).fill('48');
  await expect(
    page.getByRole('button', { name: 'Retry', exact: true }),
  ).toBeVisible();
  await expect(page.getByLabel('Final follow-up', { exact: true })).toHaveValue(
    '48',
  );
  await page.getByRole('button', { name: 'Retry', exact: true }).click();
  await expect(page.getByText('Saved', { exact: true })).toBeVisible();
  expect(writes.at(-1)?.ai_follow_up_second_delay_hours).toBe(48);
  await page
    .getByRole('switch', { name: 'Follow up on quiet conversations' })
    .click();
  await expect(page.getByLabel('First check-in', { exact: true })).toHaveCount(
    0,
  );
  await expect.poll(() => writes.at(-1)?.ai_follow_up_enabled).toBe(false);
});

test('missing agent keeps AI disabled and explains how to enable it', async ({
  page,
}) => {
  await mock(page, { empty: true });
  await page.goto('/e2e/support/harness/ai-settings.html');
  await page.getByRole('switch', { name: 'Enable AI assistant' }).click();
  await expect(
    page
      .getByRole('alert')
      .filter({
        hasText: 'Create a support agent before enabling AI Assistant.',
      }),
  ).toBeVisible();
  await expect(
    page.getByRole('switch', { name: 'Enable AI assistant' }),
  ).not.toBeChecked();
});

test('load failure offers retry instead of an editable empty form', async ({
  page,
}) => {
  await mock(page, { loadError: true });
  await page.goto('/e2e/support/harness/ai-settings.html');
  await expect(
    page.getByText('Could not load support settings.'),
  ).toBeVisible();
  await expect(
    page.getByRole('switch', { name: 'Enable AI assistant' }),
  ).toHaveCount(0);
  await page.getByRole('button', { name: 'Retry', exact: true }).click();
  await expect(
    page.getByRole('switch', { name: 'Enable AI assistant' }),
  ).toBeChecked();
});

for (const width of [1440, 390]) {
  test(`Return to AI lives in the menu and keeps human-request confirmation (${width}px)`, async ({
    page,
  }) => {
    await page.setViewportSize({ width, height: 900 });
    const { controls } = await mock(page, { humanRequest: true });
    await page.goto('/e2e/support/harness/ai-settings.html?menu');
    await expect(
      page.getByRole('button', { name: 'Return to AI', exact: true }),
    ).toHaveCount(0);
    await page
      .getByRole('button', { name: 'Open conversation actions' })
      .click();
    await page
      .getByRole('menuitem', { name: 'Return to AI', exact: true })
      .click();
    await expect(page.getByRole('alertdialog')).toBeVisible();
    expect(controls).toHaveLength(0);
    await page
      .getByRole('button', { name: 'Keep with humans', exact: true })
      .click();
    expect(controls).toHaveLength(0);
    await page
      .getByRole('button', { name: 'Open conversation actions' })
      .click();
    await page
      .getByRole('menuitem', { name: 'Return to AI', exact: true })
      .click();
    await page
      .getByRole('button', { name: 'Return to AI', exact: true })
      .click();
    await expect.poll(() => controls.length).toBe(1);
    expect(controls[0]).toMatchObject({
      action: 'return',
      expected_version: 4,
      confirm_human_request: true,
    });
    await expect(page.getByRole('alertdialog')).toHaveCount(0);
    await page
      .getByRole('button', { name: 'Open conversation actions' })
      .click();
    await expect(
      page.getByRole('menuitem', { name: 'Pause AI', exact: true }),
    ).toBeVisible();
  });
}

test('menu respects permissions and workspace availability', async ({
  page,
}) => {
  await mock(page, { readOnly: true });
  await page.goto('/e2e/support/harness/ai-settings.html?menu');
  await page.getByRole('button', { name: 'Open conversation actions' }).click();
  await expect(
    page.getByRole('menuitem', { name: 'Return to AI', exact: true }),
  ).toHaveCount(0);
});

test('settings stay non-editable while loading', async ({ page }) => {
  await mock(page, { delay: 800 });
  await page.goto('/e2e/support/harness/ai-settings.html');
  await expect(page.getByRole('heading', { name: 'AI assistant', exact: true })).toBeVisible();
  await expect(page.getByRole('switch', { name: 'Enable AI assistant' })).toHaveCount(0);
  await expect(page.getByRole('switch', { name: 'Enable AI assistant' })).toBeChecked();
});
