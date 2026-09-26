// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest';
import { api } from '@/lib/api';
import type { Capability } from '@/lib/capabilityTypes';
import type { GitHubAppStatus } from '@/lib/pm-types/delivery';
import { assignBrowserLocation } from '@/lib/githubReturn';
import { GITHUB_APP_ENV_HINT, SetupGitHubStep } from '../SetupGitHubStep';
import { button, capability, click, flush, renderWithQuery, type Rendered } from './setupTestUtils';

vi.mock('@/lib/api', () => ({ api: { get: vi.fn(), post: vi.fn() } }));
vi.mock('@tanstack/react-router', () => ({
  Link: ({ to, children }: { to: string; children: React.ReactNode }) => <a href={to}>{children}</a>,
}));
vi.mock('@/components/git/GitHubAppCreateButton', () => ({
  GitHubAppCreateButton: ({ workspaceId, returnTo }: { workspaceId?: string; returnTo?: string }) => (
    <button type="button" data-workspace={workspaceId} data-return-to={returnTo}>Create GitHub App</button>
  ),
}));
vi.mock('@/lib/githubReturn', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/lib/githubReturn')>()),
  assignBrowserLocation: vi.fn(),
}));

let rendered: Rendered | undefined;
afterEach(async () => {
  await rendered?.unmount();
  rendered = undefined;
  vi.mocked(api.get).mockReset();
  vi.mocked(assignBrowserLocation).mockReset();
});

const base: GitHubAppStatus = { configured: false, source: 'none', slug: '', install_url: '', webhook_configured: false, manifest_available: false };

function appStatus(status: Partial<GitHubAppStatus>, installURL?: { data?: unknown; error?: string }) {
  vi.mocked(api.get).mockImplementation(async (path: string) => {
    if (path === '/workspaces/ws-1/github/app-status') return { data: { ...base, ...status }, error: null, status: 200 } as never;
    if (installURL && path.startsWith('/git/github/install-url')) {
      return { data: installURL.data ?? null, error: installURL.error ?? null, status: installURL.error ? 400 : 200 } as never;
    }
    return { data: null, error: 'unexpected', status: 404 } as never;
  });
}

const notConfigured = capability({ key: 'github', detail: 'The GitHub App is not configured on this server.', action: { kind: 'open_settings', label: 'Set up GitHub', path: 'settings/git-connections' } });
const notInstalled = capability({ key: 'github', detail: 'The GitHub App is not installed for this organization.', action: { kind: 'open_settings', label: 'Install the GitHub App', path: 'settings/git-connections' } });
const noRepositories = capability({ key: 'github', action: { kind: 'open_settings', label: 'Select repositories', path: 'settings/repositories' } });

async function renderStep(cap: Capability, { isOwner = true, canManage = true, returnTo }: { isOwner?: boolean; canManage?: boolean; returnTo?: 'setup' | 'system_status' } = {}) {
  rendered = await renderWithQuery(<SetupGitHubStep capability={cap} workspaceId="ws-1" slug="acme" canManage={canManage} isOwner={isOwner} returnTo={returnTo} />);
  return rendered.container;
}

describe('SetupGitHubStep', () => {
  it('lets the owner create the GitHub App when the manifest flow is available', async () => {
    appStatus({ manifest_available: true });
    const container = await renderStep(notConfigured, { returnTo: 'setup' });

    expect(button(container, 'Create GitHub App')?.dataset.workspace).toBe('ws-1');
    expect(button(container, 'Create GitHub App')?.dataset.returnTo).toBe('setup');
  });

  it('explains when GitHub cannot reach this server', async () => {
    const reason = 'GitHub must reach this server to deliver events. Set APP_BASE_URL to a public https address (currently http://localhost:8080).';
    appStatus({ manifest_available: false, manifest_blocked_reason: reason });
    const container = await renderStep(notConfigured);

    expect(container.textContent).toBe(reason);
    expect(button(container, 'Create GitHub App')).toBeUndefined();
    expect(container.querySelector('code')).toBeNull();
  });

  it('does not repeat the reason when the visible capability detail already states it', async () => {
    const reason = 'GitHub must reach this server to deliver events. Set APP_BASE_URL to a public https address (currently http://localhost:8080).';
    appStatus({ manifest_available: false, manifest_blocked_reason: reason });
    const cap = capability({ key: 'github', detail: `The GitHub App is not configured on this server. ${reason}` });
    rendered = await renderWithQuery(<SetupGitHubStep capability={cap} workspaceId="ws-1" slug="acme" canManage isOwner detailShown />);
    await flush();

    expect(rendered.container.textContent).toBe('');
  });

  it('tells admins who is not the owner that the owner creates the App', async () => {
    appStatus({ manifest_available: true });
    const container = await renderStep(notConfigured, { isOwner: false });

    expect(button(container, 'Create GitHub App')).toBeUndefined();
    expect(container.textContent).toContain('The workspace owner can create the GitHub App');
  });

  it('shows server configuration when the App cannot be created from the browser', async () => {
    appStatus({ manifest_available: false });
    const container = await renderStep(notConfigured);

    expect(container.querySelector('code')?.textContent).toBe(GITHUB_APP_ENV_HINT);
    expect(button(container, 'Create GitHub App')).toBeUndefined();
  });

  it('installs with Helpin’s signed link and returns to the page it started from', async () => {
    const signed = 'https://github.com/apps/helpin-acme/installations/new?state=signed';
    appStatus(
      { configured: true, source: 'database', install_url: 'https://github.com/apps/helpin-acme/installations/new', webhook_configured: true, private: true, owner_login: 'acme', owner_type: 'Organization' },
      { data: { install_url: signed, action: 'install' } },
    );
    const container = await renderStep(notInstalled, { returnTo: 'system_status' });

    expect(container.querySelector('a[href^="https://github.com"]')).toBeNull();
    expect(container.textContent).toContain('Private App owned by acme. Install it on acme to connect its repositories.');
    await click(button(container, 'Install the GitHub App'));

    expect(api.get).toHaveBeenCalledWith('/git/github/install-url?workspace_id=ws-1&return_to=system_status');
    expect(assignBrowserLocation).toHaveBeenCalledWith(signed);
    expect(container.textContent).not.toContain('webhooks aren’t configured');
  });

  it('reports an install link failure inline', async () => {
    appStatus({ configured: true, source: 'env', install_url: 'https://github.com/apps/helpin/installations/new', webhook_configured: true, owner_login: 'acme' },
      { error: 'only organization owners or admins can connect git integrations' });
    const container = await renderStep(notInstalled);

    expect(container.textContent).not.toContain('Private App owned by');
    await click(button(container, 'Install the GitHub App'));

    expect(container.querySelector('[role="status"]')?.textContent).toBe('only organization owners or admins can connect git integrations');
    expect(assignBrowserLocation).not.toHaveBeenCalled();
  });

  it('points to repository selection once installed and flags a missing webhook secret', async () => {
    appStatus({ configured: true, source: 'env', install_url: 'https://github.com/apps/helpin/installations/new', webhook_configured: false });
    const container = await renderStep(noRepositories);

    expect(container.querySelector('a')?.getAttribute('href')).toBe('/w/acme/settings/repositories');
    expect(container.textContent).toContain('GitHub webhooks aren’t configured on this server');
  });

  it('is done when GitHub is ready and does not query the App', async () => {
    const container = await renderStep(capability({ key: 'github', status: 'ready', detail: '2 repository(ies) connected.' }));

    expect(container.innerHTML).toBe('');
    expect(api.get).not.toHaveBeenCalled();
  });

  it('shows members status only', async () => {
    const container = await renderStep(notConfigured, { canManage: false });

    expect(container.textContent).toContain('A workspace admin can finish this step.');
    expect(api.get).not.toHaveBeenCalled();
  });
});
