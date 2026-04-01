// @vitest-environment jsdom
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { CodingInterruptionPanel } from '../CodingSession/CodingInterruptionPanel';
import type { CodingSession, CodingSessionInteraction } from '@/lib/pmTypes';

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true });

function buildSession(overrides: Partial<CodingSession> = {}): CodingSession {
  return {
    id: 'session-1',
    run_id: 'run-1',
    workspace_id: 'ws-1',
    target_type: 'story',
    target_id: 'story-1',
    agent_id: 'agent-1',
    runtime_kind: 'codex',
    invocation_mode: 'interactive',
    status: 'paused',
    pause_reason: 'authentication',
    title: 'Coding Session',
    capabilities: {
      live_text_streaming: true,
      tool_streaming: true,
      repo_diff_streaming: true,
      plan_streaming: false,
      approvals: true,
      human_input: true,
      authentication: true,
      previews: false,
      terminal_output: true,
      checkpoints: false,
    },
    repo: {
      is_dirty: false,
      changed_file_count: 0,
      changed_files: [],
    },
    auth_state: {
      state: 'pending',
      auth_mode: 'chatgpt_device_code',
      verification_url: 'https://chatgpt.com/device',
      user_code: 'ABCD-EFGH',
      updated_at: '2026-03-31T10:00:00Z',
    },
    created_at: '2026-03-31T10:00:00Z',
    updated_at: '2026-03-31T10:00:00Z',
    ...overrides,
  };
}

function buildInteraction(overrides: Partial<CodingSessionInteraction> = {}): CodingSessionInteraction {
  return {
    interaction_id: 'interaction-1',
    interaction_kind: 'request_user_input',
    status: 'pending',
    request_schema_version: 'codex.v2',
    request_payload: {
      questions: [
        {
          id: 'continue',
          header: 'Continue',
          question: 'How should the coding run continue?',
          isOther: true,
          isSecret: false,
          options: [
            {
              label: 'Continue coding now',
              description: 'Resume immediately.',
            },
          ],
        },
      ],
    },
    ...overrides,
  };
}

describe('CodingInterruptionPanel', () => {
  afterEach(() => {
    document.body.innerHTML = '';
  });

  it('renders ChatGPT device-code controls when authentication is required', () => {
    const container = document.createElement('div');
    document.body.appendChild(container);
    const root = createRoot(container);

    act(() => {
      root.render(
        <CodingInterruptionPanel
          session={buildSession()}
          activeInteraction={null}
          acting={null}
          onAuthStart={vi.fn()}
          onAuthCancel={vi.fn()}
          onResolveInteraction={vi.fn()}
          onCancelRun={vi.fn()}
        />,
      );
    });

    expect(container.textContent).toContain('ChatGPT sign-in required');
    expect(container.textContent).toContain('ABCD-EFGH');
    expect(container.textContent).toContain('Open verification page');
    expect(container.textContent).toContain('Cancel sign-in');

    act(() => {
      root.unmount();
    });
  });

  it('renders resume controls when human input is required', () => {
    const container = document.createElement('div');
    document.body.appendChild(container);
    const root = createRoot(container);

    act(() => {
      root.render(
        <CodingInterruptionPanel
          session={buildSession({
            pause_reason: 'human_input',
            auth_state: undefined,
          })}
          activeInteraction={buildInteraction()}
          acting={null}
          onAuthStart={vi.fn()}
          onAuthCancel={vi.fn()}
          onResolveInteraction={vi.fn()}
          onCancelRun={vi.fn()}
        />,
      );
    });

    expect(container.textContent).toContain('User input required');
    expect(container.textContent).toContain('How should the coding run continue?');
    expect(container.textContent).toContain('Submit answers');

    act(() => {
      root.unmount();
    });
  });
});
