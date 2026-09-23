// @vitest-environment jsdom

import { act, type ReactNode } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { CapabilitiesResponse } from '@/lib/capabilityTypes';

const state = vi.hoisted(() => ({
  navigate: vi.fn(),
  workspaces: [] as { id: string; slug: string }[],
  workspaceBySlug: undefined as { id: string; slug: string; name: string; website_url?: string; company_product_context?: string } | undefined,
  capabilities: undefined as CapabilitiesResponse | undefined,
  permissions: new Set<string>(['workspace.update']),
  sampleStatus: { loaded: false, counts: {}, modules: ['pm'] } as { loaded: boolean; counts: object; modules: string[] },
  loadSample: vi.fn(),
  create: vi.fn(),
  update: vi.fn(),
  generate: vi.fn(),
}));

vi.mock('@tanstack/react-router', () => ({ useNavigate: () => state.navigate }));
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
  usePermissions: () => ({ has: (permission: string) => state.permissions.has(permission) }),
  useOrganizations: () => ({ data: [{ id: 'org-1', name: 'Acme Org' }], isLoading: false }),
  useCreateOrganization: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));
vi.mock('@/hooks/queries/useSampleData', () => ({
  useSampleDataStatus: () => ({ data: state.sampleStatus }),
  useLoadSampleData: () => ({ mutateAsync: state.loadSample, isPending: false }),
}));
vi.mock('@/components/setup/SetupAIStep', () => ({
  SetupAIStep: ({ capability }: { capability: { status: string } }) => <div data-testid="setup-ai" data-status={capability.status} />,
}));
vi.mock('@/components/layout/PublicPageShell', () => ({
  PublicPageShell: ({ children, headerAction }: { children: ReactNode; headerAction?: ReactNode }) => (
    <div>{headerAction}<main>{children}</main></div>
  ),
}));
vi.mock('@/lib/services/workspacesService', () => ({
  workspacesService: {
    create: state.create,
    update: state.update,
    generateCompanyProductDescription: state.generate,
  },
}));
vi.mock('@/lib/services/inviteService', () => ({ inviteService: { send: vi.fn() } }));
vi.mock('@/lib/services/settingsService', () => ({ settingsService: {} }));
vi.mock('@/stores/authStore', () => ({
  useAuthStore: (selector: (value: unknown) => unknown) =>
    selector({ user: { email: 'ana@acme.com', full_name: 'Ana Lee' }, signOut: vi.fn() }),
}));
vi.mock('@/stores/organizationStore', () => ({
  useOrganizationStore: () => ({ currentOrganization: { id: 'org-1', name: 'Acme Org' }, setCurrentOrganization: vi.fn() }),
}));
vi.mock('@/hooks/useSetupGuideEnabled', () => ({ useSetupGuideEnabled: () => true }));
vi.mock('@/lib/analytics', () => ({ trackAnalyticsEvent: vi.fn() }));
vi.mock('sonner', () => ({ toast: { success: vi.fn(), warning: vi.fn(), error: vi.fn() } }));

import { TooltipProvider } from '@/components/ui/tooltip';
import { OnboardingFlow, type OnboardingFlowProps } from '../OnboardingFlow';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
// Radix checkboxes measure themselves.
globalThis.ResizeObserver ??= class {
  observe() {}
  unobserve() {}
  disconnect() {}
} as unknown as typeof ResizeObserver;

let container: HTMLDivElement | null = null;
let root: Root | null = null;

const acme = { id: 'ws-1', slug: 'acme', name: 'Acme' };

function caps(edition: CapabilitiesResponse['edition'], ai: string, email = 'ready'): CapabilitiesResponse {
  return {
    edition,
    capabilities: [
      { key: 'ai_chat', status: ai as never, detail: '', required: false },
      { key: 'email_outbound', status: email as never, detail: '', required: false },
    ],
  };
}

function render(props: OnboardingFlowProps) {
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
  act(() => root?.render(<TooltipProvider><OnboardingFlow {...props} /></TooltipProvider>));
}

function button(label: string) {
  return Array.from(container?.querySelectorAll('button') ?? []).find((element) => element.textContent?.trim() === label) as HTMLButtonElement | undefined;
}

function goalButton(label: string) {
  return Array.from(container?.querySelectorAll('button[aria-pressed]') ?? []).find((element) => element.textContent?.includes(label)) as HTMLButtonElement | undefined;
}

function typeInto(element: HTMLInputElement | HTMLTextAreaElement, value: string) {
  const prototype = element instanceof HTMLTextAreaElement ? HTMLTextAreaElement.prototype : HTMLInputElement.prototype;
  Object.getOwnPropertyDescriptor(prototype, 'value')?.set?.call(element, value);
  element.dispatchEvent(new Event('input', { bubbles: true }));
}

beforeEach(() => {
  state.workspaces = [];
  state.workspaceBySlug = undefined;
  state.capabilities = undefined;
  state.permissions = new Set(['workspace.update']);
  state.sampleStatus = { loaded: false, counts: {}, modules: ['pm'] };
});

afterEach(() => {
  act(() => root?.unmount());
  root = null;
  container?.remove();
  container = null;
  vi.clearAllMocks();
});

describe('OnboardingFlow', () => {
  it('creates the workspace with every selected goal, in selection order, and no website', async () => {
    state.create.mockResolvedValue({ data: { ...acme }, error: null });
    render({ step: 'workspace' });

    expect(container?.textContent).toContain('Select everything you want to set up');
    const order = ['Sales/CRM', 'Customer support', 'Internal docs', 'Help center docs', 'Plan and ship team projects'];
    for (const label of order) {
      act(() => goalButton(label)?.click());
    }
    expect(goalButton('Sales/CRM')?.getAttribute('aria-pressed')).toBe('true');
    expect(goalButton('Sales/CRM')?.textContent).toContain('priority 1');
    expect(goalButton('Plan and ship team projects')?.textContent).toContain('priority 5');
    expect(container?.textContent).toContain('5 selected');

    await act(async () => button('Create workspace')?.click());

    expect(state.create).toHaveBeenCalledWith(expect.objectContaining({
      name: 'Acme',
      organization_id: 'org-1',
      setup_goals: ['sales_crm', 'customer_support', 'internal_docs', 'help_center_docs', 'product_delivery'],
    }));
    expect(state.create.mock.calls[0][0]).not.toHaveProperty('website_url');
    expect(state.navigate).toHaveBeenCalledWith({ to: '/onboarding', search: { step: 'ai', workspace: 'acme' }, replace: true });
  });

  it('requires at least one goal before creating the workspace', async () => {
    render({ step: 'workspace' });
    await act(async () => button('Create workspace')?.click());
    expect(state.create).not.toHaveBeenCalled();
    expect(container?.querySelector('[role="alert"]')?.textContent).toBe('Select at least one thing to set up.');
  });

  it('shows Connect AI on Community when AI needs setup, and Skip continues to company context', () => {
    state.workspaceBySlug = acme;
    state.capabilities = caps('community', 'needs_setup');
    render({ step: 'ai', workspaceSlug: 'acme' });

    expect(container?.querySelector('[data-testid="setup-ai"]')?.getAttribute('data-status')).toBe('needs_setup');
    expect(container?.textContent).toContain('Settings → AI');
    act(() => button('Skip for now')?.click());
    expect(state.navigate).toHaveBeenCalledWith({ to: '/onboarding', search: { step: 'context', workspace: 'acme' } });
  });

  it('never shows Connect AI on Cloud or Enterprise', () => {
    state.workspaceBySlug = acme;
    state.capabilities = caps('enterprise', 'needs_setup');
    render({ step: 'ai', workspaceSlug: 'acme' });

    expect(container?.querySelector('[data-testid="setup-ai"]')).toBeNull();
    expect(container?.textContent).toContain('Tell Helpin about your company');
    expect(state.navigate).toHaveBeenCalledWith({ to: '/onboarding', search: { step: 'context', workspace: 'acme' }, replace: true });
  });

  it('skips Connect AI when the workspace already has working AI', () => {
    state.workspaceBySlug = acme;
    state.capabilities = caps('community', 'ready');
    render({ step: 'ai', workspaceSlug: 'acme' });

    expect(container?.querySelector('[data-testid="setup-ai"]')).toBeNull();
    expect(state.navigate).toHaveBeenCalledWith({ to: '/onboarding', search: { step: 'context', workspace: 'acme' }, replace: true });
  });

  it('shows a failed generation inline with the server sentence and keeps the text editable', async () => {
    state.workspaceBySlug = acme;
    state.capabilities = caps('community', 'ready');
    const sentence = 'We couldn’t read acme.com. Check the address, or write the description yourself.';
    state.generate.mockResolvedValue({ data: null, error: sentence, status: 422 });
    state.update.mockResolvedValue({ data: { ...acme }, error: null });
    render({ step: 'context', workspaceSlug: 'acme' });

    const website = container?.querySelector('input[type="url"]') as HTMLInputElement;
    expect(website.value).toBe('https://acme.com');
    await act(async () => button('Generate from website')?.click());

    expect(state.generate).toHaveBeenCalledWith(expect.objectContaining({ website_url: 'https://acme.com', workspace_id: 'ws-1' }));
    expect(container?.querySelector('[role="status"]')?.textContent).toBe(sentence);
    const textarea = container?.querySelector('textarea') as HTMLTextAreaElement;
    expect(textarea.disabled).toBe(false);

    act(() => typeInto(textarea, 'Acme makes outdoor gear.'));
    await act(async () => button('Save and continue')?.click());
    expect(state.update).toHaveBeenCalledWith('ws-1', { company_product_context: 'Acme makes outdoor gear.', website_url: 'https://acme.com' });
    expect(state.navigate).toHaveBeenCalledWith({ to: '/onboarding', search: { step: 'teams', workspace: 'acme' } });
  });

  it('disables generation without AI and lets people skip company context', () => {
    state.workspaceBySlug = acme;
    state.capabilities = caps('community', 'needs_setup');
    render({ step: 'context', workspaceSlug: 'acme' });

    expect(button('Generate from website')?.disabled).toBe(true);
    act(() => button('Skip')?.click());
    expect(state.update).not.toHaveBeenCalled();
    expect(state.navigate).toHaveBeenCalledWith({ to: '/onboarding', search: { step: 'teams', workspace: 'acme' } });
  });

  it('lets people skip teams', () => {
    state.workspaceBySlug = acme;
    state.capabilities = caps('community', 'ready');
    render({ step: 'teams', workspaceSlug: 'acme' });

    act(() => button('Skip')?.click());
    expect(state.navigate).toHaveBeenCalledWith({ to: '/onboarding', search: { step: 'invite', workspace: 'acme' } });
  });

  it('explains that invitations need email when outbound email is not set up', () => {
    state.workspaceBySlug = acme;
    state.capabilities = caps('community', 'ready', 'needs_setup');
    render({ step: 'invite', workspaceSlug: 'acme' });

    expect(container?.textContent).toContain('Email isn’t set up on this server, so invitations can’t be sent yet. You can invite people later from Settings → Members.');
    expect(container?.querySelector('input[aria-label="Email addresses"]')).toBeNull();
    act(() => button('Continue')?.click());
    expect(state.navigate).toHaveBeenCalledWith({ to: '/onboarding', search: { step: 'finish', workspace: 'acme' } });
  });

  it('shows the invitation form when email works', () => {
    state.workspaceBySlug = acme;
    state.capabilities = caps('enterprise', 'ready', 'ready');
    render({ step: 'invite', workspaceSlug: 'acme' });

    expect(container?.querySelector('input[aria-label="Email addresses"]')).not.toBeNull();
    expect(button('Skip')).toBeDefined();
  });

  it('loads sample data from the finish step, then opens the workspace', async () => {
    state.workspaceBySlug = acme;
    state.capabilities = caps('community', 'ready');
    state.loadSample.mockResolvedValue({ loaded: true, counts: {}, modules: ['pm'] });
    render({ step: 'finish', workspaceSlug: 'acme' });

    expect(container?.textContent).toContain('You’re set');
    await act(async () => button('Explore with sample data')?.click());
    expect(state.loadSample).toHaveBeenCalled();
    expect(state.navigate).toHaveBeenCalledWith({ to: '/w/acme/setup' });
  });

  it('offers sample data only to people who can update the workspace', () => {
    state.workspaceBySlug = acme;
    state.capabilities = caps('community', 'ready');
    state.permissions = new Set();
    render({ step: 'finish', workspaceSlug: 'acme' });

    expect(button('Explore with sample data')).toBeUndefined();
    act(() => button('Go to your workspace')?.click());
    expect(state.navigate).toHaveBeenCalledWith({ to: '/w/acme/setup' });
  });

  it('resumes past the workspace form once the workspace exists, so it is never created twice', () => {
    state.workspaceBySlug = acme;
    state.capabilities = caps('community', 'ready');
    render({ step: 'workspace', workspaceSlug: 'acme' });

    expect(button('Create workspace')).toBeUndefined();
    expect(container?.textContent).toContain('Tell Helpin about your company');
    expect(state.navigate).toHaveBeenCalledWith({ to: '/onboarding', search: { step: 'context', workspace: 'acme' }, replace: true });
  });

  it('returns to the workspace step when the workspace in the URL cannot be found', () => {
    render({ step: 'teams', workspaceSlug: 'missing' });

    expect(button('Create workspace')).toBeDefined();
    expect(state.navigate).toHaveBeenCalledWith({ to: '/onboarding', search: { step: 'workspace', workspace: undefined }, replace: true });
  });
});
