// @vitest-environment jsdom
import { act, type ButtonHTMLAttributes } from 'react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { Workspace } from '@/lib/types';

let currentWorkspace: Partial<Workspace> | null = null;
let workspaces: Partial<Workspace>[] = [];

vi.mock('@/hooks/useTitle', () => ({ useTitle: () => {} }));
vi.mock('@/stores/workspaceStore', () => ({
  useWorkspaceStore: (selector: (state: { currentWorkspace: Partial<Workspace> | null }) => unknown) => selector({ currentWorkspace }),
}));
vi.mock('@/hooks/queries', () => ({
  useWorkspaces: () => ({ data: workspaces, isPending: false }),
}));
vi.mock('@/lib/services/gitService', () => ({ gitService: { claimOrgGitHubInstallation: vi.fn() } }));
vi.mock('@/lib/githubReturn', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/lib/githubReturn')>()),
  assignBrowserLocation: vi.fn(),
}));
vi.mock('@/components/design-system/quiet', () => ({
  QuietPageHeader: ({ title, description }: { title: string; description?: string }) => <header><h1>{title}</h1>{description && <p>{description}</p>}</header>,
  QuietTextAction: (props: ButtonHTMLAttributes<HTMLButtonElement>) => <button type="button" {...props} />,
  QuietPrimaryAction: (props: ButtonHTMLAttributes<HTMLButtonElement>) => <button type="button" {...props} />,
  QuietDropdown: ({ selected, options, onSelect }: { selected: string[]; options: { value: string; label: string }[]; onSelect: (value: string) => void }) => (
    <select aria-label="Choose workspace" value={selected[0] ?? ''} onChange={(event) => onSelect(event.target.value)}>
      <option value="">Choose a workspace</option>
      {options.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
    </select>
  ),
}));

import { assignBrowserLocation, parseGitHubInstalledQuery } from '@/lib/githubReturn';
import { gitService } from '@/lib/services/gitService';
import { GitHubInstalledPage } from '../GitHubInstalledPage';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

let root: Root | null = null;
let container: HTMLDivElement | null = null;

const acme = { id: 'ws-1', name: 'Acme', slug: 'acme', organization_id: 'org-1' };
const beta = { id: 'ws-2', name: 'Beta', slug: 'beta', organization_id: 'org-2' };

async function settle() {
  for (let i = 0; i < 4; i += 1) {
    await act(async () => { await new Promise((resolve) => setTimeout(resolve, 0)); });
  }
}

async function renderPage(search: Record<string, unknown>) {
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
  await act(async () => {
    root!.render(
      <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })}>
        <GitHubInstalledPage query={parseGitHubInstalledQuery(search)} />
      </QueryClientProvider>,
    );
  });
  await settle();
  return container;
}

afterEach(() => {
  act(() => root?.unmount());
  container?.remove();
  root = null;
  container = null;
  currentWorkspace = null;
  workspaces = [];
  vi.clearAllMocks();
});

describe('GitHubInstalledPage', () => {
  it('claims an installation for the only workspace and returns to Git connections', async () => {
    workspaces = [acme];
    vi.mocked(gitService.claimOrgGitHubInstallation).mockResolvedValue({
      data: { integration_id: 'gi-1', account_login: 'acme', account_type: 'Organization', created: true },
      error: null,
    } as never);

    const view = await renderPage({ installation_id: 777, setup_action: 'install' });

    expect(gitService.claimOrgGitHubInstallation).toHaveBeenCalledTimes(1);
    expect(gitService.claimOrgGitHubInstallation).toHaveBeenCalledWith('org-1', '777', 'ws-1');
    expect(assignBrowserLocation).toHaveBeenCalledWith('/w/acme/settings/git-connections?github=connected&github_message=GitHub+App+connected+to+acme.');
    expect(view.querySelector('[role="status"]')?.textContent).toContain('GitHub App connected to acme.');
  });

  it('uses the current workspace and reports an update as a refresh', async () => {
    currentWorkspace = beta;
    workspaces = [acme, beta];
    vi.mocked(gitService.claimOrgGitHubInstallation).mockResolvedValue({
      data: { integration_id: 'gi-2', account_login: 'beta-org', account_type: 'Organization', created: false },
      error: null,
    } as never);

    const view = await renderPage({ installation_id: '888', setup_action: 'update' });

    expect(view.querySelector('h1')?.textContent).toBe('Update GitHub access');
    expect(gitService.claimOrgGitHubInstallation).toHaveBeenCalledWith('org-2', '888', 'ws-2');
    expect(assignBrowserLocation).toHaveBeenCalledWith(expect.stringContaining('/w/beta/settings/git-connections?github=connected'));
  });

  it('shows why a claim failed and offers a retry', async () => {
    workspaces = [acme];
    vi.mocked(gitService.claimOrgGitHubInstallation).mockResolvedValue({
      data: null,
      error: 'This GitHub installation is already connected to another Helpin organization',
      status: 409,
    } as never);

    const view = await renderPage({ installation_id: '777', setup_action: 'install' });

    expect(view.querySelector('[role="alert"]')?.textContent).toBe('This GitHub installation is already connected to another Helpin organization');
    expect(Array.from(view.querySelectorAll('button')).some((button) => button.textContent === 'Try again')).toBe(true);
    expect(view.querySelector('a')?.getAttribute('href')).toBe('/w/acme/settings/git-connections');
    expect(assignBrowserLocation).not.toHaveBeenCalled();
  });

  it('asks which workspace to use when there are several', async () => {
    workspaces = [acme, beta];
    vi.mocked(gitService.claimOrgGitHubInstallation).mockResolvedValue({
      data: { integration_id: 'gi-1', account_login: 'acme', account_type: 'Organization', created: true },
      error: null,
    } as never);
    const view = await renderPage({ installation_id: '777', setup_action: 'install' });

    expect(gitService.claimOrgGitHubInstallation).not.toHaveBeenCalled();
    const connect = Array.from(view.querySelectorAll('button')).find((button) => button.textContent === 'Connect GitHub')!;
    expect(connect.disabled).toBe(true);
    await act(async () => {
      const select = view.querySelector('select')!;
      select.value = 'ws-2';
      select.dispatchEvent(new Event('change', { bubbles: true }));
    });
    await act(async () => { connect.click(); });
    await settle();

    expect(gitService.claimOrgGitHubInstallation).toHaveBeenCalledWith('org-2', '777', 'ws-2');
  });

  it('shows results that could not return to a workspace page', async () => {
    workspaces = [acme];
    const view = await renderPage({ github: 'error', github_message: 'The GitHub App setup link is invalid or expired. Start again from Helpin.' });

    expect(view.querySelector('[role="alert"]')?.textContent).toBe('The GitHub App setup link is invalid or expired. Start again from Helpin.');
    expect(view.querySelector('a')?.getAttribute('href')).toBe('/w/acme/settings/git-connections');
    expect(gitService.claimOrgGitHubInstallation).not.toHaveBeenCalled();
  });
});
