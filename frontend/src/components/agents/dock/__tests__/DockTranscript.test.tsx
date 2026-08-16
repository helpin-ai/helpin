// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import type {
  CodingSessionLiveTurnSegment,
  CodingSessionStreamState,
  CodingSessionTranscriptMessage,
} from '@/lib/pmTypes';
import { useAuthStore } from '@/stores/authStore';
import { DockTranscript } from '../DockTranscript';

const mocks = vi.hoisted(() => ({
  resolveTeamMemberAvatarSrc: vi.fn(),
}));

vi.mock('@/lib/teamMemberAvatar', () => ({
  resolveTeamMemberAvatarSrc: mocks.resolveTeamMemberAvatarSrc,
}));

vi.mock('@/hooks/useWorkspaceMembers', () => ({
  useWorkspaceMembers: () => ({
    members: [
      {
        id: 'membership-1',
        user_id: 'user-1',
        email: 'alice@example.com',
        full_name: 'Alice Johnson',
        avatar_style: 'personas',
        avatar_seed: 'alice-seed',
        avatar_background_mode: 'color',
        avatar_background_color: '#fbbf24',
      },
      {
        id: 'membership-2',
        user_id: 'user-2',
        email: 'bob@example.com',
        full_name: 'Bob Smith',
        avatar_style: 'initials',
        avatar_seed: 'bob-seed',
      },
    ],
    loading: false,
  }),
}));

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

let container: HTMLDivElement;
let root: Root;

beforeEach(() => {
  mocks.resolveTeamMemberAvatarSrc.mockReset();
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
  useAuthStore.setState({
    user: {
      id: 'user-1',
      email: 'alice@example.com',
      full_name: 'Alice Johnson',
      avatar_style: 'personas',
      avatar_seed: 'alice-seed',
      avatar_background_mode: 'color',
      avatar_background_color: '#fbbf24',
      created_at: '2026-08-06T00:00:00Z',
      updated_at: '2026-08-06T00:00:00Z',
    },
  });
});

afterEach(() => {
  act(() => root.unmount());
  useAuthStore.setState({ user: null });
  container.remove();
});

function assistantMessage(id: string, content: string, sequenceNo: number): CodingSessionTranscriptMessage {
  return {
    event_id: id,
    message_id: id,
    role: 'assistant',
    message_type: 'message',
    content,
    timestamp: `2026-08-06T00:00:0${sequenceNo}Z`,
    sequence_no: sequenceNo,
  };
}

function userMessage(id: string, content: string, sequenceNo: number, actorUserId?: string): CodingSessionTranscriptMessage {
  return {
    event_id: id,
    message_id: id,
    role: 'user',
    message_type: 'message',
    content,
    timestamp: `2026-08-06T00:00:0${sequenceNo}Z`,
    sequence_no: sequenceNo,
    actor_user_id: actorUserId,
  };
}

function streamWithMessages(messages: CodingSessionTranscriptMessage[]): CodingSessionStreamState {
  return {
    transcript_messages: messages,
    live_assistant_message: null,
    live_reasoning_message: null,
    live_turn_segments: [],
    activity_events: [],
    current_plan: null,
    completed_tool_calls: [],
  };
}

function toolTurn(
  id: string,
  toolName: string,
  durationMs: number,
  status: 'running' | 'completed' | 'failed' = 'completed',
): CodingSessionLiveTurnSegment {
  return {
    segment_id: id,
    kind: 'tool_call',
    tool_call: {
      tool_call_id: id,
      tool_name: toolName,
      args_text: '{}',
      status,
      duration_ms: durationMs,
    },
  };
}

function assistantTurn(id: string, content: string): CodingSessionLiveTurnSegment {
  return {
    segment_id: id,
    kind: 'assistant_message',
    assistant_message: {
      message_id: id,
      content,
      status: 'completed',
      tool_calls: [],
    },
  };
}

describe('DockTranscript', () => {
  it('adds a clear turn boundary before the first assistant reply after a user message', () => {
    act(() => {
      root.render(
        <DockTranscript
          stream={streamWithMessages([
            userMessage('user-1', 'Can you check this?', 1),
            assistantMessage('assistant-1', 'Yes, I will take a look.', 2),
          ])}
          active={false}
          workspaceId="ws-1"
          compactAssistantProgress
        />,
      );
    });

    expect(container.querySelector('[data-after-user-message="true"]')?.textContent)
      .toContain('Yes, I will take a look.');
  });

  it('marks only structurally final assistant responses as high contrast across intervals', () => {
    act(() => {
      root.render(
        <DockTranscript
          stream={streamWithMessages([
            userMessage('user-1', 'First question', 1),
            assistantMessage('progress-1', 'First progress update.', 2),
            assistantMessage('final-1', 'First final response.', 3),
            userMessage('user-2', 'Follow-up question', 4),
            assistantMessage('progress-2', 'Current progress update.', 5),
          ])}
          active
          workspaceId="ws-1"
          compactAssistantProgress
        />,
      );
    });

    const rows = Array.from(container.querySelectorAll<HTMLElement>('[data-assistant-presentation]'));
    const presentation = (text: string) => rows.find((row) => row.textContent?.includes(text))?.dataset.assistantPresentation;
    expect(presentation('First progress update.')).toBe('progress');
    expect(presentation('First final response.')).toBe('final');
    expect(presentation('Current progress update.')).toBe('progress');
  });

  it('never content-collapses the final response of an earlier assistant turn', () => {
    const longFinal = `Final response ${'with complete detail '.repeat(40)}`;
    act(() => {
      root.render(
        <DockTranscript
          stream={streamWithMessages([
            userMessage('user-1', 'First question', 1),
            assistantMessage('final-1', longFinal, 2),
            userMessage('user-2', 'Follow-up question', 3),
            assistantMessage('final-2', 'Second final response.', 4),
          ])}
          active={false}
          workspaceId="ws-1"
          compactAssistantProgress
        />,
      );
    });

    const firstFinal = container.querySelector('[data-assistant-presentation="final"]');
    expect(firstFinal?.textContent).toContain(longFinal.trim());
    expect(firstFinal?.textContent).not.toContain('Show more');
  });

  it('separates a final response from preceding work in the same interval only', () => {
    const worked = {
      ...assistantMessage('worked-turn', '', 2),
      turn_segments: [
        assistantTurn('progress', 'Inspecting the source.'),
        toolTurn('tool-1', 'repository_search', 100),
        assistantTurn('final', 'The issue is identified.'),
      ],
    };
    act(() => {
      root.render(
        <DockTranscript
          stream={streamWithMessages([userMessage('user-1', 'Investigate.', 1), worked])}
          active={false}
          workspaceId="ws-1"
          compactAssistantProgress
        />,
      );
    });

    const finalWithWork = container.querySelector('[data-assistant-presentation="final"]');
    expect(finalWithWork?.textContent).toContain('The issue is identified.');
    expect(finalWithWork?.getAttribute('data-final-response-separator')).toBe('true');

    act(() => {
      root.render(
        <DockTranscript
          stream={streamWithMessages([
            userMessage('user-2', 'Answer directly.', 1),
            assistantMessage('direct-answer', 'Here is the direct answer.', 2),
          ])}
          active={false}
          workspaceId="ws-1"
          compactAssistantProgress
        />,
      );
    });
    const direct = container.querySelector('[data-assistant-presentation="final"]');
    expect(direct?.textContent).toContain('Here is the direct answer.');
    expect(direct?.hasAttribute('data-final-response-separator')).toBe(false);
  });

  it('keeps assistant narration flat while collapsing complete tool phases', () => {
    const message = {
      ...assistantMessage('assistant-tools', '', 2),
      turn_segments: [
        assistantTurn('assistant-progress-1', 'I will inspect the conversation.'),
        {
          ...toolTurn('tool-1', 'list_conversation_messages', 100),
          tool_call: {
            ...toolTurn('tool-1', 'list_conversation_messages', 100).tool_call,
            args_text: '{"conversation_id":"conversation-1"}',
            result: { content: '{"messages":12}' },
          },
        },
        assistantTurn('assistant-progress-2', 'Now I will inspect the repository.'),
        toolTurn('tool-2', 'repository_search', 100),
        assistantTurn('assistant-final', 'The pagination state is not advancing.'),
      ],
    };

    act(() => {
      root.render(
        <DockTranscript
          stream={streamWithMessages([userMessage('user-1', 'Investigate it.', 1), message])}
          active={false}
          workspaceId="ws-1"
          compactAssistantProgress
        />,
      );
    });

    const groups = container.querySelectorAll('[data-agent-working-group]');
    expect(groups).toHaveLength(2);
    expect(groups[0]?.querySelector('button')?.getAttribute('aria-expanded')).toBe('false');
    expect(groups[1]?.querySelector('button')?.getAttribute('aria-expanded')).toBe('false');
    expect(container.textContent).toContain('I will inspect the conversation.');
    expect(container.textContent).toContain('Now I will inspect the repository.');
    expect(container.textContent).toContain('The pagination state is not advancing.');
    expect(groups[0]?.textContent).toContain('list_conversation_messages');
    expect(groups[0]?.textContent).not.toContain('I will inspect the conversation.');
    expect(groups[1]?.textContent).not.toContain('The pagination state is not advancing.');
    expect(container.querySelector('[data-working-group-active="true"]')).toBeNull();

    act(() => (groups[0]?.querySelector('button') as HTMLButtonElement | null)?.click());
    expect(groups[0]?.querySelectorAll('button')).toHaveLength(1);
    expect(container.textContent).not.toContain('conversation-1');
    expect(container.textContent).not.toContain('"messages":12');
    expect(groups[0]?.querySelector('[data-working-group-label]')?.nextElementSibling)
      .toBe(groups[0]?.querySelector('[data-working-group-chevron]'));
  });

  it('uses retained runtime chronology after completion instead of the final durable tool aggregate', () => {
    const completedStream = streamWithMessages([
      userMessage('user-1', 'Investigate it.', 1),
      assistantMessage('progress-1', 'First I will inspect the conversation.', 2),
      assistantMessage('progress-2', 'Now I will inspect the documentation.', 3),
      {
        ...assistantMessage('durable-final', '', 4),
        message_id: 'final-answer',
        turn_segments: [
          toolTurn('tool-conversation', 'list_conversation_messages', 100),
          toolTurn('tool-docs', 'search_documents', 100),
          assistantTurn('final-answer', 'Here is the final diagnosis.'),
        ],
      },
    ]);
    completedStream.live_turn_segments = [
      assistantTurn('progress-1', 'First I will inspect the conversation.'),
      toolTurn('tool-conversation', 'list_conversation_messages', 100),
      assistantTurn('progress-2', 'Now I will inspect the documentation.'),
      toolTurn('tool-docs', 'search_documents', 100),
      assistantTurn('final-answer', 'Here is the final diagnosis.'),
    ];

    act(() => {
      root.render(
        <DockTranscript
          stream={completedStream}
          active={false}
          useRuntimeTimeline
          workspaceId="ws-1"
          compactAssistantProgress
        />,
      );
    });

    const text = container.textContent ?? '';
    expect(container.querySelectorAll('[data-agent-working-group]')).toHaveLength(2);
    expect(text.indexOf('First I will inspect the conversation.')).toBeLessThan(text.indexOf('list_conversation_messages'));
    expect(text.indexOf('list_conversation_messages')).toBeLessThan(text.indexOf('Now I will inspect the documentation.'));
    expect(text.indexOf('Now I will inspect the documentation.')).toBeLessThan(text.indexOf('search_documents'));
    expect(text.indexOf('search_documents')).toBeLessThan(text.indexOf('Here is the final diagnosis.'));
    expect(container.querySelector('[data-working-group-active="true"]')).toBeNull();
  });

  it('keeps the retained final response through running-to-paused handoff without stale live styling', () => {
    const handoffStream = streamWithMessages([
      userMessage('user-1', 'Investigate it.', 1),
      assistantMessage('progress-1', 'I am checking the source.', 2),
    ]);
    handoffStream.live_turn_segments = [
      assistantTurn('progress-1', 'I am checking the source.'),
      toolTurn('tool-1', 'repository_search', 100, 'running'),
      {
        segment_id: 'final-live',
        kind: 'assistant_message',
        assistant_message: {
          message_id: 'final-live',
          content: 'The final answer is ready.',
          status: 'streaming',
          tool_calls: [],
        },
      },
    ];

    act(() => {
      root.render(
        <DockTranscript
          stream={handoffStream}
          active
          useRuntimeTimeline
          workspaceId="ws-1"
          compactAssistantProgress
        />,
      );
    });
    expect(container.textContent).toContain('The final answer is ready.');
    expect(container.querySelector('[data-agent-working-group]')).not.toBeNull();
    expect(container.textContent).not.toContain('Live');

    act(() => {
      root.render(
        <DockTranscript
          stream={handoffStream}
          active={false}
          useRuntimeTimeline
          workspaceId="ws-1"
          compactAssistantProgress
        />,
      );
    });

    expect(container.textContent).toContain('I am checking the source.');
    expect(container.textContent).toContain('The final answer is ready.');
    expect(container.querySelector('[data-working-group-active="true"]')).toBeNull();
    expect(container.textContent).not.toContain('Live');
  });

  it('keeps the same working group identity through the live-to-persisted handoff', () => {
    const liveStream = streamWithMessages([userMessage('user-1', 'Investigate it.', 1)]);
    liveStream.live_turn_segments = [
      assistantTurn('assistant-progress', 'I will inspect the conversation.'),
      toolTurn('tool-live', 'repository_search', 100, 'running'),
    ];
    act(() => {
      root.render(
        <DockTranscript
          stream={liveStream}
          active
          workspaceId="ws-1"
          compactAssistantProgress
        />,
      );
    });
    const liveGroup = container.querySelector('[data-working-group-id="work:tool-live"]');
    expect(liveGroup).not.toBeNull();
    expect(liveGroup?.getAttribute('data-working-group-id')).toBe('work:tool-live');
    expect(liveGroup?.querySelector('button')?.getAttribute('aria-expanded')).toBe('true');

    const persistedMessage = {
      ...assistantMessage('assistant-persisted', '', 2),
      turn_segments: [
        assistantTurn('assistant-progress', 'I will inspect the conversation.'),
        toolTurn('tool-live', 'repository_search', 100),
        assistantTurn('assistant-final', 'The final finding.'),
      ],
    };
    act(() => {
      root.render(
        <DockTranscript
          stream={streamWithMessages([userMessage('user-1', 'Investigate it.', 1), persistedMessage])}
          active={false}
          workspaceId="ws-1"
          compactAssistantProgress
        />,
      );
    });
    const persistedGroup = container.querySelector('[data-working-group-id="work:tool-live"]');
    expect(persistedGroup?.getAttribute('data-working-group-id')).toBe('work:tool-live');
    expect(persistedGroup?.querySelector('button')?.getAttribute('aria-expanded')).toBe('false');
    expect(container.textContent).toContain('The final finding.');
  });

  it('auto-collapses the previous live group when the next group starts', () => {
    const firstStream = streamWithMessages([userMessage('user-1', 'Investigate it.', 1)]);
    firstStream.live_turn_segments = [
      assistantTurn('progress-1', 'Checking the conversation.'),
      toolTurn('tool-1', 'list_conversation_messages', 100, 'running'),
    ];
    act(() => {
      root.render(<DockTranscript stream={firstStream} active workspaceId="ws-1" compactAssistantProgress />);
    });
    expect(container.querySelector('[data-agent-working-group] button')?.getAttribute('aria-expanded')).toBe('true');

    const nextStream = streamWithMessages([userMessage('user-1', 'Investigate it.', 1)]);
    nextStream.live_turn_segments = [
      assistantTurn('progress-1', 'Checking the conversation.'),
      toolTurn('tool-1', 'list_conversation_messages', 100),
      assistantTurn('progress-2', 'Checking the repository.'),
      toolTurn('tool-2', 'repository_search', 100, 'running'),
    ];
    act(() => {
      root.render(<DockTranscript stream={nextStream} active workspaceId="ws-1" compactAssistantProgress />);
    });

    const toggles = container.querySelectorAll('[data-agent-working-group] > button');
    expect(toggles).toHaveLength(2);
    expect(toggles[0]?.getAttribute('aria-expanded')).toBe('false');
    expect(toggles[1]?.getAttribute('aria-expanded')).toBe('true');
  });

  it('allows multiple completed working groups to remain manually expanded', () => {
    const message = {
      ...assistantMessage('assistant-tools', '', 2),
      turn_segments: [
        assistantTurn('progress-1', 'Checking the conversation.'),
        toolTurn('tool-1', 'list_conversation_messages', 100),
        assistantTurn('progress-2', 'Checking the repository.'),
        toolTurn('tool-2', 'repository_search', 100),
        assistantTurn('final', 'Done.'),
      ],
    };
    act(() => {
      root.render(<DockTranscript stream={streamWithMessages([message])} active={false} workspaceId="ws-1" compactAssistantProgress />);
    });

    const toggles = container.querySelectorAll('[data-agent-working-group] > button');
    act(() => {
      (toggles[0] as HTMLButtonElement).click();
      (toggles[1] as HTMLButtonElement).click();
    });
    const toolToggles = container.querySelectorAll('[data-agent-working-group] button[aria-expanded="false"]');
    act(() => {
      toolToggles.forEach((toggle) => (toggle as HTMLButtonElement).click());
    });
    expect(toggles[0]?.getAttribute('aria-expanded')).toBe('true');
    expect(toggles[1]?.getAttribute('aria-expanded')).toBe('true');
    expect(container.textContent).toContain('Checking the conversation.');
    expect(container.textContent).toContain('Checking the repository.');
    expect(container.querySelectorAll('[data-tool-call-details]')).toHaveLength(0);
  });

  it('shows every assistant message by default for full run views', () => {
    act(() => {
      root.render(
        <DockTranscript
          stream={streamWithMessages([
            assistantMessage('assistant-1', 'First progress update.', 1),
            assistantMessage('assistant-2', 'Second progress update.', 2),
          ])}
          active={false}
          workspaceId="ws-1"
        />,
      );
    });

    expect(container.textContent).toContain('First progress update.');
    expect(container.textContent).toContain('Second progress update.');
  });

  it('never collapses the latest assistant message', () => {
    const longOlderMessage = `Older response ${'old '.repeat(180)}`;
    const longLatestMessage = `Latest response ${'new '.repeat(180)}`;

    act(() => {
      root.render(
        <DockTranscript
          stream={streamWithMessages([
            assistantMessage('assistant-1', longOlderMessage, 1),
            assistantMessage('assistant-2', longLatestMessage, 2),
          ])}
          active={false}
          workspaceId="ws-1"
        />,
      );
    });

    const assistantMessages = container.querySelectorAll('.group\\/assistant');
    expect(assistantMessages).toHaveLength(2);
    expect(assistantMessages[0]?.textContent).toContain('Show more');
    expect(assistantMessages[1]?.textContent).not.toContain('Show more');
    expect(assistantMessages[1]?.textContent).not.toContain('Show less');
  });

  it('places sub-agent runs before messages sent after their launch', () => {
    act(() => {
      root.render(
        <DockTranscript
          stream={streamWithMessages([
            assistantMessage('assistant-1', 'I will delegate this work.', 1),
            userMessage('user-2', 'Did you finish it?', 5, 'user-1'),
          ])}
          active={false}
          workspaceId="ws-1"
          subAgentRuns={[
            {
              id: 'plan-1',
              createdAt: '2026-08-06T00:00:03Z',
              runCount: 1,
              content: <div>Forge · Create the deal · Failed</div>,
            },
          ]}
        />,
      );
    });

    const text = container.textContent ?? '';
    expect(text).toContain('Sub-agent runs');
    expect(text.indexOf('I will delegate this work.')).toBeLessThan(text.indexOf('Sub-agent runs'));
    expect(text.indexOf('Sub-agent runs')).toBeLessThan(text.indexOf('Did you finish it?'));
  });

  it('groups launches in the same timeline position under one counted heading', () => {
    act(() => {
      root.render(
        <DockTranscript
          stream={streamWithMessages([
            assistantMessage('assistant-1', 'Delegating now.', 1),
            userMessage('user-2', 'What happened?', 5, 'user-1'),
          ])}
          active={false}
          workspaceId="ws-1"
          subAgentRuns={[
            { id: 'plan-2', createdAt: '2026-08-06T00:00:03Z', runCount: 1, content: <div>Lens run</div> },
            { id: 'plan-1', createdAt: '2026-08-06T00:00:02Z', runCount: 1, content: <div>Forge run</div> },
          ]}
        />,
      );
    });

    expect(container.querySelectorAll('[data-agent-dock-sub-agent-runs]')).toHaveLength(1);
    expect(container.textContent).toContain('Sub-agent runs · 2');
    expect((container.textContent ?? '').indexOf('Forge run')).toBeLessThan((container.textContent ?? '').indexOf('Lens run'));
  });

  it('never places a settled attempt after its durable result position', () => {
    act(() => {
      root.render(
        <DockTranscript
          stream={streamWithMessages([
            assistantMessage('assistant-1', 'Launching Beacon.', 1),
            assistantMessage('assistant-3', 'Beacon failed to start.', 3),
          ])}
          active={false}
          workspaceId="ws-1"
          subAgentRuns={[{
            id: 'plan-1',
            createdAt: '2026-08-06T00:00:09Z',
            resultSequence: 2,
            runCount: 1,
            content: <div>Beacon · Create the deal · Failed to start</div>,
          }]}
        />,
      );
    });

    const text = container.textContent ?? '';
    expect(text.indexOf('Launching Beacon.')).toBeLessThan(text.indexOf('Sub-agent runs'));
    expect(text.indexOf('Sub-agent runs')).toBeLessThan(text.indexOf('Beacon failed to start.'));
  });

  it('uses the signed-in user\'s configured avatar for persisted messages', () => {
    act(() => {
      root.render(
        <DockTranscript
          stream={streamWithMessages([userMessage('user-1', 'Show my avatar.', 1, 'user-1')])}
          active={false}
          workspaceId="ws-1"
        />,
      );
    });

    expect(container.textContent).toContain('Alice Johnson');
    expect(mocks.resolveTeamMemberAvatarSrc).toHaveBeenCalledWith({
      avatarUrl: undefined,
      avatarStyle: 'personas',
      avatarSeed: 'alice-seed',
      avatarBackgroundMode: 'color',
      avatarBackgroundColor: '#fbbf24',
      fallbackSeed: 'Alice Johnson',
    });
  });

  it('shows the teammate who authored each persisted message', () => {
    act(() => {
      root.render(
        <DockTranscript
          stream={streamWithMessages([userMessage('user-2', 'I will take it from here.', 1, 'user-2')])}
          active={false}
          workspaceId="ws-1"
        />,
      );
    });

    expect(container.textContent).toContain('Bob Smith');
  });

  it('shows the teammate who approved an agent action', () => {
    const approval = {
      ...userMessage('approval-1', 'Approved the proposed changes.', 1),
      message_type: 'approval_request_resolution',
      resolver_user_id: 'user-2',
    };

    act(() => {
      root.render(
        <DockTranscript
          stream={streamWithMessages([approval])}
          active={false}
          workspaceId="ws-1"
        />,
      );
    });

    expect(container.textContent).toContain('Bob Smith approved');
  });

  it('groups adjacent successful calls without timing and keeps failures separate', () => {
    const message = {
      ...assistantMessage('assistant-tools', '', 1),
      turn_segments: [
        toolTurn('tool-1', 'browser_act', 1_200),
        toolTurn('tool-2', 'browser_act', 800),
        toolTurn('tool-failed', 'browser_act', 400, 'failed'),
        assistantTurn('assistant-break', 'I found the next step.'),
        toolTurn('tool-3', 'browser_act', 500),
        toolTurn('tool-4', 'browser_open', 300),
      ],
    };

    act(() => {
      root.render(<DockTranscript stream={streamWithMessages([message])} active={false} workspaceId="ws-1" />);
    });

    expect(container.textContent).toContain('browser_act ×2 · Browser act');
    expect(container.textContent).not.toContain('2s');
    expect(container.textContent?.match(/Browser act/g)).toHaveLength(3);
    expect(container.textContent).not.toContain('browser_act ×3');
    expect(container.textContent).toContain('Browser open');
  });
});
