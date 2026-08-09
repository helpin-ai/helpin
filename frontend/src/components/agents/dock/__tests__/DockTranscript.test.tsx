// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import type { CodingSessionStreamState, CodingSessionTranscriptMessage } from '@/lib/pmTypes';
import { useAuthStore } from '@/stores/authStore';
import { DockTranscript } from '../DockTranscript';

const mocks = vi.hoisted(() => ({
  resolveTeamMemberAvatarSrc: vi.fn(),
}));

vi.mock('@/lib/teamMemberAvatar', () => ({
  resolveTeamMemberAvatarSrc: mocks.resolveTeamMemberAvatarSrc,
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

function userMessage(id: string, content: string, sequenceNo: number): CodingSessionTranscriptMessage {
  return {
    event_id: id,
    message_id: id,
    role: 'user',
    message_type: 'message',
    content,
    timestamp: `2026-08-06T00:00:0${sequenceNo}Z`,
    sequence_no: sequenceNo,
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

describe('DockTranscript', () => {
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
        />,
      );
    });

    const assistantMessages = container.querySelectorAll('.group\\/assistant');
    expect(assistantMessages).toHaveLength(2);
    expect(assistantMessages[0]?.textContent).toContain('Show more');
    expect(assistantMessages[1]?.textContent).not.toContain('Show more');
    expect(assistantMessages[1]?.textContent).not.toContain('Show less');
  });

  it('uses the signed-in user\'s configured avatar for persisted messages', () => {
    act(() => {
      root.render(
        <DockTranscript
          stream={streamWithMessages([userMessage('user-1', 'Show my avatar.', 1)])}
          active={false}
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
});
