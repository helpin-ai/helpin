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
  it('compacts progress prose while retaining tools and the latest response when enabled', () => {
    const message = {
      ...assistantMessage('assistant-tools', '', 2),
      turn_segments: [
        assistantTurn('assistant-progress-1', 'I will inspect the conversation.'),
        toolTurn('tool-1', 'list_conversation_messages', 100),
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

    expect(container.textContent).not.toContain('I will inspect the conversation.');
    expect(container.textContent).not.toContain('Now I will inspect the repository.');
    expect(container.textContent).toContain('Conversation Messages');
    expect(container.textContent).toContain('Repository Search');
    expect(container.textContent).toContain('The pagination state is not advancing.');
  });

  it('keeps the same compacted result through the live-to-persisted handoff', () => {
    const liveStream = streamWithMessages([userMessage('user-1', 'Investigate it.', 1)]);
    liveStream.live_turn_segments = [
      assistantTurn('assistant-progress', 'I will inspect the conversation.'),
      toolTurn('tool-live', 'repository_search', 100),
      assistantTurn('assistant-final', 'The final finding.'),
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
    expect(container.textContent).not.toContain('I will inspect the conversation.');
    expect(container.textContent).toContain('Repository Search');
    expect(container.textContent).toContain('The final finding.');

    const persistedMessage = {
      ...assistantMessage('assistant-persisted', '', 2),
      turn_segments: liveStream.live_turn_segments,
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
    expect(container.textContent).not.toContain('I will inspect the conversation.');
    expect(container.textContent).toContain('Repository Search');
    expect(container.textContent).toContain('The final finding.');
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

  it('groups adjacent successful calls, sums duration, and keeps failures separate', () => {
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

    expect(container.textContent).toContain('Browser Act × 2 (browser_act)');
    expect(container.textContent).toContain('2s');
    expect(container.textContent?.match(/Browser Act/g)).toHaveLength(3);
    expect(container.textContent).not.toContain('Browser Act x 3');
    expect(container.textContent).toContain('Browser Open');
  });
});
