// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { CodingTranscriptPane } from '../CodingTranscriptPane';
import type {
  AgentRunArtifact,
  CodingSession,
  CodingSessionInteraction,
  CodingSessionLiveTurnSegment,
  CodingSessionTranscriptMessage,
} from '@/lib/pmTypes';

const scrollToIndexMock = vi.hoisted(() => vi.fn());

vi.mock('@tanstack/react-virtual', () => ({
  useVirtualizer: ({ count }: { count: number }) => ({
    scrollToIndex: scrollToIndexMock,
    getTotalSize: () => count * 120,
    getVirtualItems: () => Array.from({ length: count }, (_, index) => ({
      index,
      key: index,
      start: index * 120,
    })),
    measureElement: vi.fn(),
  }),
}));

vi.mock('@/hooks/queries', () => ({
  useWorkspaceMembers: () => ({
    data: [{
      user_id: 'user-1',
      email: 'john@example.com',
      full_name: 'John Doe',
      avatar_url: '',
    }],
  }),
}));

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

let container: HTMLDivElement;
let root: Root;

beforeEach(() => {
  scrollToIndexMock.mockClear();
  window.requestAnimationFrame = ((callback: FrameRequestCallback) => {
    callback(0);
    return 0;
  }) as typeof window.requestAnimationFrame;
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
});

afterEach(() => {
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
    target_type: 'document',
    target_id: 'document-1',
    agent_id: 'agent-1',
    runtime_kind: 'codex',
    invocation_mode: 'interactive',
    status: 'paused',
    pause_reason: 'human_approval',
    approval_state: 'not_required',
    title: 'Docs Operator',
    capabilities: {
      live_text_streaming: true,
      tool_streaming: true,
      repo_diff_streaming: true,
      plan_streaming: true,
      approvals: true,
      human_input: true,
      authentication: true,
      previews: true,
      terminal_output: true,
      checkpoints: true,
    },
    repo: {
      repo_name: 'helpin-ai/helpin',
      branch: 'feature/docs-task-document-approval-preview',
      base_branch: 'waqar-work',
      is_dirty: false,
      changed_file_count: 0,
      changed_files: [],
    },
    cached_input_tokens: 0,
    input_tokens: 0,
    output_tokens: 0,
    tokens_used: 0,
    created_at: '2026-05-07T08:00:00Z',
    updated_at: '2026-05-07T08:20:00Z',
    ...overrides,
  };
}

function buildApprovalInteraction(): CodingSessionInteraction {
  return {
    interaction_id: 'interaction-1',
    interaction_kind: 'approval_request',
    status: 'pending',
    request_schema_version: 'helpin.v1',
    title: 'Approve task document',
    summary: 'Review the proposed task document.',
    request_payload: {
      phase: 'task document',
      preview_panel_key: 'task_plan_doc',
      title: 'Approve task document',
    },
  };
}

function buildTranscriptMessage(overrides: Partial<CodingSessionTranscriptMessage> = {}): CodingSessionTranscriptMessage {
  return {
    event_id: 'event-1',
    message_id: 'message-1',
    role: 'assistant',
    message_type: 'message',
    content: 'Finished the latest step.',
    timestamp: '2026-05-07T08:10:00Z',
    sequence_no: 1,
    ...overrides,
  };
}

function buildReviewArtifact(overrides: Partial<AgentRunArtifact> = {}): AgentRunArtifact {
  return {
    id: 'review-artifact-1',
    workspace_id: 'workspace-1',
    run_id: 'run-1',
    artifact_type: 'review_findings',
    format: 'json',
    storage_mode: 'inline',
    inline_content: JSON.stringify({
      findings: [{
        id: 'finding-1',
        title: 'Fix null handling',
        priority: 'P1',
      }],
    }),
    metadata: {},
    sequence_no: 1,
    created_at: '2026-05-07T08:15:00Z',
    ...overrides,
  };
}

describe('CodingTranscriptPane', () => {
  it('opens an existing run with breathing room after the latest activity', () => {
    act(() => {
      root.render(
        <CodingTranscriptPane
          transcriptMessages={[
            buildTranscriptMessage({ event_id: 'event-1', message_id: 'message-1', sequence_no: 1, content: 'Earlier update' }),
            buildTranscriptMessage({ event_id: 'event-2', message_id: 'message-2', sequence_no: 2, content: 'Latest update' }),
          ]}
          liveAssistantMessage={null}
          liveReasoningMessage={null}
          liveTurnSegments={[]}
          loading={false}
          session={buildSession({ status: 'completed', pause_reason: 'none' })}
        />,
      );
    });

    expect(scrollToIndexMock).toHaveBeenCalledWith(2, expect.objectContaining({
      align: 'end',
      behavior: 'auto',
    }));
    const activitySpacer = container.querySelector('[data-coding-session-activity-spacer]');
    expect(activitySpacer?.className).toContain('h-[calc(env(safe-area-inset-bottom)+4rem)]');
    expect(container.textContent).toContain('Activity');
  });

  it('keeps bottom breathing room after approval controls in the interruption panel', () => {
    act(() => {
      root.render(
        <CodingTranscriptPane
          transcriptMessages={[]}
          liveAssistantMessage={null}
          liveReasoningMessage={null}
          liveTurnSegments={[]}
          loading={false}
          session={buildSession()}
          activeInteraction={buildApprovalInteraction()}
          acting={null}
          attachedPreview={null}
          availablePreviewPanelKey={null}
          onResolveInteraction={() => {}}
        />,
      );
    });

    const interruptionSpacer = container.querySelector('[data-coding-session-interruption-spacer]');
    expect(interruptionSpacer?.className).toContain('h-[calc(env(safe-area-inset-bottom)+5rem)]');
    expect(container.textContent).toContain('Approve task document');
    expect(container.textContent).toContain('Approve');
  });

  it('shows an approval placeholder while a paused run has no projected interaction yet', () => {
    act(() => {
      root.render(
        <CodingTranscriptPane
          transcriptMessages={[]}
          liveAssistantMessage={null}
          liveReasoningMessage={null}
          liveTurnSegments={[]}
          loading={false}
          session={buildSession({ status: 'paused', pause_reason: 'human_approval' })}
          activeInteraction={null}
          acting={null}
          attachedPreview={null}
          availablePreviewPanelKey={null}
          onResolveInteraction={() => {}}
        />,
      );
    });

    expect(container.textContent).toContain('The agent paused for your approval');
    expect(container.textContent).toContain('Loading the approval details…');
  });

  it('shows run-level approval controls when execution is gated before an interaction exists', () => {
    const onApproveRun = vi.fn();
    act(() => {
      root.render(
        <CodingTranscriptPane
          transcriptMessages={[]}
          liveAssistantMessage={null}
          liveReasoningMessage={null}
          liveTurnSegments={[]}
          loading={false}
          session={buildSession({ approval_state: 'pending' })}
          activeInteraction={null}
          acting={null}
          attachedPreview={null}
          availablePreviewPanelKey={null}
          onApproveRun={onApproveRun}
          onResolveInteraction={() => {}}
        />,
      );
    });

    expect(container.textContent).toContain('Approve this run to let the agent begin.');
    expect(container.textContent).not.toContain('Loading the approval details…');
    const approveButton = Array.from(container.querySelectorAll('button'))
      .find((button) => button.textContent === 'Approve');
    expect(approveButton).toBeTruthy();
    act(() => approveButton?.click());
    expect(onApproveRun).toHaveBeenCalledTimes(1);
  });

  it('does not show the approval placeholder for an authentication pause', () => {
    act(() => {
      root.render(
        <CodingTranscriptPane
          transcriptMessages={[]}
          liveAssistantMessage={null}
          liveReasoningMessage={null}
          liveTurnSegments={[]}
          loading={false}
          session={buildSession({ status: 'paused', pause_reason: 'authentication' })}
          activeInteraction={null}
          acting={null}
          attachedPreview={null}
          availablePreviewPanelKey={null}
          onResolveInteraction={() => {}}
        />,
      );
    });

    expect(container.textContent).not.toContain('Loading the approval details…');
    expect(container.textContent).toContain('Authentication required');
  });

  it('does not label a human-input pause as awaiting approval', () => {
    act(() => {
      root.render(
        <CodingTranscriptPane
          transcriptMessages={[]}
          liveAssistantMessage={null}
          liveReasoningMessage={null}
          liveTurnSegments={[]}
          loading={false}
          session={buildSession({ status: 'paused', pause_reason: 'human_input' })}
          activeInteraction={null}
          acting={null}
          attachedPreview={null}
          availablePreviewPanelKey={null}
          onResolveInteraction={() => {}}
        />,
      );
    });

    expect(container.textContent).not.toContain('Awaiting your approval');
    expect(container.textContent).not.toContain('The agent paused for your approval');
  });

  it('uses the subtle shared focus border on the main composer', () => {
    act(() => {
      root.render(
        <CodingTranscriptPane
          transcriptMessages={[]}
          liveAssistantMessage={null}
          liveReasoningMessage={null}
          liveTurnSegments={[]}
          loading={false}
          session={buildSession({ status: 'running', pause_reason: 'none' })}
          onSendMessage={async () => {}}
          messageComposer={{
            visible: true,
            enabled: true,
            mode: 'answer',
            placeholder: 'Answer the agent...',
          }}
        />,
      );
    });

    const composer = container.querySelector('textarea[placeholder^="Answer the agent"]');
    expect(composer?.className).toContain('focus-visible:border-ring/70');
    expect(composer?.className).toContain('focus-visible:ring-ring/15');
  });

  it('renders live assistant prose inline with live tool rows in coding sessions', () => {
    const liveTurnSegments: CodingSessionLiveTurnSegment[] = [
      {
        segment_id: 'assistant-live:segment:1',
        kind: 'assistant_message',
        assistant_message: {
          message_id: 'assistant-live',
          content: 'This streamed prose should render inline in the coding-session transcript.',
          started_at: '2026-05-07T08:12:00Z',
          status: 'streaming',
          tool_calls: [],
        },
      },
      {
        segment_id: 'tool-1',
        kind: 'tool_call',
        tool_call: {
          tool_call_id: 'tool-1',
          parent_message_id: 'assistant-live',
          tool_name: 'read_file',
          args_text: '{"path":"Dockerfile"}',
          status: 'running',
          started_at: '2026-05-07T08:12:01Z',
        },
      },
      {
        segment_id: 'assistant-live:segment:2',
        kind: 'assistant_message',
        assistant_message: {
          message_id: 'assistant-live',
          content: 'Second streamed chunk.',
          started_at: '2026-05-07T08:12:02Z',
          status: 'streaming',
          tool_calls: [],
        },
      },
    ];

    act(() => {
      root.render(
        <CodingTranscriptPane
          transcriptMessages={[]}
          liveAssistantMessage={{
            message_id: 'assistant-live',
            content: 'This streamed prose should render inline in the coding-session transcript.',
            started_at: '2026-05-07T08:12:00Z',
            status: 'streaming',
            tool_calls: [],
          }}
          liveReasoningMessage={{
            message_id: 'reasoning-live',
            content: 'Hidden reasoning stream.',
            started_at: '2026-05-07T08:12:00Z',
            status: 'streaming',
          }}
          liveTurnSegments={liveTurnSegments}
          loading={false}
          session={buildSession({ status: 'running', pause_reason: 'none' })}
        />,
      );
    });

    expect(container.textContent).toContain('This streamed prose should render inline in the coding-session transcript.');
    expect(container.textContent).toContain('Second streamed chunk.');
    expect(container.textContent).not.toContain('Hidden reasoning stream');
    expect(container.textContent).toContain('Read Dockerfile');
    expect(container.textContent.indexOf('This streamed prose should render inline')).toBeLessThan(
      container.textContent.indexOf('Read Dockerfile'),
    );
    expect(container.textContent.indexOf('Read Dockerfile')).toBeLessThan(
      container.textContent.indexOf('Second streamed chunk.'),
    );
    expect(container.querySelectorAll('.markdown-caret')).toHaveLength(1);
    expect(container.querySelector('[data-agent-streaming-status]')).toBeNull();
  });

  it('does not append review history artifacts to the main transcript', () => {
    act(() => {
      root.render(
        <CodingTranscriptPane
          reviewArtifacts={[{ artifact: buildReviewArtifact() }]}
          transcriptMessages={[
            buildTranscriptMessage({ content: 'Implemented the requested changes.' }),
          ]}
          liveAssistantMessage={null}
          liveReasoningMessage={null}
          liveTurnSegments={[]}
          loading={false}
          session={buildSession({ status: 'completed', pause_reason: 'none' })}
        />,
      );
    });

    expect(container.textContent).toContain('Implemented the requested changes.');
    expect(container.textContent).not.toContain('Review history');
  });

  it('renders a disabled composer when messages cannot be delivered', () => {
    const onSendMessage = vi.fn(async () => {});
    act(() => {
      root.render(
        <CodingTranscriptPane
          transcriptMessages={[]}
          liveAssistantMessage={null}
          liveReasoningMessage={null}
          liveTurnSegments={[]}
          loading={false}
          session={buildSession({ status: 'running', pause_reason: 'none' })}
          onSendMessage={onSendMessage}
          messageComposer={{
            visible: true,
            enabled: false,
            mode: 'waiting',
            placeholder: 'Agent is working. You can answer when it asks for input.',
          }}
        />,
      );
    });

    const composer = container.querySelector('textarea[placeholder^="Agent is working"]') as HTMLTextAreaElement | null;
    const submit = container.querySelector('[data-coding-session-message-submit]') as HTMLButtonElement | null;
    expect(composer?.disabled).toBe(true);
    expect(submit?.disabled).toBe(true);

    act(() => {
      submit?.click();
    });
    expect(onSendMessage).not.toHaveBeenCalled();
  });

  it('renders one quiet text status before live output starts', () => {
    act(() => {
      root.render(
        <CodingTranscriptPane
          transcriptMessages={[]}
          liveAssistantMessage={null}
          liveReasoningMessage={null}
          liveTurnSegments={[]}
          loading={false}
          session={buildSession({ status: 'running', pause_reason: 'none' })}
        />,
      );
    });

    const streamingStatus = container.querySelector('[data-agent-streaming-status]');
    expect(streamingStatus?.textContent).toBe('Thinking…');
    expect(streamingStatus?.querySelector('.agent-streaming-text')).not.toBeNull();
    expect(container.querySelectorAll('[data-agent-streaming-status]')).toHaveLength(1);
    expect(container.querySelector('.animate-bounce')).toBeNull();
    expect(container.querySelector('[data-agent-working-spinner]')).toBeNull();
  });

  it('auto-grows the main composer while typing', () => {
    act(() => {
      root.render(
        <CodingTranscriptPane
          transcriptMessages={[]}
          liveAssistantMessage={null}
          liveReasoningMessage={null}
          liveTurnSegments={[]}
          loading={false}
          session={buildSession({ status: 'running', pause_reason: 'none' })}
          onSendMessage={async () => {}}
          messageComposer={{
            visible: true,
            enabled: true,
            mode: 'answer',
            placeholder: 'Answer the agent...',
          }}
        />,
      );
    });

    const composer = container.querySelector('textarea[placeholder^="Answer the agent"]') as HTMLTextAreaElement | null;
    expect(composer).toBeTruthy();
    Object.defineProperty(composer!, 'scrollHeight', {
      configurable: true,
      value: 96,
    });

    act(() => {
      const valueSetter = Object.getOwnPropertyDescriptor(window.HTMLTextAreaElement.prototype, 'value')?.set;
      valueSetter?.call(composer, 'Line one\nLine two\nLine three');
      composer!.dispatchEvent(new Event('input', { bubbles: true }));
    });

    expect(composer!.style.height).toBe('96px');
    expect(composer!.style.overflowY).toBe('hidden');
  });

  it('labels approval resolution activity with the actor decision', () => {
    act(() => {
      root.render(
        <CodingTranscriptPane
          transcriptMessages={[
            buildTranscriptMessage({
              event_id: 'event-approval-resolution',
              role: 'user',
              message_type: 'approval_request_resolution',
              content: 'Requested changes on task document.\n\nNote: Split it into two tasks.',
              resolver_user_id: 'user-1',
            }),
          ]}
          liveAssistantMessage={null}
          liveReasoningMessage={null}
          liveTurnSegments={[]}
          loading={false}
          session={buildSession({ status: 'running', pause_reason: 'none' })}
        />,
      );
    });

    expect(container.textContent).toContain('John Doe requested changes');
    expect(container.textContent).toContain('Split it into two tasks.');
  });

  it('renders a visible expand control for long user transcript messages', () => {
    const longUserMessage = Array.from({ length: 80 }, (_, index) => `Line ${index + 1}: Review this implementation detail carefully.`).join('\n');

    act(() => {
      root.render(
        <CodingTranscriptPane
          transcriptMessages={[
            buildTranscriptMessage({
              event_id: 'event-user-long-message',
              message_id: 'message-user-long-message',
              role: 'user',
              content: longUserMessage,
              user_id: 'user-1',
            }),
          ]}
          liveAssistantMessage={null}
          liveReasoningMessage={null}
          liveTurnSegments={[]}
          loading={false}
          session={buildSession({ status: 'completed', pause_reason: 'none' })}
        />,
      );
    });

    const expandButton = Array.from(container.querySelectorAll<HTMLButtonElement>('button'))
      .find((button) => button.textContent === 'Show more');
    expect(expandButton).toBeTruthy();
    expect(expandButton?.className).not.toContain('text-white/80');

    act(() => {
      expandButton?.click();
    });

    expect(expandButton?.textContent).toBe('Show less');
  });
});
