// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest';
import { api } from '@/lib/api';
import type { Capability } from '@/lib/capabilityTypes';
import type { GitHubAppStatus } from '@/lib/pm-types/delivery';
import { GITHUB_APP_ENV_HINT, SetupGitHubStep } from '../SetupGitHubStep';
import { button, capability, renderWithQuery, type Rendered } from './setupTestUtils';

vi.mock('@/lib/api', () => ({ api: { get: vi.fn(), post: vi.fn() } }));
vi.mock('@tanstack/react-router', () => ({
  Link: ({ to, children }: { to: string; children: React.ReactNode }) => <a href={to}>{children}</a>,
}));
vi.mock('@/components/git/GitHubAppCreateButton', () => ({
  GitHubAppCreateButton: ({ workspaceId }: { workspaceId?: string }) => <button type="button" data-workspace={workspaceId}>Create GitHub App</button>,
}));

let rendered: Rendered | undefined;
afterEach(async () => {
  await rendered?.unmount();
  rendered = undefined;
  vi.mocked(api.get).mockReset();
});

const base: GitHubAppStatus = { configured: false, source: 'none', slug: '', install_url: '', webhook_configured: false, manifest_available: false };

function appStatus(status: Partial<GitHubAppStatus>) {
  vi.mocked(api.get).mockImplementation(async (path: string) => path === '/workspaces/ws-1/github/app-status'
    ? { data: { ...base, ...status }, error: null, status: 200 } as never
    : { data: null, error: 'unexpected', status: 404 } as never);
}

const notConfigured = capability({ key: 'github', detail: 'The GitHub App is not configured on this server.', action: { kind: 'open_settings', label: 'Set up GitHub', path: 'settings/git-connections' } });
const notInstalled = capability({ key: 'github', detail: 'The GitHub App is not installed for this organization.', action: { kind: 'open_settings', label: 'Install the GitHub App', path: 'settings/git-connections' } });
const noRepositories = capability({ key: 'github', action: { kind: 'open_settings', label: 'Select repositories', path: 'settings/repositories' } });

async function renderStep(cap: Capability, { isOwner = true, canManage = true } = {}) {
  rendered = await renderWithQuery(<SetupGitHubStep capability={cap} workspaceId="ws-1" slug="acme" canManage={canManage} isOwner={isOwner} />);
  return rendered.container;
}

describe('SetupGitHubStep', () => {
  it('lets the owner create the GitHub App when the manifest flow is available', async () => {
    appStatus({ manifest_available: true });
    const container = await renderStep(notConfigured);

    expect(button(container, 'Create GitHub App')?.dataset.workspace).toBe('ws-1');
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

  it('links to the GitHub installation when the App is configured but not installed', async () => {
    appStatus({ configured: true, source: 'database', install_url: 'https://github.com/apps/helpin-acme/installations/new', webhook_configured: true });
    const container = await renderStep(notInstalled);

    const install = Array.from(container.querySelectorAll('a')).find((link) => link.textContent?.includes('Install the GitHub App'));
    expect(install?.getAttribute('href')).toBe('https://github.com/apps/helpin-acme/installations/new');
    expect(install?.getAttribute('target')).toBe('_blank');
    expect(install?.getAttribute('rel')).toContain('noopener');
    expect(container.textContent).not.toContain('webhooks aren’t configured');
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
