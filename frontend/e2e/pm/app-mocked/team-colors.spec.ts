import { expect, test, type Page } from '@playwright/test';
import { installSupportAppMocks, WORKSPACE_ID } from '../../support/fixtures/supportE2E';

async function installTeamMocks(page: Page, options: { failOnce?: boolean; editable?: boolean } = {}) {
  await installSupportAppMocks(page, { pmAccess: true });
  const teams = [
    { id: 'devops', workspace_id: WORKSPACE_ID, name: 'DevOps', handle: 'devops', color: '#2da88e', team_type: 'engineering', sprints_enabled: true },
    { id: 'engineering', workspace_id: WORKSPACE_ID, name: 'Engineering', handle: 'engineering', color: '#4e8fea', team_type: 'engineering', sprints_enabled: true },
    { id: 'marketing', workspace_id: WORKSPACE_ID, name: 'Marketing', handle: 'marketing', color: '#8b5cf6', team_type: 'custom', sprints_enabled: false },
    { id: 'support', workspace_id: WORKSPACE_ID, name: 'Support', handle: 'support', color: '#e58c3a', team_type: 'custom', sprints_enabled: false },
  ];
  const writes: Array<Record<string, unknown>> = [];
  let failed = false;
  await page.route('**/api/**', async route => {
    const path = new URL(route.request().url()).pathname;
    const method = route.request().method();
    const json = (body: unknown, status = 200) => route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(body) });
    if (path === '/api/auth/config') return json({ email_verification_required: false, app_email_configured: true, google_login_enabled: false });
    if (path === `/api/workspaces/${WORKSPACE_ID}/me`) return json({
      workspace_id: WORKSPACE_ID, modules: ['pm', 'support', 'docs'],
      membership: { role: options.editable === false ? 'member' : 'owner', status: 'active' }, team_memberships: [],
      permissions: ['workspace.read', 'settings.read', 'pm.read', 'pm.edit', 'support.read', 'docs.read', ...(options.editable === false ? [] : ['team.manage', 'settings.manage'])],
    });
    if (path === '/api/settings' && method === 'GET') return json({
      settings: { id: 'settings', workspace_id: WORKSPACE_ID, notifications_enabled: true },
      teams, people: [], memberships: [], user_memberships: [], managers: [], job_role_criteria: [],
      invitation_team_preassignments: [], team_estimate_settings: [], team_field_visibility: [], team_repo_defaults: [],
    });
    if (path === '/api/settings/teams' && method === 'POST') {
      const body = route.request().postDataJSON();
      writes.push(body);
      const team = { ...body, id: 'new-team', handle: 'new-team', sprints_enabled: true };
      teams.push(team);
      return json(team);
    }
    if (path === '/api/settings/teams/devops' && method === 'PUT') {
      const body = route.request().postDataJSON();
      writes.push(body);
      if (options.failOnce && !failed) { failed = true; return json({ error: 'Could not save team' }, 500); }
      Object.assign(teams[0], body, { color: body.color || null });
      return json(teams[0]);
    }
    if (path.startsWith('/api/settings/teams/') && method === 'PUT') return json({});
    if (path.startsWith('/api/pm/') || path === '/api/git/repositories' || path === '/api/invitations') return json([]);
    return route.fallback();
  });
  return writes;
}

async function openEditor(page: Page) {
  await page.goto('/w/workspace/settings/teams?team=devops', { waitUntil: 'domcontentloaded' });
  await page.getByRole('button', { name: /^General Name/ }).click({ timeout: 90_000 });
  return page.getByRole('dialog', { name: 'Edit Team' });
}

test.beforeEach(async ({ page }) => {
  test.setTimeout(240_000);
  await page.setViewportSize({ width: 1440, height: 1000 });
});

test('one saved team color appears in Settings and on the Projects expand arrow', async ({ page }, testInfo) => {
  const writes = await installTeamMocks(page);
  const dialog = await openEditor(page);
  const color = dialog.getByRole('button', { name: 'Select color #e2564a' });
  await color.focus();
  await color.press('Enter');
  await expect(color).toHaveAttribute('aria-pressed', 'true');
  await page.screenshot({ path: testInfo.outputPath('team-color-picker.png'), animations: 'disabled' });
  await dialog.getByRole('button', { name: 'Save', exact: true }).click();
  await expect(dialog).toBeHidden();
  expect(writes[0].color).toBe('#e2564a');
  const badge = page.locator('span[style*="--team-color-bg"]').filter({ hasText: 'D' }).first();
  const savedBackground = await badge.evaluate(element => getComputedStyle(element).backgroundColor);
  await page.getByLabel('Projects', { exact: true }).click();
  const team = page.getByRole('button', { name: 'DevOps', exact: true });
  const arrow = team.locator('span[aria-hidden="true"]');
  await expect(arrow).toHaveCSS('background-color', savedBackground);
  const nameColor = await page.getByRole('button', { name: 'Engineering', exact: true }).locator('span.truncate').evaluate(element => getComputedStyle(element).color);
  await expect(team.locator('span.truncate')).toHaveCSS('color', nameColor);
  await team.click();
  await expect(team).toHaveAttribute('aria-expanded', 'false');
  await team.click();
  await expect(team).toHaveAttribute('aria-expanded', 'true');
  await page.screenshot({ path: testInfo.outputPath('team-colors-projects-light.png'), animations: 'disabled' });
  await page.evaluate(() => document.documentElement.classList.add('dark'));
  await expect(arrow).not.toHaveCSS('background-color', savedBackground);
  await page.screenshot({ path: testInfo.outputPath('team-colors-projects-dark.png'), animations: 'disabled' });
  await page.reload();
  await expect(team).toBeVisible();
  await expect(arrow).toHaveAttribute('style', /--team-color-bg:/);
});

test('custom colors keep the draft on failure and can be reset to Default', async ({ page }, testInfo) => {
  const writes = await installTeamMocks(page, { failOnce: true });
  const dialog = await openEditor(page);
  await dialog.getByRole('button', { name: 'Custom color', exact: true }).click();
  const hex = page.getByRole('textbox', { name: 'Hex color' });
  await hex.fill('#123456');
  await hex.press('Enter');
  await page.keyboard.press('Escape');
  await dialog.getByRole('button', { name: 'Save', exact: true }).click();
  await expect(page.getByText('Could not save team', { exact: true })).toBeVisible();
  await expect(dialog).toBeVisible();
  await expect(dialog.getByRole('button', { name: 'Custom color', exact: true })).toHaveAttribute('aria-pressed', 'true');
  await dialog.getByRole('button', { name: 'Save', exact: true }).click();
  await expect(dialog).toBeHidden();
  expect(writes[1].color).toBe('#123456');
  await page.getByRole('button', { name: /^General Name/ }).click();
  await dialog.getByRole('button', { name: 'Default', exact: true }).click();
  await expect(dialog.getByRole('button', { name: 'Default', exact: true })).toHaveAttribute('aria-pressed', 'true');
  await expect(dialog.getByRole('button', { name: 'Custom color', exact: true })).toHaveAttribute('aria-pressed', 'false');
  await page.setViewportSize({ width: 390, height: 844 });
  await page.evaluate(() => document.documentElement.classList.add('dark'));
  await page.screenshot({ path: testInfo.outputPath('team-color-mobile-dark.png'), animations: 'disabled' });
  expect(await dialog.evaluate(element => element.scrollWidth <= element.clientWidth)).toBe(true);
  await dialog.getByRole('button', { name: 'Save', exact: true }).click();
  await expect(dialog).toBeHidden();
  expect(writes[2].color).toBe('');
  await page.getByRole('button', { name: /^General Name/ }).click();
  await expect(dialog.getByRole('button', { name: 'Default', exact: true })).toHaveAttribute('aria-pressed', 'true');
});

test('team creation saves a color and read-only members can see colors without editing', async ({ page }) => {
  const writes = await installTeamMocks(page);
  await page.goto('/w/workspace/settings/teams', { waitUntil: 'domcontentloaded' });
  await page.getByRole('button', { name: 'New Team', exact: true }).click({ timeout: 90_000 });
  const dialog = page.getByRole('dialog', { name: 'Create Team' });
  await expect(dialog.locator('button[aria-label^="Select color"][aria-pressed="true"]')).toHaveCount(1);
  await expect(dialog.getByRole('button', { name: 'Default', exact: true })).toHaveCount(0);
  await dialog.getByRole('textbox').first().fill('Website Dev');
  await dialog.getByRole('button', { name: 'Select color #e54e78' }).click();
  await dialog.getByRole('button', { name: 'Save', exact: true }).click();
  await expect(dialog).toBeHidden();
  expect(writes[0]).toMatchObject({ name: 'Website Dev', color: '#e54e78' });
  await installTeamMocks(page, { editable: false });
  await page.goto('/w/workspace/settings/teams?team=devops');
  await expect(page.getByRole('button', { name: /^General Name/ })).toBeDisabled();
  await expect(page.locator('span[style*="--team-color-bg"]').filter({ hasText: 'D' }).first()).toBeVisible();
});
