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

function clickButton(label: string) {
  const button = Array.from(container.querySelectorAll('button')).find((candidate) => candidate.textContent?.trim() === label);
  expect(button).toBeTruthy();
  act(() => {
    button!.dispatchEvent(new MouseEvent('click', { bubbles: true }));
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
  });

  it('lets users expand completed session details manually', () => {
    renderHeader(buildSession());

    clickButton('Details');

    expect(container.textContent).toContain('Tokens');
    expect(container.textContent).toContain('Progress');
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
    expect(container.textContent).toContain('Details');
  });
});
