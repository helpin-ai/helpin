// @vitest-environment jsdom

import React, { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { systemStatusEnabled } from '@edition/config';
import type { CapabilitiesResponse, Capability } from '@/lib/capabilityTypes';
import type { SetupJourney, SetupTask, SetupView } from '@/lib/setupTypes';

const navigate = vi.fn();
const mutate = vi.fn();
let workspace = { id: 'workspace-1', slug: 'acme' };
let setupView: SetupView;
let capabilities: CapabilitiesResponse | undefined;
let isAdmin = true;
const queryClient = new QueryClient();

vi.mock('@tanstack/react-router', () => ({
  useNavigate: () => navigate,
  Link: ({ to, children, className }: { to: string; children: React.ReactNode; className?: string }) => <a href={to} className={className}>{children}</a>,
}));
vi.mock('sonner', () => ({ toast: { info: vi.fn(), error: vi.fn() } }));
vi.mock('@/lib/analytics', () => ({ trackAnalyticsEvent: vi.fn() }));
vi.mock('@/lib/services/setupService', () => ({ setupService: { startRecommendation: vi.fn() } }));
vi.mock('@/components/setup/SampleDataButton', () => ({
  SampleDataCard: () => <section data-testid="sample-data" />,
}));
vi.mock('@/components/setup/SetupAIStep', () => ({
  SetupAIStep: () => <div data-testid="ai-step" />,
}));
vi.mock('@/components/setup/SetupGitHubStep', () => ({
  SetupGitHubStep: () => <div data-testid="github-step" />,
}));
vi.mock('@/stores/workspaceStore', () => ({
  useWorkspaceStore: (selector: (state: { currentWorkspace: typeof workspace }) => unknown) => selector({ currentWorkspace: workspace }),
}));
vi.mock('@/hooks/queries', () => ({
  useSetup: () => ({ data: setupView, isLoading: false, isError: false, refetch: vi.fn() }),
  useUpdateSetupGoals: () => ({ mutate, isPending: false }),
  useUpdateSetupPreference: () => ({ mutate, isPending: false }),
  useWorkspaceAccess: () => ({ data: {} }),
  usePermissions: () => ({ has: () => isAdmin }),
  useWorkspaceCapabilities: () => ({ data: capabilities }),
}));

import { SetupSuccessPage } from '../SetupSuccessPage';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

let container: HTMLDivElement | null = null;
let root: Root | null = null;

afterEach(() => {
  act(() => root?.unmount());
  root = null;
  container?.remove();
  container = null;
  workspace = { id: 'workspace-1', slug: 'acme' };
  capabilities = undefined;
  isAdmin = true;
	mutate.mockReset();
});

function journey(key: string, title: string, taskTitle: string): SetupJourney {
  return {
    key,
    title,
    description: `${title} description`,
    accent: 'indigo',
    scope: 'active',
    maturity: 'ready',
    completed_count: 1,
    total_count: 2,
    tasks: [{ key: `${key}.task`, title: taskTitle, description: '', stage: 'Prepare', status: 'available', shared: true, core: true }],
  };
}

function view(journeys: SetupJourney[]): SetupView {
  return {
    goals: ['product_delivery'],
    journeys,
    preference: { id: 'preference-1', workspace_id: workspace.id, user_id: 'user-1', sidebar_dismissed: false },
    completed_count: 1,
    total_count: 4,
    placeholder_goals: [],
  };
}

function renderPage() {
  if (!container) {
    container = document.createElement('div');
    document.body.appendChild(container);
    root = createRoot(container);
  }
  act(() => root?.render(<QueryClientProvider client={queryClient}><SetupSuccessPage /></QueryClientProvider>));
}

function journeyTrigger(title: string) {
  return Array.from(container?.querySelectorAll('button') ?? []).find((button) => button.textContent?.includes(title));
}

describe('SetupSuccessPage journey sections', () => {
	it('labels the finite denominator as core setup progress', () => {
		setupView = view([journey('foundation', 'Workspace essentials', 'Add company details')]);
		renderPage();

		expect(container?.textContent).toContain('1 of 4 core steps verified');
		expect(container?.textContent).toContain('1/2 core complete');
	});

	it('lets workspace managers update a focused set of goals', () => {
		setupView = view([journey('foundation', 'Workspace essentials', 'Add company details')]);
		renderPage();

		const editGoals = Array.from(container?.querySelectorAll('button') ?? []).find((button) => button.textContent?.includes('Edit goals'));
		act(() => editGoals?.click());
		const supportLabel = Array.from(document.querySelectorAll('label')).find((label) => label.textContent?.includes('Scale customer support'));
		const supportCheckbox = supportLabel?.querySelector('button[role="checkbox"]');
		act(() => (supportCheckbox as HTMLButtonElement | null)?.click());
		const save = Array.from(document.querySelectorAll('button')).find((button) => button.textContent?.includes('Save goals'));
		act(() => save?.click());

		expect(mutate).toHaveBeenCalledWith(
			['product_delivery', 'customer_support'],
			expect.objectContaining({ onSuccess: expect.any(Function) }),
		);
	});

  it('opens only the first journey initially and toggles sections independently', () => {
    setupView = view([
      journey('foundation', 'Workspace essentials', 'Add company details'),
      journey('customer_support', 'Scale customer support', 'Connect support email'),
      journey('automation_mastery', 'Automate repeatable work', 'Choose work to automate'),
    ]);
    renderPage();

    const first = journeyTrigger('Workspace essentials');
    const second = journeyTrigger('Scale customer support');
    const third = journeyTrigger('Automate repeatable work');

    expect(first?.getAttribute('aria-expanded')).toBe('true');
    expect(second?.getAttribute('aria-expanded')).toBe('false');
    expect(third?.getAttribute('aria-expanded')).toBe('false');
    expect(container?.textContent).toContain('1/2 core complete');
    expect(document.getElementById('setup-journey-tasks-foundation')?.hidden).toBe(false);
    expect(document.getElementById('setup-journey-tasks-customer_support')?.hidden).toBe(true);

    act(() => second?.click());
    expect(first?.getAttribute('aria-expanded')).toBe('true');
    expect(second?.getAttribute('aria-expanded')).toBe('true');

    act(() => first?.click());
    expect(first?.getAttribute('aria-expanded')).toBe('false');
    expect(second?.getAttribute('aria-expanded')).toBe('true');
  });

  it('preserves keyed toggles on refresh and resets them for another workspace', () => {
    const foundation = journey('foundation', 'Workspace essentials', 'Add company details');
    const support = journey('customer_support', 'Scale customer support', 'Connect support email');
    setupView = view([foundation, support]);
    renderPage();

    act(() => journeyTrigger('Workspace essentials')?.click());
    act(() => journeyTrigger('Scale customer support')?.click());

    const automation = journey('automation_mastery', 'Automate repeatable work', 'Choose work to automate');
    setupView = view([support, foundation, automation]);
    renderPage();

    expect(journeyTrigger('Workspace essentials')?.getAttribute('aria-expanded')).toBe('false');
    expect(journeyTrigger('Scale customer support')?.getAttribute('aria-expanded')).toBe('true');
    expect(journeyTrigger('Automate repeatable work')?.getAttribute('aria-expanded')).toBe('false');

    workspace = { id: 'workspace-2', slug: 'other' };
    setupView = view([automation, support]);
    renderPage();

    expect(journeyTrigger('Automate repeatable work')?.getAttribute('aria-expanded')).toBe('true');
    expect(journeyTrigger('Scale customer support')?.getAttribute('aria-expanded')).toBe('false');
  });

  it('keeps collapsed progress visible and uses an accessible stable content target', () => {
    setupView = view([
      journey('foundation', 'Workspace essentials', 'Add company details'),
      journey('customer_support', 'Scale customer support', 'Connect support email'),
    ]);
    renderPage();

    const supportTrigger = journeyTrigger('Scale customer support');
    const controlledId = supportTrigger?.getAttribute('aria-controls');
    const controlled = controlledId ? document.getElementById(controlledId) : null;

    expect(supportTrigger?.textContent).toContain('1/2 core complete');
    expect(controlledId).toBe('setup-journey-tasks-customer_support');
    expect(controlled?.hidden).toBe(true);
    expect(controlled?.querySelector('li')).not.toBeNull();
    expect(supportTrigger?.querySelector('svg')?.className.baseVal).toContain('motion-reduce:transition-none');
  });
});

function cap(key: Capability['key'], status: Capability['status'], overrides: Partial<Capability> = {}): Capability {
  return { key, status, detail: `${key} detail`, required: false, ...overrides };
}

function taskRow(key: string, status: SetupTask['status'] = 'available'): SetupTask {
  return { key, title: `Task ${key}`, description: '', stage: 'Core', status, shared: true, core: true };
}

function supportJourney(): SetupJourney {
  return {
    ...journey('customer_support', 'Scale customer support', 'Connect support email'),
    tasks: [taskRow('support.email_inbox_connected'), taskRow('support.ai_agent_activated'), taskRow('support.routing_enabled')],
  };
}

function taskItem(key: string) {
  return Array.from(container?.querySelectorAll('li') ?? []).find((item) => item.textContent?.includes(`Task ${key}`));
}

describe('SetupSuccessPage adoption guide', () => {
  it('shows goals, journeys and sample data without a connections list', () => {
    capabilities = { edition: 'community', capabilities: [cap('ai_chat', 'ready'), cap('email_outbound', 'needs_setup'), cap('support_widget', 'needs_setup')] };
    setupView = view([supportJourney()]);
    renderPage();

    expect(container?.querySelector('[data-testid="sample-data"]')).not.toBeNull();
    expect(container?.querySelector('[data-capability]')).toBeNull();
    expect(container?.textContent).not.toContain('Connections');
    expect(container?.textContent).not.toContain('Application email');
  });

  it('asks admins to connect AI inside the AI-dependent task only', () => {
    capabilities = { edition: 'community', capabilities: [cap('ai_chat', 'needs_setup', { action: { kind: 'open_settings', label: 'Connect an AI provider', path: 'settings/ai' } })] };
    setupView = view([supportJourney()]);
    renderPage();

    expect(container?.querySelectorAll('[data-requirement="ai"]')).toHaveLength(1);
    expect(taskItem('support.ai_agent_activated')?.querySelector('[data-requirement="ai"]')).not.toBeNull();
    expect(taskItem('support.email_inbox_connected')?.querySelector('[data-requirement]')).toBeNull();
    expect(taskItem('support.ai_agent_activated')?.textContent).toContain('Connect AI');
  });

  it('tells members a workspace admin can connect AI', () => {
    isAdmin = false;
    capabilities = { edition: 'community', capabilities: [cap('ai_chat', 'needs_setup')] };
    setupView = view([supportJourney()]);
    renderPage();

    const item = taskItem('support.ai_agent_activated');
    expect(item?.textContent).toContain('A workspace admin can connect AI.');
    expect(Array.from(item?.querySelectorAll('button') ?? []).some((element) => element.textContent?.includes('Connect AI'))).toBe(false);
  });

  it('never asks to connect AI when AI is ready or platform-managed', () => {
    capabilities = { edition: 'community', capabilities: [cap('ai_chat', 'ready')] };
    setupView = view([supportJourney()]);
    renderPage();
    expect(container?.querySelector('[data-requirement="ai"]')).toBeNull();

    capabilities = { edition: 'enterprise', capabilities: [cap('ai_chat', 'needs_setup')] };
    renderPage();
    expect(container?.querySelector('[data-requirement="ai"]')).toBeNull();
  });

  it('inlines the GitHub App step in the repository task', () => {
    capabilities = { edition: 'community', capabilities: [cap('github', 'needs_setup', { action: { kind: 'open_settings', label: 'Set up GitHub', path: 'settings/git-connections' } })] };
    setupView = view([{
      ...journey('product_delivery', 'Plan and ship team projects', 'Plan epic'),
      tasks: [taskRow('product.project_planned'), taskRow('product.repository_ready')],
    }]);
    renderPage();

    expect(taskItem('product.repository_ready')?.querySelector('[data-testid="github-step"]')).not.toBeNull();
    expect(taskItem('product.project_planned')?.querySelector('[data-requirement]')).toBeNull();
  });

  it.skipIf(!systemStatusEnabled)('shows admins one notice linking to System status when a required service is down', () => {
    capabilities = {
      edition: 'community',
      capabilities: [cap('object_storage', 'needs_setup', { required: true }), cap('workers', 'unable_to_verify', { required: true }), cap('email_outbound', 'needs_setup')],
    };
    setupView = view([journey('foundation', 'Workspace essentials', 'Add company details')]);
    renderPage();

    const notices = container?.querySelectorAll('[data-testid="required-services-notice"]');
    expect(notices).toHaveLength(1);
    expect(notices?.[0].textContent).toContain('2 required server services need attention.');
    expect(notices?.[0].querySelector('a')?.getAttribute('href')).toBe('/w/acme/settings/system-status');
    expect(notices?.[0].querySelector('li')).toBeNull();
  });

  it('hides the required-service notice from members, on other editions, and when services are ready', () => {
    const down = [cap('object_storage', 'needs_setup', { required: true })];
    setupView = view([journey('foundation', 'Workspace essentials', 'Add company details')]);

    isAdmin = false;
    capabilities = { edition: 'community', capabilities: down };
    renderPage();
    expect(container?.querySelector('[data-testid="required-services-notice"]')).toBeNull();

    isAdmin = true;
    capabilities = { edition: 'enterprise', capabilities: down };
    renderPage();
    expect(container?.querySelector('[data-testid="required-services-notice"]')).toBeNull();

    capabilities = { edition: 'community', capabilities: [cap('object_storage', 'ready', { required: true }), cap('workers', 'ready', { required: true })] };
    renderPage();
    expect(container?.querySelector('[data-testid="required-services-notice"]')).toBeNull();
  });
});

describe('SetupSuccessPage GitHub return', () => {
  afterEach(() => window.history.replaceState(null, '', '/'));

  it('announces the GitHub result inline and removes it from the address bar', async () => {
    window.history.replaceState(null, '', '/w/acme/setup?github=connected&github_message=GitHub+App+connected+to+acme.&integration_id=gi-1&tab=x');
    setupView = view([journey('foundation', 'Workspace essentials', 'Add company details')]);
    renderPage();
    await act(async () => { await new Promise((resolve) => setTimeout(resolve, 0)); });

    const status = Array.from(container?.querySelectorAll('[role="status"]') ?? []).find((element) => element.textContent?.includes('GitHub'));
    expect(status?.textContent).toBe('GitHub App connected to acme.');
    expect(status?.getAttribute('aria-live')).toBe('polite');
    expect(window.location.search).toBe('?tab=x');
  });

  it('shows failures from the older manifest flag', async () => {
    window.history.replaceState(null, '', '/w/acme/setup?github_app_manifest=error&github_message=Only+a+workspace+owner+can+create+the+GitHub+App.');
    setupView = view([journey('foundation', 'Workspace essentials', 'Add company details')]);
    renderPage();
    await act(async () => { await new Promise((resolve) => setTimeout(resolve, 0)); });

    const status = Array.from(container?.querySelectorAll('[role="status"]') ?? []).find((element) => element.textContent?.includes('owner'));
    expect(status?.className).toContain('text-quiet-accent');
    expect(window.location.search).toBe('');
  });
});
