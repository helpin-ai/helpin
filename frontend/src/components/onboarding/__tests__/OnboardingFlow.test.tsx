// @vitest-environment jsdom

import { act, type ReactNode } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { CapabilitiesResponse } from '@/lib/capabilityTypes';

const state = vi.hoisted(() => ({
  navigate: vi.fn(),
  workspaces: [] as { id: string; slug: string }[],
  workspaceBySlug: undefined as { id: string; slug: string; name: string } | undefined,
  capabilities: undefined as CapabilitiesResponse | undefined,
  create: vi.fn(),
  start: vi.fn(),
}));

vi.mock('@tanstack/react-router', () => ({ useNavigate: () => state.navigate, Link: ({ children }: { children: ReactNode }) => <a>{children}</a> }));
vi.mock('@tanstack/react-query', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@tanstack/react-query')>()),
  useQueryClient: () => ({ setQueryData: vi.fn(), invalidateQueries: vi.fn() }),
}));
vi.mock('@/hooks/queries', () => ({
  useWorkspaces: () => ({ data: state.workspaces, isLoading: false }),
  useWorkspaceBySlug: (slug: string) => ({
    data: slug ? state.workspaceBySlug : undefined,
    isLoading: false,
    isError: Boolean(slug) && !state.workspaceBySlug,
  }),
  useWorkspaceCapabilities: (id?: string) => ({
    data: id ? state.capabilities : undefined,
    isSuccess: Boolean(id && state.capabilities),
    isError: false,
  }),
  useWorkspaceAccess: () => ({ data: { permissions: [] } }),
  usePermissions: () => ({ has: () => true }),
  useOrganizations: () => ({ data: [{ id: 'org-1', name: 'Acme Org' }], isLoading: false }),
  useCreateOrganization: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));
vi.mock('../ConnectAIStep', () => ({ ConnectAIStep: () => <div data-testid="setup-ai" /> }));
vi.mock('../SupportSetupProgress', () => ({ SupportSetupProgress: () => <div data-testid="setup-progress" /> }));
vi.mock('@/components/layout/PublicPageShell', () => ({
  PublicPageShell: ({ children, headerAction }: { children: ReactNode; headerAction?: ReactNode }) => (
    <div>{headerAction}<main>{children}</main></div>
  ),
}));
vi.mock('@/lib/services/workspacesService', () => ({ workspacesService: { create: state.create } }));
vi.mock('@/lib/services/supportQuickstartService', () => ({ supportQuickstartService: { start: state.start } }));
vi.mock('@/stores/authStore', () => ({
  useAuthStore: (selector: (value: unknown) => unknown) =>
    selector({ user: { email: 'ana@acme.com', full_name: 'Ana Lee' }, signOut: vi.fn() }),
}));
vi.mock('@/stores/organizationStore', () => ({
  useOrganizationStore: () => ({ currentOrganization: { id: 'org-1', name: 'Acme Org' }, setCurrentOrganization: vi.fn() }),
}));

import { readPendingSupportSetup, savePendingSupportSetup } from '@/lib/pendingSupportSetup';
import { OnboardingFlow, type OnboardingFlowProps } from '../OnboardingFlow';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

let container: HTMLDivElement | null = null;
let root: Root | null = null;

const acme = { id: 'ws-1', slug: 'acme', name: 'Acme' };
const detection = {
  website_url: 'https://acme.com',
  company_name: 'Acme',
  website_readable: true,
  help_center: { url: 'https://help.acme.com/en', platform: 'intercom' as const },
};

function render(props: OnboardingFlowProps) {
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
  act(() => root?.render(<OnboardingFlow {...props} />));
}

function button(label: string) {
  return Array.from(container?.querySelectorAll('button') ?? []).find((element) => element.textContent?.trim() === label) as HTMLButtonElement | undefined;
}

async function flush() {
  await act(async () => { await Promise.resolve(); });
}

beforeEach(() => {
  state.navigate.mockReset();
  state.workspaces = [];
  state.workspaceBySlug = undefined;
  state.capabilities = undefined;
  state.create.mockReset().mockResolvedValue({ data: acme, error: null });
  localStorage.clear();
  state.start.mockReset().mockResolvedValue({ data: { started: true }, error: null });
});

afterEach(() => {
  act(() => root?.unmount());
  container?.remove();
  container = null;
  root = null;
});

describe('OnboardingFlow', () => {
  it('sends people without saved choices to /start with their company website', () => {
    render({ step: 'website' });

    expect(state.navigate).toHaveBeenCalledWith({ to: '/start', search: { website: 'acme.com' }, replace: true });
    expect(state.create).not.toHaveBeenCalled();
  });

  it('creates the workspace and starts setup from the choices saved on /start', async () => {
    savePendingSupportSetup({ detection, companyName: 'Acme Inc', helpCenterUrl: 'https://help.acme.com/en', aiMode: 'draft' });
    render({ step: 'website' });
    await flush();
    await flush();

    expect(state.create).toHaveBeenCalledWith(expect.objectContaining({ name: 'Acme Inc', organization_id: 'org-1', website_url: 'https://acme.com', setup_goals: ['customer_support'] }));
    expect(state.start).toHaveBeenCalledWith('ws-1', { website_url: 'https://acme.com', company_name: 'Acme Inc', help_center_url: 'https://help.acme.com/en', ai_mode: 'draft' });
    expect(state.navigate).toHaveBeenCalledWith({ to: '/onboarding', search: { step: undefined, workspace: 'acme' }, replace: true });
    expect(readPendingSupportSetup()).toBeNull();
  });

  it('shows the form to retry only the setup when the saved setup fails', async () => {
    state.start.mockResolvedValueOnce({ data: null, error: 'Support setup didn’t finish. Try again.' });
    savePendingSupportSetup({ detection, companyName: 'Acme', aiMode: 'answer' });
    render({ step: 'website' });
    await flush();
    await flush();

    expect(container?.textContent).toContain('Support setup didn’t finish');
    await act(async () => { button('Try again')?.click(); });
    await flush();

    expect(state.create).toHaveBeenCalledTimes(1);
    expect(state.start).toHaveBeenCalledTimes(2);
  });

  it('shows setup progress once the workspace exists', () => {
    state.workspaceBySlug = acme;
    state.capabilities = { edition: 'cloud', capabilities: [{ key: 'ai_chat', status: 'ready' as never, detail: '', required: false }] };
    render({ workspaceSlug: 'acme' });

    expect(container?.querySelector('[data-testid="setup-progress"]')).not.toBeNull();
    expect(container?.textContent).toContain('Setting up Acme');
    expect(state.navigate).toHaveBeenCalledWith({ to: '/onboarding', search: { step: 'setup', workspace: 'acme' }, replace: true });
  });

  it('asks Community workspaces without AI to connect it first', () => {
    state.workspaceBySlug = acme;
    state.capabilities = { edition: 'community', capabilities: [{ key: 'ai_chat', status: 'needs_setup' as never, detail: '', required: false }] };
    render({ workspaceSlug: 'acme' });

    expect(container?.querySelector('[data-testid="setup-ai"]')).not.toBeNull();
  });
});
