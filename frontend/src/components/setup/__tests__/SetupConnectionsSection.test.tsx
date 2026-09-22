// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { Capability } from '@/lib/capabilityTypes';
import { api } from '@/lib/api';
import type { SetupGoalKey } from '@/lib/setupTypes';
import { SetupConnectionsSection } from '../SetupConnectionsSection';
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

function serve(capabilities: Capability[] | 'error') {
  vi.mocked(api.get).mockImplementation(async (path: string) => {
    if (path === '/workspaces/ws-1/capabilities') {
      return capabilities === 'error'
        ? { data: null, error: 'boom', status: 500 } as never
        : { data: { edition: 'community', capabilities }, error: null, status: 200 } as never;
    }
    if (path.startsWith('/ai-connections/')) {
      return { data: { enabled: true, connections: [], models: [] }, error: null, status: 200 } as never;
    }
    if (path === '/workspaces/ws-1/github/app-status') {
      return { data: { configured: true, source: 'env', slug: 'helpin', install_url: 'https://github.com/apps/helpin/installations/new', webhook_configured: true, manifest_available: false }, error: null, status: 200 } as never;
    }
    return { data: null, error: 'unexpected', status: 404 } as never;
  });
}

async function renderSection(options: { goals?: SetupGoalKey[]; canManage?: boolean } = {}) {
  rendered = await renderWithQuery(
    <SetupConnectionsSection workspaceId="ws-1" slug="acme" goals={options.goals ?? ['customer_support']} canManage={options.canManage ?? true} isOwner />,
  );
  return rendered.container;
}

function row(container: HTMLElement, key: string) {
  return container.querySelector(`[data-capability="${key}"]`) as HTMLElement;
}

describe('SetupConnectionsSection', () => {
  it('names every status in words and never presents an unverified check as ready', async () => {
    serve([
      capability({ key: 'email_outbound', status: 'ready', detail: 'The last test email was accepted for delivery.', checked_at: new Date().toISOString() }),
      capability({ key: 'ai_embeddings', status: 'unable_to_verify', detail: 'Confirmed once knowledge is indexed.' }),
      capability({ key: 'support_widget', status: 'needs_setup', action: { kind: 'open_settings', label: 'Install the chat widget', path: 'settings/chat-general' } }),
      capability({ key: 'github', status: 'unavailable', detail: 'Projects and Agents are not enabled on this server.' }),
    ]);
    const container = await renderSection();

    expect(row(container, 'email_outbound').textContent).toContain('Ready');
    expect(row(container, 'email_outbound').textContent).toMatch(/Checked .* ago/);
    const unverified = row(container, 'ai_embeddings').textContent ?? '';
    expect(unverified).toContain('Couldn’t verify');
    expect(unverified).not.toContain('Ready');
    expect(row(container, 'support_widget').textContent).toContain('Needs setup');
    expect(row(container, 'github').textContent).toContain('Not available');
  });

  it('groups unavailable items last and puts required services that are not ready first', async () => {
    serve([
      capability({ key: 'github', status: 'unavailable' }),
      capability({ key: 'support_widget', status: 'needs_setup' }),
      capability({ key: 'object_storage', status: 'needs_setup', required: true, action: { kind: 'server_config', label: 'Configure S3-compatible storage (AWS_S3_BUCKET_NAME and credentials)' } }),
      capability({ key: 'workers', status: 'ready', required: true }),
    ]);
    const container = await renderSection();
    const order = Array.from(container.querySelectorAll('[data-capability]')).map((element) => element.getAttribute('data-capability'));

    expect(order[0]).toBe('object_storage');
    expect(order.at(-1)).toBe('github');
    expect(row(container, 'object_storage').textContent).toContain('Required');
    expect(row(container, 'workers').textContent).not.toContain('Required');
    expect(container.textContent).toContain('A required service needs attention');
    expect(container.textContent).toContain('Not available on this server');
    expect(row(container, 'github').className).toContain('opacity-70');
  });

  it('shows what the chosen goals need first and the rest under other connections', async () => {
    serve([
      capability({ key: 'github', status: 'needs_setup' }),
      capability({ key: 'support_widget', status: 'needs_setup' }),
    ]);
    const container = await renderSection({ goals: ['customer_support'] });
    const order = Array.from(container.querySelectorAll('[data-capability]')).map((element) => element.getAttribute('data-capability'));

    expect(order).toEqual(['support_widget', 'github']);
    expect(row(container, 'support_widget').textContent).toContain('Needed for Scale customer support');
    expect(container.textContent).toContain('Other connections');
  });

  it('renders each action kind as a link, copyable guidance or an inline button', async () => {
    serve([
      capability({ key: 'support_email_inbound', action: { kind: 'open_settings', label: 'Add a support email address', path: 'settings/inboxes-routing' } }),
      capability({ key: 'workers', required: true, action: { kind: 'server_config', label: 'Start the worker service' } }),
      capability({ key: 'email_outbound', status: 'unable_to_verify', action: { kind: 'send_test_email', label: 'Send a test email' } }),
      capability({ key: 'ai_chat', status: 'unable_to_verify', action: { kind: 'test_ai_connection', label: 'Test the connection', path: 'settings/ai', connection_id: 'conn-1' } }),
    ]);
    const container = await renderSection();

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

  it('shows status but hides admin actions from members without settings access', async () => {
    serve([
      capability({ key: 'email_outbound', status: 'unable_to_verify', action: { kind: 'send_test_email', label: 'Send a test email' } }),
      capability({ key: 'ai_chat', status: 'unable_to_verify', action: { kind: 'test_ai_connection', label: 'Test the connection', connection_id: 'conn-1' } }),
      capability({ key: 'support_widget', action: { kind: 'open_settings', label: 'Install the chat widget', path: 'settings/chat-general' } }),
    ]);
    const container = await renderSection({ canManage: false });

    expect(button(container, 'Send test email')).toBeUndefined();
    expect(button(container, 'Test connection')).toBeUndefined();
    expect(container.querySelector('a')).toBeNull();
    expect(row(container, 'email_outbound').textContent).toContain('A workspace admin can finish this step.');
    expect(row(container, 'email_outbound').textContent).toContain('Couldn’t verify');
  });

  it('offers a retry when capabilities cannot be loaded', async () => {
    serve('error');
    const container = await renderSection();

    expect(container.textContent).toContain('We couldn’t check this workspace’s connections.');
    expect(button(container, 'Check again')).toBeDefined();
  });
});
