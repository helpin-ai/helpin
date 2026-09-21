// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import type {
  CodingSessionLiveTurnSegment,
  CodingSessionStreamState,
  CodingSessionTranscriptMessage,
} from '@/lib/pmTypes';
import { useDockStore } from '@/stores/dockStore';
import { useAuthStore } from '@/stores/authStore';
import { buildCodingSessionStreamState } from '@/components/pm/CodingSession/codingSessionStream';
import type { CodingSessionEvent } from '@/lib/pmTypes';
import { resolveAgentLiveProgress } from '../agentProgress';
import { AgentLiveStatus } from '../AgentLiveStatus';
import { DockTranscript } from '../DockTranscript';

const mocks = vi.hoisted(() => ({
  resolveTeamMemberAvatarSrc: vi.fn(),
  getMessageWorkDetail: vi.fn(),
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

vi.mock('@/lib/services/dockChatService', () => ({
  dockChatService: {
    getMessageWorkDetail: mocks.getMessageWorkDetail,
  },
}));

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

let container: HTMLDivElement;
let root: Root;

beforeEach(() => {
  useDockStore.setState({ transcriptView: 'detailed' });
  mocks.resolveTeamMemberAvatarSrc.mockReset();
  mocks.getMessageWorkDetail.mockReset();
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
  argsText = '{}',
): CodingSessionLiveTurnSegment {
  return {
    segment_id: id,
    kind: 'tool_call',
    tool_call: {
      tool_call_id: id,
      tool_name: toolName,
      args_text: argsText,
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
  it('keeps an anchored plan above a follow-up and collapses the earlier plan', () => {
    const stream = streamWithMessages([userMessage('first', 'Initial request', 1), assistantMessage('answer', 'First response', 3), userMessage('followup', 'Next request', 4)]);
    stream.current_plan = {origin:{event_id:'plan-event',turn_id:'turn-1',created_at:'2026-08-06T00:00:02Z'},plan:[{step:'Inspect code',status:'completed'}]};
    act(() => root.render(<DockTranscript stream={stream} active={false} workspaceId="ws-1" />));
    const plan = container.querySelector('[data-coding-session-plan]')!;
    expect(plan).not.toBeNull();
    expect(plan.hasAttribute('open')).toBe(false);
    expect(container.textContent!.indexOf('Work plan')).toBeLessThan(container.textContent!.indexOf('Next request'));
    expect(container.textContent!.indexOf('Initial request')).toBeLessThan(container.textContent!.indexOf('Work plan'));
  });

  it('collapses already-loaded standalone run work and keeps the final response visible', () => {
    act(() => {
      root.render(
        <DockTranscript
          stream={streamWithMessages([
            userMessage('user-1', 'Investigate this.', 1),
            assistantMessage('progress-1', 'I will inspect the repository.', 2),
            assistantMessage('final-1', 'The issue is in the event sorter.', 9),
          ])}
          active={false}
          workspaceId="ws-1"
          compactAssistantProgress
          completedRun
        />,
      );
    });

    expect(container.textContent).toContain('Worked for 8s');
    expect(container.textContent).toContain('The issue is in the event sorter.');
    expect(container.textContent).not.toContain('I will inspect the repository.');
    expect(mocks.getMessageWorkDetail).not.toHaveBeenCalled();

    const disclosure = Array.from(container.querySelectorAll('button'))
      .find((button) => button.textContent?.includes('Worked for 8s'))!;
    expect(disclosure.getAttribute('aria-expanded')).toBe('false');
    act(() => disclosure.click());
    expect(container.textContent).toContain('I will inspect the repository.');
  });

  it('loads completed turn work only when Worked for is expanded and reuses it', async () => {
    mocks.getMessageWorkDetail.mockResolvedValue({
      data: {
        messages: [{
          id: 'assistant-final',
          workspace_id: 'ws-1',
          run_id: 'run-1',
          dock_chat_id: 'chat-1',
          role: 'assistant',
          content: 'Final answer.',
          message_type: 'assistant_turn',
          sequence_no: 2,
          created_at: '2026-08-06T00:00:09Z',
          turn_segments: [
            assistantTurn('progress', 'Checking the repository.'),
            toolTurn('tool-1', 'read_file', 8000),
            assistantTurn('final', 'Final answer.'),
          ],
        }],
      },
      error: null,
    });
    const summary = {
      ...assistantMessage('work:assistant-final', '', 2),
      message_type: 'status',
      dock_work_summary: { message_id: 'assistant-final', duration_ms: 9000, activity_count: 2 },
    } as CodingSessionTranscriptMessage;

    act(() => {
      root.render(
        <DockTranscript
          stream={streamWithMessages([
            userMessage('user-1', 'Investigate this.', 1),
            summary,
            assistantMessage('assistant-final', 'Final answer.', 3),
          ])}
          active={false}
          workspaceId="ws-1"
          chatId="chat-1"
          compactAssistantProgress
        />,
      );
    });

    expect(container.textContent).toContain('Worked for 9s');
    expect(container.textContent).toContain('Final answer.');
    expect(container.textContent).not.toContain('Checking the repository.');
    expect(mocks.getMessageWorkDetail).not.toHaveBeenCalled();

    const disclosure = Array.from(container.querySelectorAll('button'))
      .find((button) => button.textContent?.includes('Worked for 9s'))!;
    expect(disclosure.textContent).not.toContain('▸');
    expect(disclosure.querySelector('[data-disclosure-chevron]')).not.toBeNull();
    await act(async () => {
      disclosure.click();
      await Promise.resolve();
      await Promise.resolve();
    });

    expect(mocks.getMessageWorkDetail).toHaveBeenCalledWith('ws-1', 'chat-1', 'assistant-final');
    expect(container.textContent).toContain('Checking the repository.');
    expect(container.textContent).toContain('read_file');

    await act(async () => {
      disclosure.click();
      disclosure.click();
      await Promise.resolve();
    });
    expect(mocks.getMessageWorkDetail).toHaveBeenCalledTimes(1);
  });

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
        toolTurn('tool-1b', 'repository_search', 100),
        assistantTurn('assistant-progress-2', 'Now I will inspect the repository.'),
        toolTurn('tool-2', 'repository_search', 100),
        toolTurn('tool-2b', 'read_files', 100),
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
    expect(groups[0]?.querySelector('button')?.lastElementChild)
      .toBe(groups[0]?.querySelector('[data-disclosure-chevron]'));
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
          toolTurn('tool-conversation-extra', 'read_files', 100),
          assistantTurn('progress-2', 'Now I will inspect the documentation.'),
          toolTurn('tool-docs', 'search_documents', 100),
          toolTurn('tool-docs-extra', 'read_files', 100),
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
      toolTurn('tool-1b', 'read_files', 100, 'running'),
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
      toolTurn('tool-live-extra', 'read_files', 100, 'running', '{"files":[{"path":"src/ChatView.tsx"}]}'),
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
    expect(liveGroup?.querySelector('button')?.getAttribute('aria-expanded')).toBe('false');
    expect(liveGroup?.textContent).toContain('Read src/ChatView.tsx');
    expect(liveGroup?.textContent).toContain('1 previous');
    expect(liveGroup?.className).not.toContain('rounded-lg');
    expect(liveGroup?.className).not.toContain('bg-muted');

    const persistedMessage = {
      ...assistantMessage('assistant-persisted', '', 2),
      turn_segments: [
        assistantTurn('assistant-progress', 'I will inspect the conversation.'),
        toolTurn('tool-live', 'repository_search', 100),
        toolTurn('tool-live-extra', 'read_files', 100),
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

  it('keeps tool history collapsed while the contextual latest action advances', () => {
    const firstStream = streamWithMessages([userMessage('user-1', 'Investigate it.', 1)]);
    firstStream.live_turn_segments = [
      assistantTurn('progress-1', 'Checking the conversation.'),
      toolTurn('tool-1', 'list_conversation_messages', 100, 'running'),
      toolTurn('tool-1b', 'read_files', 100, 'running', '{"files":[{"path":"src/conversation.ts"}]}'),
    ];
    act(() => {
      root.render(<DockTranscript stream={firstStream} active workspaceId="ws-1" compactAssistantProgress />);
    });
    const firstGroup = container.querySelector('[data-agent-working-group]');
    expect(firstGroup?.querySelector('button')?.getAttribute('aria-expanded')).toBe('false');
    expect(firstGroup?.textContent).toContain('Read src/conversation.ts');

    const nextStream = streamWithMessages([userMessage('user-1', 'Investigate it.', 1)]);
    nextStream.live_turn_segments = [
      assistantTurn('progress-1', 'Checking the conversation.'),
      toolTurn('tool-1', 'list_conversation_messages', 100),
      toolTurn('tool-1b', 'read_files', 100),
      assistantTurn('progress-2', 'Checking the repository.'),
      toolTurn('tool-2', 'repository_search', 100, 'running'),
      toolTurn('tool-2b', 'read_files', 100, 'running', '{"files":[{"path":"src/repository.ts"}]}'),
    ];
    act(() => {
      root.render(<DockTranscript stream={nextStream} active workspaceId="ws-1" compactAssistantProgress />);
    });

    const toggles = container.querySelectorAll('[data-agent-working-group] > button');
    expect(toggles).toHaveLength(2);
    expect(toggles[0]?.getAttribute('aria-expanded')).toBe('false');
    expect(toggles[1]?.getAttribute('aria-expanded')).toBe('false');
    expect(toggles[1]?.textContent).toContain('Read src/repository.ts');
  });

  it('allows multiple completed working groups to remain manually expanded', () => {
    const message = {
      ...assistantMessage('assistant-tools', '', 2),
      turn_segments: [
        assistantTurn('progress-1', 'Checking the conversation.'),
        toolTurn('tool-1', 'list_conversation_messages', 100),
        toolTurn('tool-1b', 'read_files', 100),
        assistantTurn('progress-2', 'Checking the repository.'),
        toolTurn('tool-2', 'repository_search', 100),
        toolTurn('tool-2b', 'read_files', 100),
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

  it('shows the sender below the persisted message without an avatar', () => {
    act(() => {
      root.render(
        <DockTranscript
          stream={streamWithMessages([userMessage('user-1', 'Show my message.', 1, 'user-1')])}
          active={false}
          workspaceId="ws-1"
        />,
      );
    });

    expect(container.textContent).toContain('Alice Johnson');
    expect(mocks.resolveTeamMemberAvatarSrc).not.toHaveBeenCalled();
    expect(container.querySelector('[data-message-sender]')?.textContent).toContain('Alice Johnson');
    expect(container.querySelector('[data-message-sender] time')?.getAttribute('datetime')).toBe('2026-08-06T00:00:01Z');
    expect(container.querySelector('[data-message-sender] time')?.textContent).toBe(new Date('2026-08-06T00:00:01Z').toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' }));
    expect(container.textContent!.indexOf('Show my message.')).toBeLessThan(container.textContent!.indexOf('Alice Johnson'));
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

    expect(container.textContent).toContain('Browser Act ×2 · browser_act');
    expect(container.textContent).not.toContain('2s');
    expect(container.textContent?.match(/Browser Act/g)).toHaveLength(3);
    expect(container.textContent).not.toContain('browser_act ×3');
    expect(container.textContent).toContain('Browser Open');
  });
});


describe('follow-up with retained live history', () => {
  it.each(['pending', 'sent'] as const)('keeps the earlier answer above the %s follow-up and the new answer below it', async (deliveryStatus) => {
    const stream = streamWithMessages([
      userMessage('user-first', 'First question', 1),
      { ...userMessage('client:follow-up', 'Follow-up question', 3), client_message_id: 'follow-up', delivery_status: deliveryStatus },
    ]);
    stream.live_turn_segments = [
      { kind: 'assistant_message', segment_id: 'earlier-answer', assistant_message: {
        message_id: 'earlier-answer', content: 'Earlier answer', status: 'completed', tool_calls: [], started_at: '2026-08-06T00:00:02Z',
      } },
      { kind: 'assistant_message', segment_id: 'new-answer', assistant_message: {
        message_id: 'new-answer', content: 'New answer', status: 'streaming', tool_calls: [], started_at: '2026-08-06T00:00:04Z',
      } },
    ];
    await act(async () => root.render(<DockTranscript stream={stream} active useRuntimeTimeline />));
    const text = container.textContent ?? '';
    expect(text.indexOf('First question')).toBeLessThan(text.indexOf('Earlier answer'));
    expect(text.indexOf('Earlier answer')).toBeLessThan(text.indexOf('Follow-up question'));
    expect(text.indexOf('Follow-up question')).toBeLessThan(text.indexOf('New answer'));
  });
});


describe('follow-up after timestamp-less live completion', () => {
  it.each(['pending', 'sent'] as const)('preserves pre-submit history for the %s row without moving a new reply above it', async (deliveryStatus) => {
    const stream = streamWithMessages([
      userMessage('user-first', 'First question', 1),
      { ...userMessage('client:follow-up', 'Follow-up question', 3), client_message_id: 'follow-up', delivery_status: deliveryStatus },
    ]);
    stream.live_turn_segments = ['Earlier answer', 'New answer'].map((content) => ({
      kind: 'assistant_message', segment_id: content, assistant_message: {
        message_id: content, content, status: 'completed', tool_calls: [],
      },
    }));
    await act(async () => root.render(<DockTranscript stream={stream} active useRuntimeTimeline
      latestSubmission={{ clientMessageId: 'follow-up', precedingLiveSegmentIds: new Set(['live:Earlier answer']) }} />));
    const text = container.textContent ?? '';
    expect(text.indexOf('First question')).toBeLessThan(text.indexOf('Earlier answer'));
    expect(text.indexOf('Earlier answer')).toBeLessThan(text.indexOf('Follow-up question'));
    expect(text.indexOf('Follow-up question')).toBeLessThan(text.indexOf('New answer'));
  });
});


describe('explicit turn delivery in the rendered dock', () => {
  const start = '2026-09-11T12:00:00Z';
  const preamble = "Done — verified both pages. Here's what changed.";
  const answer = Array.from({ length: 6 }, (_, i) => `### ${i + 1}. Document ${i + 1}\n\nMermaid, nwdiag, and Excalidraw details for document ${i + 1}.`).join('\n\n');
  const event = (sequence: number, type: string, payload: Record<string, unknown> = {}): CodingSessionEvent => ({
    id: `event-${sequence}`, session_id: 'session', run_id: 'run', sequence_no: sequence,
    type, runtime_kind: 'native_sdk', timestamp: new Date(Date.parse(start) + sequence * 1000).toISOString(),
    payload: { completion_mode: 'explicit', turn_id: 'turn-1', turn_started_at: start, ...payload },
  });
  const progress = event(1, 'assistant.message.completed', { message_id: 'preamble', message_type: 'assistant_progress', content: preamble });
  const final = event(20, 'assistant.message.completed', { message_id: 'answer', message_type: 'assistant_final', content: answer });
  const render = (events: CodingSessionEvent[], active = true) => {
    const stream = buildCodingSessionStreamState(events);
    const status = resolveAgentLiveProgress({ run: { status: active ? 'running' : 'paused', pause_reason: 'awaiting_user_message', started_at: '2026-09-11T10:40:00Z', created_at: start }, stream, currentPlan: null, sending: false });
    act(() => root.render(<><DockTranscript stream={stream} active={active} useRuntimeTimeline compactAssistantProgress />{status && <AgentLiveStatus progress={status} />}</>));
    return { stream, status };
  };

  it('keeps the preamble as progress, displays all six sections immediately, and freezes work before cleanup', () => {
    const first = render([progress]);
    expect(first.status?.label).toBe('Working…');
    expect(first.status?.startedAt).toBe(start);
    expect(container.querySelector('[data-assistant-presentation="final"]')).toBeNull();
    expect(container.querySelector('[data-assistant-presentation="progress"]')?.textContent).toContain(preamble);
    const accepted = render([progress, final]);
    expect(accepted.status).toBeNull();
    const finalRow = container.querySelector('[data-assistant-presentation="final"]');
    expect(finalRow?.textContent).toContain('6. Document 6');
    expect(container.querySelectorAll('[data-assistant-presentation="final"]')).toHaveLength(1);
    const worked = container.querySelector('[data-working-group-label]')?.textContent;
    expect(worked).toContain('Worked');
    const paused = render([progress, final, event(40, 'run.paused', { pause_reason: 'awaiting_user_message' })], false);
    expect(paused.status).toBeNull();
    expect(container.querySelector('[data-working-group-label]')?.textContent).toBe(worked);
    expect(container.querySelector('[data-assistant-presentation="final"]')?.textContent).toBe(finalRow?.textContent);
  });

  it('never presents a settled explicit preamble as a successful answer', () => {
    const { status } = render([progress, event(40, 'run.paused', { pause_reason: 'awaiting_user_message' })], false);
    expect(status?.label).toBe('Run ended without a final answer');
    expect(container.querySelector('[data-assistant-presentation="final"]')).toBeNull();
  });

  it('does not let a duplicate old final stop a resumed turn', () => {
    const resumed = event(50, 'run.resumed', { turn_id: 'turn-2', turn_started_at: '2026-09-11T12:00:50Z' });
    const { stream, status } = render([progress, final, resumed, { ...final, id: 'late', sequence_no: 51 }]);
    expect(stream.turn_state).toMatchObject({ turn_id: 'turn-2', phase: 'working' });
    expect(status?.label).toBe('Working…');
    expect(status?.startedAt).toBe('2026-09-11T12:00:50Z');
  });
});

describe('Timeline view', () => {
  it.each([
    ['awaiting_user_message', 'Activity'],
    ['human_input', 'Needs your input'],
    ['human_approval', 'Waiting for approval'],
    ['authentication', 'Waiting for sign-in'],
  ] as const)('labels a paused %s turn accurately', (pauseReason, label) => {
    useDockStore.setState({ transcriptView: 'timeline' });
    const progress = { ...assistantMessage('progress', 'I will add a paragraph.', 1), message_type: 'assistant_progress' };
    act(() => root.render(<DockTranscript stream={streamWithMessages([progress])} active={false}
      compactAssistantProgress runStatus="paused" pauseReason={pauseReason} />));
    expect(container.querySelector('[data-dock-activity-timeline] > button')?.textContent).toBe(label);
    if (pauseReason === 'awaiting_user_message') expect(container.textContent).not.toContain('Needs your input');
  });

  it('shows completed work without a caret or an interactive control when there are no steps', () => {
    useDockStore.setState({ transcriptView: 'timeline' });
    const summary = { ...assistantMessage('work:empty-final', '', 2), message_type: 'status',
      dock_work_summary: { message_id: 'empty-final', duration_ms: 9000, activity_count: 0 } };
    act(() => root.render(<DockTranscript stream={streamWithMessages([
      summary, { ...assistantMessage('empty-final', 'Done.', 3), message_type: 'assistant_final' },
    ])} active={false} workspaceId="ws-1" chatId="chat-1" compactAssistantProgress
      runStatus="paused" pauseReason="awaiting_user_message" />));
    const completion = container.querySelector('[data-dock-work-disclosure]');
    expect(completion?.textContent).toContain('Work completed');
    expect(completion?.querySelector('button, [data-disclosure-chevron], [aria-expanded]')).toBeNull();
    expect(mocks.getMessageWorkDetail).not.toHaveBeenCalled();
    expect(container.textContent).not.toContain('Needs your input');
    expect(container.textContent).toContain('Done.');
  });

  it('shows live activity with the branded loader and collapses it when the final answer arrives', () => {
    useDockStore.setState({ transcriptView: 'timeline' });
    const progress = { ...assistantMessage('progress', 'Reviewing your tasks.', 1), message_type: 'assistant_progress' };
    const answer = { ...assistantMessage('answer', 'Three tasks need attention.', 2), message_type: 'assistant_final' };
    act(() => root.render(<DockTranscript stream={streamWithMessages([progress])} active compactAssistantProgress />));
    expect(container.querySelector('[data-agent-work-loader]')).not.toBeNull();
    expect(container.querySelector('[aria-label="Activity steps"]')?.textContent).toContain('Reviewing your tasks.');
    act(() => root.render(<DockTranscript stream={streamWithMessages([progress, answer])} active={false} compactAssistantProgress />));
    expect(container.textContent).toContain('Work completed');
    expect(container.textContent).toContain('Three tasks need attention.');
    expect(container.textContent).not.toContain('Reviewing your tasks.');
    act(() => container.querySelector<HTMLButtonElement>('[data-dock-activity-timeline] > button')?.click());
    expect(container.textContent).toContain('Reviewing your tasks.');
  });

  it('preserves manual collapse during streaming and reveals failed steps when work stops', () => {
    useDockStore.setState({ transcriptView: 'timeline' });
    const progress = { ...assistantMessage('progress', 'Reviewing your tasks.', 1), message_type: 'assistant_progress' };
    const tool = toolTurn('search', 'search_tasks', 100, 'running');
    const render = (active: boolean) => act(() => root.render(<DockTranscript
      stream={streamWithMessages([progress, { ...assistantMessage('tools', '', 2), turn_segments: [tool] }])}
      active={active} compactAssistantProgress runStatus={active ? 'running' : 'failed'} />));
    render(true);
    act(() => container.querySelector<HTMLButtonElement>('[data-dock-activity-timeline] > button')?.click());
    render(true);
    expect(container.querySelector('[aria-label="Activity steps"]')).toBeNull();
    if (tool.kind !== 'tool_call') throw new Error('Expected a tool');
    tool.tool_call.status = 'failed';
    tool.tool_call.result = { error: 'Task search is unavailable.' };
    render(false);
    expect(container.textContent).toContain('Some steps failed');
    expect(container.textContent).toContain('Task search is unavailable.');
    expect(container.querySelector('[data-agent-work-loader]')).toBeNull();
  });

  it('keeps an expanded tool row mounted as live execution is saved', () => {
    useDockStore.setState({ transcriptView: 'timeline' });
    const live = streamWithMessages([]);
    live.live_turn_segments = [toolTurn('search', 'search_tasks', 100, 'running', '{"query":"sprint"}')];
    act(() => root.render(<DockTranscript stream={live} active useRuntimeTimeline compactAssistantProgress />));
    const button = container.querySelector<HTMLButtonElement>('[aria-label="Activity steps"] button')!;
    act(() => button.click());
    expect(button.getAttribute('aria-expanded')).toBe('true');
    const saved = streamWithMessages([{ ...assistantMessage('tools', '', 1), turn_segments: [toolTurn('search', 'search_tasks', 200, 'completed', '{"query":"sprint"}')] }]);
    act(() => root.render(<DockTranscript stream={saved} active useRuntimeTimeline compactAssistantProgress />));
    expect(container.querySelector('[aria-label="Activity steps"] button')).toBe(button);
    expect(button.getAttribute('aria-expanded')).toBe('true');
    expect(container.textContent).toContain('{"query":"sprint"}');
  });

  it('changes presentation without losing messages and persists the preference', () => {
    useDockStore.setState({ transcriptView: 'timeline' });
    const progress = { ...assistantMessage('progress', 'Reviewing your tasks.', 1), message_type: 'assistant_progress' };
    act(() => root.render(<DockTranscript stream={streamWithMessages([progress])} active compactAssistantProgress />));
    expect(container.querySelector('[data-dock-activity-timeline]')).not.toBeNull();
    act(() => useDockStore.getState().setTranscriptView('detailed'));
    expect(container.querySelector('[data-dock-activity-timeline]')).toBeNull();
    expect(container.textContent).toContain('Reviewing your tasks.');
    expect(localStorage.getItem('helpin:agent-dock-transcript-view')).toBe('detailed');
    act(() => useDockStore.getState().setTranscriptView('timeline'));
    expect(container.querySelector('[data-dock-activity-timeline]')).not.toBeNull();
  });
});
