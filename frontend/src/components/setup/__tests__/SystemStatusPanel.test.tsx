// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { Capability } from '@/lib/capabilityTypes';
import { api } from '@/lib/api';
import { SystemStatusPanel } from '../SystemStatusPanel';
import { button, capability, renderWithQuery, type Rendered } from './setupTestUtils';

vi.mock('@/lib/api', () => ({ api: { get: vi.fn(), post: vi.fn() } }));
vi.mock('@tanstack/react-router', () => ({
  Link: ({ to, children, className }: { to: string; children: React.ReactNode; className?: string }) => <a href={to} className={className}>{children}</a>,
}));
vi.mock('@/components/git/GitHubAppCreateButton', () => ({
  GitHubAppCreateButton: () => <button type="button">Create GitHub App</button>,
}));

let rendered: Rendered | undefined;
afterEach(async () => {
  await rendered?.unmount();
  rendered = undefined;
  vi.mocked(api.get).mockReset();
});

type AppStatus = { configured: boolean; manifest_available: boolean; install_url?: string };

function serve(capabilities: Capability[] | 'error', options: { edition?: 'community' | 'enterprise'; appStatus?: AppStatus } = {}) {
  const appStatus = options.appStatus ?? { configured: true, manifest_available: false, install_url: 'https://github.com/apps/helpin/installations/new' };
  vi.mocked(api.get).mockImplementation(async (path: string) => {
    if (path === '/workspaces/ws-1/capabilities') {
      return capabilities === 'error'
        ? { data: null, error: 'boom', status: 500 } as never
        : { data: { edition: options.edition ?? 'community', capabilities }, error: null, status: 200 } as never;
    }
    if (path.startsWith('/ai-connections/')) {
      return { data: { enabled: true, connections: [], models: [] }, error: null, status: 200 } as never;
    }
    if (path === '/workspaces/ws-1/github/app-status') {
      return { data: { source: 'env', slug: 'helpin', webhook_configured: true, ...appStatus }, error: null, status: 200 } as never;
    }
    return { data: null, error: 'unexpected', status: 404 } as never;
  });
}

async function renderPanel(options: { canManage?: boolean; isOwner?: boolean } = {}) {
  rendered = await renderWithQuery(
    <SystemStatusPanel workspaceId="ws-1" slug="acme" canManage={options.canManage ?? true} isOwner={options.isOwner ?? true} />,
  );
  return rendered.container;
}

function row(container: HTMLElement, key: string) {
  return container.querySelector(`[data-capability="${key}"]`) as HTMLElement;
}

function order(container: HTMLElement) {
  return Array.from(container.querySelectorAll('[data-capability]')).map((element) => element.getAttribute('data-capability'));
}

describe('SystemStatusPanel', () => {
  it('names every status in words and never presents an unverified check as ready', async () => {
    serve([
      capability({ key: 'email_outbound', status: 'ready', detail: 'The last test email was accepted for delivery.', checked_at: new Date().toISOString() }),
      capability({ key: 'ai_embeddings', status: 'unable_to_verify', detail: 'Confirmed once knowledge is indexed.' }),
      capability({ key: 'support_email_inbound', status: 'needs_setup', action: { kind: 'open_settings', label: 'Add a support email address', path: 'settings/inboxes-routing' } }),
      capability({ key: 'github', status: 'unavailable', detail: 'Projects and Agents are not enabled on this server.' }),
    ]);
    const container = await renderPanel();

    expect(row(container, 'email_outbound').textContent).toContain('Application email');
    expect(row(container, 'email_outbound').textContent).toContain('Ready');
    expect(row(container, 'email_outbound').textContent).toMatch(/Checked .* ago/);
    const unverified = row(container, 'ai_embeddings').textContent ?? '';
    expect(unverified).toContain('Knowledge search');
    expect(unverified).toContain('Couldn’t verify');
    expect(unverified).not.toContain('Ready');
    expect(row(container, 'support_email_inbound').textContent).toContain('Needs setup');
    expect(row(container, 'github').textContent).toContain('Not available');
  });

  it('puts required services that are not ready first, prominently, and unavailable ones last', async () => {
    serve([
      capability({ key: 'github', status: 'unavailable' }),
      capability({ key: 'ai_chat', status: 'ready' }),
      capability({ key: 'object_storage', status: 'needs_setup', required: true, action: { kind: 'server_config', label: 'Configure S3-compatible storage (AWS_S3_BUCKET_NAME and credentials)' } }),
      capability({ key: 'workers', status: 'ready', required: true }),
    ]);
    const container = await renderPanel();

    expect(order(container)[0]).toBe('object_storage');
    expect(order(container).at(-1)).toBe('github');
    expect(row(container, 'object_storage').textContent).toContain('Required');
    expect(row(container, 'workers').textContent).not.toContain('Required');
    expect(container.querySelector('[role="note"]')?.textContent).toContain('A required service needs attention');
    expect(container.textContent).toContain('Not available on this server');
    expect(row(container, 'github').className).toContain('opacity-70');
  });

  it('lists server services only, leaving workspace adoption steps to the Setup guide', async () => {
    serve([
      capability({ key: 'support_widget', status: 'needs_setup' }),
      capability({ key: 'github', status: 'needs_setup', action: { kind: 'open_settings', label: 'Select repositories', path: 'settings/repositories' } }),
      capability({ key: 'email_outbound', status: 'unable_to_verify', action: { kind: 'send_test_email', label: 'Send a test email' } }),
    ]);
    const container = await renderPanel();

    expect(row(container, 'support_widget')).toBeNull();
    expect(order(container)).toEqual(['github', 'email_outbound']);
    expect(container.textContent).toContain('helpin doctor');
  });

  it('renders each action kind as a link, copyable guidance or an inline button', async () => {
    serve([
      capability({ key: 'support_email_inbound', action: { kind: 'open_settings', label: 'Add a support email address', path: 'settings/inboxes-routing' } }),
      capability({ key: 'workers', required: true, action: { kind: 'server_config', label: 'Start the worker service' } }),
      capability({ key: 'email_outbound', status: 'unable_to_verify', action: { kind: 'send_test_email', label: 'Send a test email' } }),
      capability({ key: 'ai_chat', status: 'unable_to_verify', action: { kind: 'test_ai_connection', label: 'Test the connection', path: 'settings/ai', connection_id: 'conn-1' } }),
    ]);
    const container = await renderPanel();

    const link = row(container, 'support_email_inbound').querySelector('a');
    expect(link?.getAttribute('href')).toBe('/w/acme/settings/inboxes-routing');
    expect(link?.textContent).toContain('Add a support email address');

    const workers = row(container, 'workers');
    expect(workers.querySelector('code')?.textContent).toBe('Start the worker service');
    expect(button(workers, 'Copy server configuration')).toBeDefined();
    expect(workers.querySelector('a')).toBeNull();

    expect(button(row(container, 'email_outbound'), 'Send test email')).toBeDefined();
    expect(button(row(container, 'ai_chat'), 'Test connection')).toBeDefined();
  });

  it('offers to connect AI inline when no provider is set up', async () => {
    serve([capability({ key: 'ai_chat', status: 'needs_setup', action: { kind: 'open_settings', label: 'Connect an AI provider', path: 'settings/ai' } })]);
    const container = await renderPanel();

    expect(button(row(container, 'ai_chat'), 'Connect and test')).toBeDefined();
  });

  it('lets the owner create the GitHub App and gives other admins server guidance', async () => {
    const github = capability({ key: 'github', status: 'needs_setup', action: { kind: 'open_settings', label: 'Set up GitHub', path: 'settings/git-connections' } });
    serve([github], { appStatus: { configured: false, manifest_available: true } });
    let container = await renderPanel({ isOwner: true });
    expect(button(row(container, 'github'), 'Create GitHub App')).toBeDefined();
    await rendered?.unmount();

    serve([github], { appStatus: { configured: false, manifest_available: true } });
    container = await renderPanel({ isOwner: false });
    expect(button(row(container, 'github'), 'Create GitHub App')).toBeUndefined();
    expect(row(container, 'github').textContent).toContain('The workspace owner can create the GitHub App');
    await rendered?.unmount();

    serve([github], { appStatus: { configured: false, manifest_available: false } });
    container = await renderPanel({ isOwner: true });
    expect(row(container, 'github').querySelector('code')?.textContent).toContain('GITHUB_APP_ID');
  });

  it('shows status but hides admin actions without settings access', async () => {
    serve([
      capability({ key: 'email_outbound', status: 'unable_to_verify', action: { kind: 'send_test_email', label: 'Send a test email' } }),
      capability({ key: 'ai_chat', status: 'unable_to_verify', action: { kind: 'test_ai_connection', label: 'Test the connection', connection_id: 'conn-1' } }),
    ]);
    const container = await renderPanel({ canManage: false });

    expect(button(container, 'Send test email')).toBeUndefined();
    expect(button(container, 'Test connection')).toBeUndefined();
    expect(row(container, 'email_outbound').textContent).toContain('A workspace admin can finish this step.');
    expect(row(container, 'email_outbound').textContent).toContain('Couldn’t verify');
  });

  it('shows nothing to fix on editions where the platform manages services', async () => {
    serve([capability({ key: 'object_storage', status: 'needs_setup', required: true })], { edition: 'enterprise' });
    const container = await renderPanel();

    expect(container.querySelector('[data-capability]')).toBeNull();
    expect(container.textContent).toContain('managed by the platform');
  });

  it('offers a retry when capabilities cannot be loaded', async () => {
    serve('error');
    const container = await renderPanel();

    expect(container.textContent).toContain('We couldn’t check this server.');
    expect(button(container, 'Check again')).toBeDefined();
  });
});
