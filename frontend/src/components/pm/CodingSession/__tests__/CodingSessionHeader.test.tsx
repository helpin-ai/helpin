// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { TooltipProvider } from '@/components/ui/tooltip';
import { CodingSessionHeader } from '../CodingSessionHeader';
import type { CodingSession } from '@/lib/pmTypes';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

let container: HTMLDivElement;
let root: Root;

beforeEach(() => {
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
});

afterEach(() => {
  vi.useRealTimers();
  act(() => {
    root.unmount();
  });
  container.remove();
});

function buildSession(overrides: Partial<CodingSession> = {}): CodingSession {
  return {
    id: 'run-1',
    run_id: 'run-1',
    workspace_id: 'workspace-1',
    target_type: 'task',
    target_id: 'task-1',
    agent_id: 'agent-1',
    runtime_kind: 'codex',
    invocation_mode: 'manual',
    status: 'completed',
    pause_reason: 'none',
    title: 'Forge',
    capabilities: {
      can_cancel: true,
      can_retry: true,
      can_continue: true,
      can_start_followup: true,
    },
    repo: {
      repo_name: 'd4interactive/contentstudio-website-v2',
      branch: 'feature/cont-139-create-a-new-page-for-hootsuite-alternative',
      base_branch: 'main',
    },
    cached_input_tokens: 119000,
    input_tokens: 120000,
    output_tokens: 722,
    tokens_used: 120722,
    created_at: '2026-05-07T08:00:00Z',
    updated_at: '2026-05-07T09:00:00Z',
    ...overrides,
  };
}

function renderHeader(session: CodingSession) {
  act(() => {
    root.render(
      <TooltipProvider>
        <CodingSessionHeader
          session={session}
          statusIcon={<span />}
          workspaceSlug="workspace"
          onRefresh={() => {}}
          onCancelRun={() => {}}
        />
      </TooltipProvider>,
    );
  });
}

describe('CodingSessionHeader', () => {
  it('renders completed sessions in a compact header with full repo and branch titles', () => {
    const session = buildSession();

    renderHeader(session);

    const repoChip = container.querySelector('[data-coding-session-repo-chip]');
    const branchChip = container.querySelector('[data-coding-session-branch-chip]');

    expect(repoChip?.getAttribute('title')).toBe(session.repo.repo_name);
    expect(branchChip?.getAttribute('title')).toBe(session.repo.branch);
    expect(container.textContent).toContain('Forge');
    expect(container.textContent).toContain('Completed');
    expect(container.textContent).not.toContain('Tokens');
    expect(container.textContent).not.toContain('Progress');

    const backAction = Array.from(container.querySelectorAll('[data-slot="tooltip-trigger"]')).find((node) => (
      node.textContent?.trim() === 'Back to activity'
    ));
    expect(backAction).toBeTruthy();

    const cancelAction = container.querySelector('button[aria-label="Cancel run"]');
    const refreshAction = container.querySelector('button[aria-label="Refresh"]');
    expect(cancelAction?.getAttribute('aria-disabled')).toBe('true');
    expect(cancelAction?.hasAttribute('disabled')).toBe(false);
    expect(cancelAction?.className).toContain('hover:bg-destructive/10');
    expect(cancelAction?.className).toContain('hover:text-destructive');
    expect(refreshAction?.getAttribute('aria-disabled')).toBe('false');
    expect(refreshAction?.hasAttribute('disabled')).toBe(false);
  });

  it('shows completed session details while the header is being inspected', () => {
    const session = buildSession();
    renderHeader(session);

    act(() => {
      container.firstElementChild?.dispatchEvent(new MouseEvent('mouseover', { bubbles: true }));
    });

    expect(container.textContent).toContain('Tokens');
    expect(container.textContent).not.toContain('Progress');
    expect(container.textContent).toContain('Queued');
    expect(container.textContent).toContain('Preparing');
    expect(container.textContent).toContain('Starting agent');
    expect(container.textContent).toContain('Working');
    expect(container.textContent?.match(/d4interactive\/contentstudio-website-v2/g)).toHaveLength(1);
    expect(container.textContent?.match(/feature\/cont-139-create-a-new-page-for-hootsuite-alternative/g)).toHaveLength(1);

    const tokenTrigger = container.querySelector('button[aria-label^="Token usage:"]');
    expect(tokenTrigger?.getAttribute('aria-label')).toBe('Token usage: 120k input (119k cached) / 722 output');
    expect(tokenTrigger?.hasAttribute('title')).toBe(false);
  });

  it('auto-collapses running session details after a short delay', () => {
    vi.useFakeTimers();
    renderHeader(buildSession({
      status: 'running',
      execution_stage: 'codex_running',
      started_at: '2026-05-07T08:00:00Z',
    }));

    expect(container.textContent).toContain('Tokens');

    act(() => {
      vi.advanceTimersByTime(5200);
    });

    expect(container.textContent).not.toContain('Tokens');
    expect(container.textContent).not.toContain('Details');
    expect(container.textContent).not.toContain('Hide details');
  });

  it('freezes elapsed time while waiting for approval', () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-05-07T08:30:00Z'));

    renderHeader(buildSession({
      status: 'paused',
      pause_reason: 'human_approval',
      started_at: '2026-05-07T08:00:00Z',
      updated_at: '2026-05-07T08:07:15Z',
    }));

    expect(container.textContent).toContain('7m 15s');
    expect(container.textContent).toContain('Awaiting approval');
    expect(container.textContent).not.toContain('Human Approval');

    act(() => {
      vi.advanceTimersByTime(10_000);
    });

    expect(container.textContent).toContain('7m 15s');
    expect(container.textContent).not.toContain('7m 25s');
  });

  it('uses lifecycle colors for running and approval status labels', () => {
    renderHeader(buildSession({
      status: 'running',
      execution_stage: 'codex_running',
      started_at: '2026-05-07T08:00:00Z',
    }));

    const runningBadge = Array.from(container.querySelectorAll('[data-slot="badge"]')).find((badge) => (
      badge.textContent?.includes('Agent working')
    ));
    expect(runningBadge?.className).toContain('bg-primary/10');

    act(() => {
      root.render(
        <TooltipProvider>
          <CodingSessionHeader
            session={buildSession({
              id: 'run-approval',
              status: 'paused',
              pause_reason: 'human_approval',
              started_at: '2026-05-07T08:00:00Z',
              updated_at: '2026-05-07T08:07:15Z',
            })}
            statusIcon={<span />}
            workspaceSlug="workspace"
            onRefresh={() => {}}
            onCancelRun={() => {}}
          />
        </TooltipProvider>,
      );
    });

    const approvalBadge = Array.from(container.querySelectorAll('[data-slot="badge"]')).find((badge) => (
      badge.textContent?.includes('Awaiting approval')
    ));
    expect(approvalBadge?.className).toContain('bg-amber-500/10');
  });

  it('uses user-facing lifecycle terms in the progress strip', () => {
    renderHeader(buildSession({
      status: 'paused',
      pause_reason: 'human_approval',
      started_at: '2026-05-07T08:00:00Z',
      updated_at: '2026-05-07T08:07:15Z',
    }));

    expect(container.textContent).toContain('Preparing');
    expect(container.textContent).toContain('Starting agent');
    expect(container.textContent).toContain('Approval');
    expect(container.textContent).not.toContain('Workspace');
    expect(container.textContent).not.toContain('Runtime');
  });
});
