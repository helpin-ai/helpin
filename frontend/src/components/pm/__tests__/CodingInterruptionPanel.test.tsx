// @vitest-environment jsdom
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import { afterEach, describe, expect, it, vi } from 'vitest';

// CodingTranscriptPane internally calls useWorkspaceMembers (TanStack Query).
// Stub it so tests don't need a QueryClientProvider — they only assert
// transcript rendering behavior, not workspace-member resolution.
vi.mock('@/hooks/queries/useWorkspaces', () => ({
  useWorkspaceMembers: () => ({ data: [], isLoading: false, error: null }),
}));

vi.mock('@tanstack/react-virtual', () => ({
  useVirtualizer: ({ count, getScrollElement }: { count: number; getScrollElement: () => HTMLElement | null }) => ({
    getTotalSize: () => count * 120,
    getVirtualItems: () => Array.from({ length: count }, (_, index) => ({
      index,
      key: index,
      start: index * 120,
    })),
    measureElement: () => {},
    scrollToIndex: (index: number) => {
      getScrollElement()?.scrollTo?.({ top: index * 120 });
    },
  }),
}));

import { CodingTranscriptPane } from '../CodingSession/CodingTranscriptPane';
import type {
  AgentRunArtifact,
  CodingSession,
  CodingSessionInteraction,
  CodingSessionLiveTurnSegment,
} from '@/lib/pmTypes';

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true });

function buildSession(overrides: Partial<CodingSession> = {}): CodingSession {
  return {
    id: 'session-1',
    run_id: 'run-1',
    workspace_id: 'ws-1',
    target_type: 'task',
    target_id: 'task-1',
    agent_id: 'agent-1',
    runtime_kind: 'codex',
    invocation_mode: 'interactive',
    status: 'paused',
    pause_reason: 'authentication',
    approval_state: 'not_required',
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

function buildReviewArtifact(overrides: Partial<AgentRunArtifact> = {}): AgentRunArtifact {
  return {
    id: 'artifact-1',
    workspace_id: 'ws-1',
    run_id: 'run-1',
    artifact_type: 'review_findings',
    format: 'json',
    storage_mode: 'inline',
    inline_content: JSON.stringify({
      overall_explanation: 'Two issues remain before this is safe to merge.',
      findings: [
        {
          id: 'finding_1',
          title: 'Nil panic in retry path',
          body: 'The retry branch dereferences a nil client.',
          priority: 'P1',
          code_location: 'server/internal/service/foo.go:42',
        },
      ],
    }),
    metadata: {},
    sequence_no: 1,
    created_at: '2026-03-31T10:00:00Z',
    ...overrides,
  };
}

function buildPromptArtifact(overrides: Partial<AgentRunArtifact> = {}): AgentRunArtifact {
  return {
    id: 'artifact-prompt-1',
    workspace_id: 'ws-1',
    run_id: 'run-1',
    artifact_type: 'codex_prompt',
    format: 'markdown',
    storage_mode: 'inline',
    inline_content: [
      'Developer prompt:',
      'Use the repository conventions and keep changes incremental.',
      '',
      'User prompt:',
      'Implement the requested change.',
    ].join('\n'),
    metadata: {},
    sequence_no: 0,
    created_at: '2026-03-31T09:59:00Z',
    ...overrides,
  };
}

function buildReviewDecisionArtifact(overrides: Partial<AgentRunArtifact> = {}): AgentRunArtifact {
  return {
    id: 'artifact-decision-1',
    workspace_id: 'ws-1',
    run_id: 'run-1',
    artifact_type: 'review_decision',
    format: 'json',
    storage_mode: 'inline',
    inline_content: JSON.stringify({
      decision: 'approve',
      findings: [
        {
          id: 'finding_1',
          title: 'Nil panic in retry path',
          status: 'approved',
        },
      ],
      assistant_message_sequence_no: 0,
    }),
    metadata: {},
    sequence_no: 2,
    created_at: '2026-03-31T10:01:00Z',
    ...overrides,
  };
}

function buildVerdictOnlyReviewArtifact(overrides: Partial<AgentRunArtifact> = {}): AgentRunArtifact {
  return {
    id: 'artifact-verdict-1',
    workspace_id: 'ws-1',
    run_id: 'run-1',
    artifact_type: 'review_findings',
    format: 'json',
    storage_mode: 'inline',
    inline_content: JSON.stringify({
      title: 'Producer Prometheus docs and alert rules added',
      summary: 'Implemented the three approved findings by adding templated producer alert rules, a runbook, and promtool unit tests.',
      overall_correctness: 'correct',
      overall_explanation: 'The requested deliverables now exist on the branch and validation passed.',
      overall_confidence_score: 0.96,
    }),
    metadata: {},
    sequence_no: 3,
    created_at: '2026-03-31T10:02:00Z',
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

  it('renders browser-continue controls when auth falls back to browser flow', () => {
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
            auth_state: {
              state: 'pending',
              auth_mode: 'chatgpt_device_code',
              auth_url: 'https://chatgpt.com/login',
              updated_at: '2026-03-31T10:00:00Z',
            },
          })}
          activeInteraction={null}
          acting={null}
          onAuthStart={vi.fn()}
          onAuthCancel={vi.fn()}
          onResolveInteraction={vi.fn()}
        />,
      );
    });

    expect(container.textContent).toContain('browser-based auth instead of a device code');
    expect(container.textContent).toContain('Continue in browser');
    expect(container.textContent).toContain('Cancel sign-in');
    expect(container.textContent).not.toContain('Open verification page');

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
          reviewArtifacts={[{ artifact: buildReviewArtifact(), decisionArtifact: buildReviewDecisionArtifact() }]}
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
    expect(container.textContent).toContain('Review history');
    expect(container.textContent).toContain('Nil panic in retry path');
    expect(container.textContent).toContain('approved');

    act(() => {
      root.unmount();
    });
  });

  it('renders verdict-only review history artifacts', () => {
    const container = document.createElement('div');
    document.body.appendChild(container);
    const root = createRoot(container);

    act(() => {
      root.render(
        <CodingTranscriptPane
          reviewArtifacts={[{ artifact: buildVerdictOnlyReviewArtifact(), decisionArtifact: null }]}
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

    expect(container.textContent).toContain('Review history');
    expect(container.textContent).toContain('Producer Prometheus docs and alert rules added');
    expect(container.textContent).toContain('correct');
    expect(container.textContent).toContain('96% confidence');

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

  it('renders incomplete markdown formatting in live assistant transcript messages', () => {
    const container = document.createElement('div');
    document.body.appendChild(container);
    const root = createRoot(container);
    const liveSegments: CodingSessionLiveTurnSegment[] = [
      {
        segment_id: 'segment-1',
        kind: 'assistant_message',
        assistant_message: {
          message_id: 'message-live-1',
          content: 'Working through **streaming markdown',
          started_at: '2026-03-31T10:00:00Z',
          status: 'streaming',
          tool_calls: [],
        },
      },
    ];

    act(() => {
      root.render(
        <CodingTranscriptPane
          transcriptMessages={[]}
          liveAssistantMessage={null}
          liveReasoningMessage={null}
          liveTurnSegments={liveSegments}
        />,
      );
    });

    expect(container.querySelector('strong')?.textContent).toBe('streaming markdown');
    expect(container.textContent).not.toContain('**streaming markdown');

    act(() => {
      root.unmount();
    });
  });

  it('does not render the agent system prompt as user context', () => {
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
            status: 'completed',
            pause_reason: 'none',
            auth_state: undefined,
            system_prompt: 'Research configured competitors and file the changelog tracking report task.',
          })}
        />,
      );
    });

    expect(container.textContent).not.toContain('System prompt');
    expect(container.textContent).not.toContain('Research configured competitors');

    act(() => {
      root.unmount();
    });
  });

  it('uses the user section of legacy prompt artifacts', () => {
    const container = document.createElement('div');
    document.body.appendChild(container);
    const root = createRoot(container);

    act(() => {
      root.render(
        <CodingTranscriptPane
          promptArtifact={buildPromptArtifact()}
          transcriptMessages={[]}
          liveAssistantMessage={null}
          liveReasoningMessage={null}
          liveTurnSegments={[]}
          session={buildSession({
            status: 'completed',
            pause_reason: 'none',
            auth_state: undefined,
            system_prompt: 'Fallback native agent system prompt.',
          })}
        />,
      );
    });

    expect(container.textContent).toContain('Prompt');
    expect(container.textContent).toContain('Implement the requested change');
    expect(container.textContent).not.toContain('Use the repository conventions');
    expect(container.textContent).not.toContain('Fallback native agent system prompt');

    act(() => {
      root.unmount();
    });
  });

  it('renders persisted apply_patch tool calls that are not present in turn segments', () => {
    const container = document.createElement('div');
    document.body.appendChild(container);
    const root = createRoot(container);

    act(() => {
      root.render(
        <CodingTranscriptPane
          transcriptMessages={[
            {
              event_id: 'event-apply-patch',
              message_id: 'assistant-apply-patch',
              role: 'assistant',
              content: '',
              timestamp: '2026-03-31T10:00:00Z',
              sequence_no: 1,
              turn_segments: [
                {
                  segment_id: 'assistant-segment-1',
                  kind: 'assistant_message',
                  assistant_message: {
                    message_id: 'assistant-apply-patch',
                    content: 'Applying the requested change.',
                    status: 'completed',
                    tool_calls: [],
                  },
                },
                {
                  segment_id: 'tool-segment-1',
                  kind: 'tool_call',
                  tool_call: {
                    tool_call_id: 'tool-read-1',
                    parent_message_id: 'assistant-apply-patch',
                    tool_name: 'read_file',
                    args_text: '{"path":"frontend/src/App.tsx"}',
                    status: 'completed',
                  },
                },
              ],
              tool_calls: [
                {
                  tool_call_id: 'tool-read-1-fallback',
                  parent_message_id: 'assistant-apply-patch',
                  tool_name: 'read_file',
                  args_text: '{"path":"frontend/src/App.tsx"}',
                  status: 'completed',
                },
                {
                  tool_call_id: 'tool-patch-1',
                  parent_message_id: 'assistant-apply-patch',
                  tool_name: 'apply_patch',
                  args_text: [
                    '*** Begin Patch',
                    '*** Update File: frontend/src/App.tsx',
                    '@@',
                    '-old',
                    '+new',
                    '*** End Patch',
                  ].join('\n'),
                  status: 'completed',
                },
              ],
            },
          ]}
          liveAssistantMessage={null}
          liveReasoningMessage={null}
          liveTurnSegments={[]}
        />,
      );
    });

    expect(container.textContent).toContain('Apply patch');
    expect(container.textContent).toContain('frontend/src/App.tsx');

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

    const scrollContainer = container.querySelector('.min-h-0.flex-1.overflow-auto') as HTMLDivElement | null;
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

    expect(scrollTo).toHaveBeenCalledWith(expect.objectContaining({ top: 120 }));

    act(() => {
      root.unmount();
    });

    requestAnimationFrameSpy.mockRestore();
    cancelAnimationFrameSpy.mockRestore();
  });
});
