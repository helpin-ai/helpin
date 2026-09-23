// @vitest-environment jsdom
import { act } from 'react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, describe, expect, it, vi } from 'vitest';

vi.mock('@tanstack/react-router', () => ({ useNavigate: () => vi.fn() }));
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn(), info: vi.fn() } }));
vi.mock('@/stores/workspaceStore', () => ({
  useWorkspaceStore: (selector: (state: { currentWorkspace: { slug: string } }) => unknown) => selector({ currentWorkspace: { slug: 'acme' } }),
}));
vi.mock('@/components/git/GitHubAppCreateButton', () => ({
  GitHubAppCreateButton: ({ returnTo }: { returnTo?: string }) => <button type="button" data-return-to={returnTo}>Create GitHub App</button>,
}));
vi.mock('@/lib/services/gitService', () => ({
  gitService: {
    getGitHubAppStatus: vi.fn(),
    listOrgIntegrations: vi.fn(),
    listRepositories: vi.fn(),
    getOrgIntegration: vi.fn(),
    getOrgGitHubInstallURL: vi.fn(),
  },
}));

import { toast } from 'sonner';
import { gitService } from '@/lib/services/gitService';
import type { GitHubAppStatus } from '@/lib/pmTypes';
import { OrgGitConnectionsTab } from '../OrgGitConnectionsTab';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

let root: Root | null = null;
let container: HTMLDivElement | null = null;

const configured: GitHubAppStatus = {
  configured: true,
  source: 'database',
  slug: 'helpin-acme',
  install_url: 'https://github.com/apps/helpin-acme/installations/new',
  webhook_configured: true,
  manifest_available: false,
  private: true,
  owner_login: 'acme',
  owner_type: 'Organization',
};

async function flush() {
  for (let i = 0; i < 4; i += 1) {
    await act(async () => { await new Promise((resolve) => setTimeout(resolve, 0)); });
  }
}

async function renderTab(status: GitHubAppStatus) {
  vi.mocked(gitService.getGitHubAppStatus).mockResolvedValue({ data: status, error: null, status: 200 } as never);
  vi.mocked(gitService.listOrgIntegrations).mockResolvedValue({ data: [], error: null, status: 200 } as never);
  vi.mocked(gitService.listRepositories).mockResolvedValue({ data: [], error: null, status: 200 } as never);
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
  await act(async () => {
    root!.render(
      <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
        <OrgGitConnectionsTab organizationId="org-1" workspaceId="ws-1" canManage />
      </QueryClientProvider>,
    );
  });
  await flush();
  return container;
}

afterEach(() => {
  act(() => root?.unmount());
  container?.remove();
  root = null;
  container = null;
  vi.clearAllMocks();
  window.history.replaceState(null, '', '/');
});

describe('OrgGitConnectionsTab GitHub flow', () => {
  it('shows the GitHub result it returned with once and strips it from the URL', async () => {
    window.history.replaceState(null, '', '/w/acme/settings/git-connections?github=connected&github_message=GitHub+App+connected+to+acme.&integration_id=gi-1');
    await renderTab(configured);

    expect(toast.success).toHaveBeenCalledTimes(1);
    expect(toast.success).toHaveBeenCalledWith('GitHub App connected to acme.');
    expect(window.location.search).toBe('');
  });

  it('still honors the older manifest flag', async () => {
    window.history.replaceState(null, '', '/w/acme/settings/git-connections?github_app_manifest=error&github_message=Try+again.');
    await renderTab(configured);

    expect(toast.error).toHaveBeenCalledWith('Try again.');
    expect(window.location.search).toBe('');
  });

  it('explains who can install a private App and starts installs that return here', async () => {
    vi.mocked(gitService.getOrgGitHubInstallURL).mockResolvedValue({ data: null, error: 'nope', status: 400 } as never);
    const view = await renderTab(configured);

    expect(view.querySelector('[data-testid="github-owner-note"]')?.textContent)
      .toBe('Private App owned by acme. Install it on acme to connect its repositories.');
    const connect = Array.from(view.querySelectorAll('button')).find((button) => button.textContent?.trim() === 'Connect GitHub');
    await act(async () => { connect!.click(); });
    expect(gitService.getOrgGitHubInstallURL).toHaveBeenCalledWith('org-1', 'ws-1', { forceInstall: false, returnTo: 'settings' });
  });

  it('shows GitHub and GitLab as quiet branded actions rather than dark buttons', async () => {
    const view = await renderTab(configured);
    const buttons = Array.from(view.querySelectorAll('button'));
    const github = buttons.find((button) => button.textContent?.trim() === 'Connect GitHub')!;
    const gitlab = buttons.find((button) => button.textContent?.trim() === 'Connect GitLab')!;

    for (const [button, provider] of [[github, 'github'], [gitlab, 'gitlab']] as const) {
      expect(button.dataset.gitProvider).toBe(provider);
      expect(button.dataset.variant).toBe('outline');
      expect(button.querySelector(`svg[data-git-provider-icon="${provider}"]`)).not.toBeNull();
    }
    expect(view.querySelectorAll('button[data-variant="default"]')).toHaveLength(0);
  });

  it('offers creation in the page body and shows the reachability problem instead when blocked', async () => {
    const missing: GitHubAppStatus = { configured: false, source: 'none', slug: '', install_url: '', webhook_configured: false, manifest_available: true };
    let view = await renderTab(missing);
    expect(view.querySelector('[data-testid="github-app-missing"] button')?.getAttribute('data-return-to')).toBe('settings');

    act(() => root?.unmount());
    container?.remove();
    const reason = 'GitHub must reach this server to deliver events. Set APP_BASE_URL to a public https address (currently http://10.0.0.5).';
    view = await renderTab({ ...missing, manifest_available: false, manifest_blocked_reason: reason });
    const notice = view.querySelector('[data-testid="github-app-missing"]');
    expect(notice?.textContent).toBe(reason);
    expect(notice?.querySelector('button')).toBeNull();
    // GitHub stays visible, disabled and pointing at the reason, so owners know it's supported.
    const github = Array.from(view.querySelectorAll('button')).find((button) => button.textContent?.trim() === 'Connect GitHub');
    expect(github?.disabled).toBe(true);
    expect(github?.getAttribute('aria-describedby')).toBe('github-app-missing-reason');
    expect(view.querySelector('#github-app-missing-reason')?.textContent).toBe(reason);
  });
});
