// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { CapabilitiesResponse, Capability } from '@/lib/capabilityTypes';
import type { SetupTask } from '@/lib/setupTypes';
import { SetupTaskRequirement } from '../SetupTaskRequirement';
import { journeyTaskRequirements } from '../setupTaskRequirements';
import { button, capability, click, renderWithQuery, type Rendered } from './setupTestUtils';

vi.mock('@tanstack/react-router', () => ({
  Link: ({ to, children, className }: { to: string; children: React.ReactNode; className?: string }) => <a href={to} className={className}>{children}</a>,
}));
vi.mock('../SetupAIStep', () => ({
  SetupAIStep: ({ canManage }: { canManage: boolean }) => <div data-testid="ai-step" data-can-manage={String(canManage)} />,
}));
vi.mock('../SetupGitHubStep', () => ({
  SetupGitHubStep: ({ isOwner }: { isOwner: boolean }) => <div data-testid="github-step" data-owner={String(isOwner)} />,
}));

let rendered: Rendered | undefined;
afterEach(async () => {
  await rendered?.unmount();
  rendered = undefined;
});

function task(key: string, status: SetupTask['status'] = 'available'): SetupTask {
  return { key, title: key, description: '', stage: 'Core', status, shared: true, core: true };
}

function response(capabilities: Capability[], edition: CapabilitiesResponse['edition'] = 'community'): CapabilitiesResponse {
  return { edition, capabilities };
}

const aiNeedsSetup = capability({ key: 'ai_chat', status: 'needs_setup', action: { kind: 'open_settings', label: 'Connect an AI provider', path: 'settings/ai' } });

describe('journeyTaskRequirements', () => {
  const support = [
    task('support.email_inbox_connected'),
    task('support.ai_agent_activated', 'blocked'),
    task('support.coverage_fix_applied'),
    task('support.routing_enabled'),
  ];

  it('asks for AI once per journey, on the first actionable AI-dependent task', () => {
    const requirements = journeyTaskRequirements(support, response([aiNeedsSetup]));

    expect([...requirements]).toEqual([['support.coverage_fix_applied', 'ai']]);
  });

  it('skips completed tasks and non-AI tasks', () => {
    const tasks = [task('automation.first_assisted_value', 'completed'), task('automation.flow_enabled'), task('automation.custom_agent_succeeded')];

    expect([...journeyTaskRequirements(tasks, response([aiNeedsSetup]))]).toEqual([['automation.custom_agent_succeeded', 'ai']]);
    expect(journeyTaskRequirements([task('foundation.team_ready')], response([aiNeedsSetup])).size).toBe(0);
  });

  it('never asks for AI when it is ready, untested, or platform-managed', () => {
    const tasks = [task('product.agent_result_used')];

    expect(journeyTaskRequirements(tasks, response([capability({ key: 'ai_chat', status: 'ready' })])).size).toBe(0);
    expect(journeyTaskRequirements(tasks, response([capability({ key: 'ai_chat', status: 'unable_to_verify' })])).size).toBe(0);
    expect(journeyTaskRequirements(tasks, response([aiNeedsSetup], 'enterprise')).size).toBe(0);
    expect(journeyTaskRequirements(tasks, undefined).size).toBe(0);
  });

  it('adds the GitHub App step to the repository task until only repository selection is left', () => {
    const tasks = [task('product.project_planned'), task('product.repository_ready')];
    const notConfigured = capability({ key: 'github', status: 'needs_setup', action: { kind: 'open_settings', label: 'Set up GitHub', path: 'settings/git-connections' } });
    const selectRepositories = capability({ key: 'github', status: 'needs_setup', action: { kind: 'open_settings', label: 'Select repositories', path: 'settings/repositories' } });

    expect([...journeyTaskRequirements(tasks, response([notConfigured]))]).toEqual([['product.repository_ready', 'github']]);
    expect([...journeyTaskRequirements(tasks, response([notConfigured], 'enterprise'))]).toEqual([['product.repository_ready', 'github']]);
    expect(journeyTaskRequirements(tasks, response([selectRepositories])).size).toBe(0);
    expect(journeyTaskRequirements(tasks, response([capability({ key: 'github', status: 'ready' })])).size).toBe(0);
  });
});

describe('SetupTaskRequirement', () => {
  async function renderRequirement(kind: 'ai' | 'github', canManage: boolean) {
    const github = capability({ key: 'github', status: 'needs_setup', action: { kind: 'open_settings', label: 'Set up GitHub', path: 'settings/git-connections' } });
    rendered = await renderWithQuery(
      <SetupTaskRequirement kind={kind} capabilities={response([aiNeedsSetup, github])} workspaceId="ws-1" slug="acme" canManage={canManage} isOwner />,
    );
    return rendered.container;
  }

  it('lets admins open the inline AI connection step', async () => {
    const container = await renderRequirement('ai', true);
    const toggle = button(container, 'Connect AI');

    expect(container.textContent).toContain('This step needs an AI provider.');
    expect(toggle?.getAttribute('aria-expanded')).toBe('false');
    expect(container.querySelector('[data-testid="ai-step"]')).toBeNull();

    await click(toggle);

    expect(toggle?.getAttribute('aria-expanded')).toBe('true');
    expect(container.querySelector('[data-testid="ai-step"]')?.getAttribute('data-can-manage')).toBe('true');
  });

  it('tells members a workspace admin can connect AI without a settings link', async () => {
    const container = await renderRequirement('ai', false);

    expect(container.textContent).toContain('A workspace admin can connect AI.');
    expect(button(container, 'Connect AI')).toBeUndefined();
    // Personal AI settings cannot make the workspace AI ready, so members get no link.
    expect(container.querySelector('a')).toBeNull();
  });

  it('inlines the GitHub App step in the repository task', async () => {
    const container = await renderRequirement('github', true);

    expect(container.querySelector('[data-testid="github-step"]')?.getAttribute('data-owner')).toBe('true');
    expect(container.textContent).toContain('GitHub needs to be connected');
  });
});
