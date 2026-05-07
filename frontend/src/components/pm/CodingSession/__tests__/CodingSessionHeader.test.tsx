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
    expect(container.textContent).toContain('Tokens');
    expect(container.textContent).not.toContain('Progress');
    const titleRow = container.querySelector('[data-coding-session-title-row]');
    expect(titleRow?.querySelector('span[aria-hidden="true"]')).toBeTruthy();
    expect(titleRow?.querySelector('h1')?.textContent).toBe('Forge');
    expect(titleRow?.textContent).not.toContain('Codex');

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

  it('shows completed session details without requiring inspection', () => {
    const session = buildSession();
    renderHeader(session);

    expect(container.textContent).toContain('Tokens');
    expect(container.textContent).not.toContain('Progress');
    const currentStage = container.querySelector('[data-coding-session-lifecycle-stage]');
    expect(currentStage?.textContent).toContain('Completed');
    expect(container.querySelectorAll('[data-coding-session-lifecycle-stage]')).toHaveLength(1);
    expect(container.textContent?.match(/d4interactive\/contentstudio-website-v2/g)).toHaveLength(1);
    expect(container.textContent?.match(/feature\/cont-139-create-a-new-page-for-hootsuite-alternative/g)).toHaveLength(1);

    const tokenTrigger = container.querySelector('button[aria-label^="Token usage:"]');
    const runtimePill = container.querySelector('[data-coding-session-runtime-pill]');
    expect(tokenTrigger?.getAttribute('aria-label')).toBe('Token usage: 120k input (119k cached) / 722 output');
    expect(tokenTrigger?.hasAttribute('title')).toBe(false);
    expect(runtimePill?.textContent).toBe('Codex');
    expect(runtimePill?.closest('[data-coding-session-detail-row]')).toBeTruthy();
    expect(
      (tokenTrigger?.compareDocumentPosition(runtimePill as Node) ?? 0) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
  });

  it('keeps running session details visible after time passes', () => {
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

    expect(container.textContent).toContain('Tokens');
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

  it('shows only the current user-facing lifecycle stage', () => {
    renderHeader(buildSession({
      status: 'paused',
      pause_reason: 'human_approval',
      started_at: '2026-05-07T08:00:00Z',
      updated_at: '2026-05-07T08:07:15Z',
    }));

    const stage = container.querySelector('[data-coding-session-lifecycle-stage]');
    expect(stage?.textContent).toContain('Approval');
    expect(stage?.textContent).toContain('Waiting for your decision');
    expect(stage?.textContent).not.toContain('Stage');
    expect(container.querySelectorAll('[data-coding-session-lifecycle-stage]')).toHaveLength(1);
    expect(stage?.textContent).not.toContain('Preparing');
    expect(stage?.textContent).not.toContain('Starting agent');
    expect(container.querySelector('.animate-pulse')).toBeNull();
  });

  it('marks the active working lifecycle step with a calm pulse', () => {
    renderHeader(buildSession({
      status: 'running',
      execution_stage: 'codex_running',
      started_at: '2026-05-07T08:00:00Z',
    }));

    const pulse = container.querySelector('.animate-pulse');
    const stage = container.querySelector('[data-coding-session-lifecycle-stage]');
    expect(pulse).toBeTruthy();
    expect(pulse?.getAttribute('aria-hidden')).toBe('true');
    expect(stage?.textContent).toContain('Working');
    expect(stage?.textContent).toContain('Agent is working');
    expect(stage?.textContent).not.toContain('Stage');
  });

  it('keeps runtime-specific starting stages on the Starting agent step', () => {
    renderHeader(buildSession({
      status: 'running',
      execution_stage: 'codex_starting',
      started_at: '2026-05-07T08:00:00Z',
    }));

    const stage = container.querySelector('[data-coding-session-lifecycle-stage]');
    expect(stage?.textContent).toContain('Starting agent');
  });

  it('shows a resuming step after approval or feedback is received', () => {
    renderHeader(buildSession({
      status: 'running',
      execution_stage: 'feedback_received',
      started_at: '2026-05-07T08:00:00Z',
    }));

    const stage = container.querySelector('[data-coding-session-lifecycle-stage]');
    expect(stage?.textContent).toContain('Resuming');
  });

  it('uses review and cancelled labels for those lifecycle states', () => {
    renderHeader(buildSession({
      status: 'paused',
      pause_reason: 'human_approval',
      execution_stage: 'awaiting_review',
      started_at: '2026-05-07T08:00:00Z',
    }));
    expect(container.textContent).toContain('Review');

    renderHeader(buildSession({
      status: 'cancelled',
      execution_stage: 'cancelled',
      completed_at: '2026-05-07T08:08:00Z',
    }));
    expect(container.textContent).toContain('Cancelled');
    expect(container.textContent).not.toContain('Done');
  });
});
