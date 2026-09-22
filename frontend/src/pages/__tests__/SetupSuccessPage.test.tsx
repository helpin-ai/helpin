// @vitest-environment jsdom

import React, { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { SetupJourney, SetupView } from '@/lib/setupTypes';

const navigate = vi.fn();
const mutate = vi.fn();
let workspace = { id: 'workspace-1', slug: 'acme' };
let setupView: SetupView;

vi.mock('@tanstack/react-router', () => ({ useNavigate: () => navigate }));
vi.mock('sonner', () => ({ toast: { info: vi.fn(), error: vi.fn() } }));
vi.mock('@/lib/analytics', () => ({ trackAnalyticsEvent: vi.fn() }));
vi.mock('@/lib/services/setupService', () => ({ setupService: { startRecommendation: vi.fn() } }));
vi.mock('@/components/setup/SetupConnectionsSection', () => ({
  SetupConnectionsSection: ({ goals }: { goals: string[] }) => <section data-testid="connections" data-goals={goals.join(',')} />,
}));
vi.mock('@/components/setup/SampleDataButton', () => ({
  SampleDataCard: () => <section data-testid="sample-data" />,
}));
vi.mock('@/stores/workspaceStore', () => ({
  useWorkspaceStore: (selector: (state: { currentWorkspace: typeof workspace }) => unknown) => selector({ currentWorkspace: workspace }),
}));
vi.mock('@/hooks/queries', () => ({
  useSetup: () => ({ data: setupView, isLoading: false, isError: false, refetch: vi.fn() }),
  useUpdateSetupGoals: () => ({ mutate, isPending: false }),
  useUpdateSetupPreference: () => ({ mutate, isPending: false }),
  useWorkspaceAccess: () => ({ data: {} }),
  usePermissions: () => ({ has: () => true }),
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
  act(() => root?.render(<SetupSuccessPage />));
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
