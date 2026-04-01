// @vitest-environment jsdom
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { CodingTranscriptPane } from '../CodingSession/CodingTranscriptPane';
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
        <CodingTranscriptPane
          transcriptMessages={[]}
          liveAssistantMessage={null}
          liveReasoningMessage={null}
          liveTurnSegments={[]}
          session={buildSession()}
          activeInteraction={null}
          acting={null}
          onAuthStart={vi.fn()}
          onAuthCancel={vi.fn()}
          onResolveInteraction={vi.fn()}
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
        <CodingTranscriptPane
          transcriptMessages={[]}
          liveAssistantMessage={null}
          liveReasoningMessage={null}
          liveTurnSegments={[]}
          session={buildSession({
            pause_reason: 'human_input',
            auth_state: undefined,
          })}
          activeInteraction={buildInteraction()}
          acting={null}
          onAuthStart={vi.fn()}
          onAuthCancel={vi.fn()}
          onResolveInteraction={vi.fn()}
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

  it('shows back and next buttons for multi-question overlays', () => {
    const container = document.createElement('div');
    document.body.appendChild(container);
    const root = createRoot(container);

    act(() => {
      root.render(
        <CodingTranscriptPane
          transcriptMessages={[]}
          liveAssistantMessage={null}
          liveReasoningMessage={null}
          liveTurnSegments={[]}
          session={buildSession({
            pause_reason: 'human_input',
            auth_state: undefined,
          })}
          activeInteraction={buildInteraction({
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
                {
                  id: 'repo',
                  header: 'Repo',
                  question: 'Which repository should be used?',
                  isOther: true,
                  isSecret: false,
                  options: [
                    {
                      label: 'Current repository',
                      description: 'Use the active workspace repo.',
                    },
                  ],
                },
              ],
            },
          })}
          acting={null}
          onAuthStart={vi.fn()}
          onAuthCancel={vi.fn()}
          onResolveInteraction={vi.fn()}
        />,
      );
    });

    expect(container.textContent).toContain('Question 1 of 2');
    expect(container.textContent).toContain('How should the coding run continue?');
    expect(container.textContent).not.toContain('Which repository should be used?');
    expect(container.textContent).toContain('Next');
    const backButton = Array.from(container.querySelectorAll('button')).find((button) => button.textContent === 'Back');
    expect(backButton).toBeTruthy();
    expect(backButton?.getAttribute('disabled')).not.toBeNull();

    const firstOption = Array.from(container.querySelectorAll('button')).find((button) => button.textContent?.includes('Continue coding now'));
    expect(firstOption).toBeTruthy();

    act(() => {
      firstOption?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });

    const nextButton = Array.from(container.querySelectorAll('button')).find((button) => button.textContent === 'Next');
    expect(nextButton).toBeTruthy();

    act(() => {
      nextButton?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });

    expect(container.textContent).toContain('Question 2 of 2');
    expect(container.textContent).toContain('Which repository should be used?');
    expect(container.textContent).toContain('Back');
    expect(container.textContent).toContain('Submit answers');

    act(() => {
      root.unmount();
    });
  });

  it('renders markdown formatting in persisted user transcript messages', () => {
    const container = document.createElement('div');
    document.body.appendChild(container);
    const root = createRoot(container);

    act(() => {
      root.render(
        <CodingTranscriptPane
          transcriptMessages={[
            {
              event_id: 'event-1',
              message_id: 'message-1',
              role: 'user',
              content: '**Approved**\n\n- keep current scope',
              timestamp: '2026-03-31T10:00:00Z',
              sequence_no: 1,
            },
          ]}
          liveAssistantMessage={null}
          liveReasoningMessage={null}
          liveTurnSegments={[]}
        />,
      );
    });

    expect(container.querySelector('strong')?.textContent).toBe('Approved');
    expect(container.querySelector('ul li')?.textContent).toBe('keep current scope');

    act(() => {
      root.unmount();
    });
  });

  it('scrolls the transcript to the latest content when new turns arrive', () => {
    const container = document.createElement('div');
    document.body.appendChild(container);
    const root = createRoot(container);
    const scrollTo = vi.fn();
    const requestAnimationFrameSpy = vi
      .spyOn(window, 'requestAnimationFrame')
      .mockImplementation((callback: FrameRequestCallback) => {
        callback(200);
        return 1;
      });
    const cancelAnimationFrameSpy = vi
      .spyOn(window, 'cancelAnimationFrame')
      .mockImplementation(() => {});

    act(() => {
      root.render(
        <CodingTranscriptPane
          transcriptMessages={[]}
          liveAssistantMessage={null}
          liveReasoningMessage={null}
          liveTurnSegments={[]}
          session={buildSession()}
          activeInteraction={null}
          acting={null}
          onAuthStart={vi.fn()}
          onAuthCancel={vi.fn()}
          onResolveInteraction={vi.fn()}
        />,
      );
    });

    act(() => {
      root.render(
        <CodingTranscriptPane
          transcriptMessages={[{
            event_id: 'event-1',
            role: 'assistant',
            content: 'New streamed text',
            timestamp: '2026-03-31T10:01:00Z',
            sequence_no: 1,
          }]}
          liveAssistantMessage={null}
          liveReasoningMessage={null}
          liveTurnSegments={[]}
          session={buildSession()}
          activeInteraction={null}
          acting={null}
          onAuthStart={vi.fn()}
          onAuthCancel={vi.fn()}
          onResolveInteraction={vi.fn()}
        />,
      );
    });

    const scrollContainer = container.querySelector('.min-h-0.flex-1.overflow-auto.px-4.py-4') as HTMLDivElement | null;
    expect(scrollContainer).toBeTruthy();
    if (!scrollContainer) {
      throw new Error('expected transcript scroll container');
    }
    scrollContainer.scrollTo = scrollTo;
    Object.defineProperty(scrollContainer, 'scrollHeight', {
      configurable: true,
      value: 640,
    });

    act(() => {
      root.render(
        <CodingTranscriptPane
          transcriptMessages={[{
            event_id: 'event-1',
            role: 'assistant',
            content: 'New streamed text plus more',
            timestamp: '2026-03-31T10:01:01Z',
            sequence_no: 2,
          }]}
          liveAssistantMessage={null}
          liveReasoningMessage={null}
          liveTurnSegments={[]}
          session={buildSession()}
          activeInteraction={null}
          acting={null}
          onAuthStart={vi.fn()}
          onAuthCancel={vi.fn()}
          onResolveInteraction={vi.fn()}
        />,
      );
    });

    expect(scrollTo).toHaveBeenCalledWith(expect.objectContaining({ top: 640 }));

    act(() => {
      root.unmount();
    });

    requestAnimationFrameSpy.mockRestore();
    cancelAnimationFrameSpy.mockRestore();
  });
});
