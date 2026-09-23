// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest';
import { api } from '@/lib/api';
import type { Capability } from '@/lib/capabilityTypes';
import type { GitHubAppStatus } from '@/lib/pm-types/delivery';
import { assignBrowserLocation } from '@/lib/githubReturn';
import { button, capability, click, renderWithQuery, type Rendered } from '@/components/setup/__tests__/setupTestUtils';
import { ConnectGitHubStep } from '../ConnectGitHubStep';

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
  window.history.replaceState(null, '', '/');
});

const base: GitHubAppStatus = { configured: false, source: 'none', slug: '', install_url: '', webhook_configured: false, manifest_available: false };
const configured: Partial<GitHubAppStatus> = {
  configured: true, source: 'database', install_url: 'https://github.com/apps/helpin-acme/installations/new', webhook_configured: true,
};

function appStatus(status: Partial<GitHubAppStatus>, installURL?: { data?: unknown; error?: string }) {
  vi.mocked(api.get).mockImplementation(async (path: string) => {
    if (path === '/workspaces/ws-1/github/app-status') return { data: { ...base, ...status }, error: null, status: 200 } as never;
    if (installURL && path.startsWith('/git/github/install-url')) {
      return { data: installURL.data ?? null, error: installURL.error ?? null, status: installURL.error ? 400 : 200 } as never;
    }
    return { data: null, error: 'unexpected', status: 404 } as never;
  });
}

const notConfigured = capability({ key: 'github', action: { kind: 'open_settings', label: 'Set up GitHub', path: 'settings/git-connections' } });
const notInstalled = capability({ key: 'github', action: { kind: 'open_settings', label: 'Install the GitHub App', path: 'settings/git-connections' } });
const noRepositories = capability({ key: 'github', action: { kind: 'open_settings', label: 'Select repositories', path: 'settings/repositories' } });

function returnedStatus(container: HTMLElement) {
  return Array.from(container.querySelectorAll('[role="status"]')).find((element) => element.textContent);
}

async function renderStep(cap: Capability) {
  const onContinue = vi.fn();
  rendered = await renderWithQuery(<ConnectGitHubStep workspaceId="ws-1" capability={cap} onContinue={onContinue} />);
  return { container: rendered.container, onContinue };
}

describe('ConnectGitHubStep', () => {
  it('creates the GitHub App with a return to this onboarding step, and can be skipped', async () => {
    appStatus({ manifest_available: true });
    const { container, onContinue } = await renderStep(notConfigured);

    expect(button(container, 'Create GitHub App')?.dataset.returnTo).toBe('onboarding');
    expect(button(container, 'Continue')).toBeUndefined();
    await click(button(container, 'Skip for now'));
    expect(onContinue).toHaveBeenCalled();
  });

  it('shows why GitHub cannot be connected from this server', async () => {
    const reason = 'GitHub must reach this server to deliver events. Set APP_BASE_URL to a public https address (currently http://localhost:8080).';
    appStatus({ manifest_blocked_reason: reason });
    const { container } = await renderStep(notConfigured);

    expect(container.querySelector('[data-testid="github-blocked"]')?.textContent).toBe(reason);
    expect(button(container, 'Create GitHub App')).toBeUndefined();
    expect(button(container, 'Skip for now')).toBeDefined();
  });

  it('installs an existing App with a signed link that returns to onboarding', async () => {
    const signed = 'https://github.com/apps/helpin-acme/installations/new?state=signed';
    appStatus(configured, { data: { install_url: signed, action: 'install' } });
    const { container } = await renderStep(notInstalled);

    await click(button(container, 'Install the GitHub App'));
    expect(api.get).toHaveBeenCalledWith('/git/github/install-url?workspace_id=ws-1&return_to=onboarding');
    expect(assignBrowserLocation).toHaveBeenCalledWith(signed);
  });

  it('shows Connected and Continue once the App is installed', async () => {
    appStatus(configured);
    const { container, onContinue } = await renderStep(noRepositories);

    expect(container.querySelector('[data-testid="github-connected"]')?.textContent).toBe('Connected');
    expect(button(container, 'Install the GitHub App')).toBeUndefined();
    expect(button(container, 'Skip for now')).toBeUndefined();
    await click(button(container, 'Continue'));
    expect(onContinue).toHaveBeenCalled();
  });

  it('shows the result GitHub returned with inline and keeps the step in the URL', async () => {
    window.history.replaceState(null, '', '/onboarding?step=github&workspace=acme&github=connected&github_message=GitHub+App+connected+to+acme.&integration_id=gi-1');
    appStatus(configured);
    const { container } = await renderStep(notInstalled);

    expect(returnedStatus(container)?.textContent).toBe('GitHub App connected to acme.');
    expect(container.querySelector('[data-testid="github-connected"]')).not.toBeNull();
    expect(button(container, 'Continue')).toBeDefined();
    expect(`${window.location.pathname}${window.location.search}`).toBe('/onboarding?step=github&workspace=acme');
  });

  it('reports a failed return and still lets the owner install or skip', async () => {
    window.history.replaceState(null, '', '/onboarding?step=github&workspace=acme&github=error&github_message=GitHub+did+not+return+an+installation+ID.');
    appStatus(configured, { data: { install_url: 'https://github.com/x', action: 'install' } });
    const { container } = await renderStep(notInstalled);

    const status = returnedStatus(container);
    expect(status?.textContent).toBe('GitHub did not return an installation ID.');
    expect(status?.className).toContain('text-quiet-accent');
    expect(button(container, 'Install the GitHub App')).toBeDefined();
    expect(button(container, 'Skip for now')).toBeDefined();
  });
});
