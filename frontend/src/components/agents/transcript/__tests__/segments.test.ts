import { describe, expect, it } from 'vitest';

import {
  ALL_SEGMENT_KINDS,
  DOCK_SEGMENT_KINDS,
  collectSegments,
  hasRenderableSegments,
  type TranscriptStreamInput,
} from '../segments';
import type {
  CodingSessionLiveToolCall,
  CodingSessionLiveTurnSegment,
  CodingSessionTranscriptMessage,
} from '@/lib/pmTypes';

function toolCall(overrides: Partial<CodingSessionLiveToolCall> = {}): CodingSessionLiveToolCall {
  return {
    tool_call_id: 'tc-1',
    tool_name: 'read_file',
    args_text: '{"path":"a.ts"}',
    status: 'completed',
    ...overrides,
  };
}

function assistantSegment(
  id: string,
  content: string,
  status: 'streaming' | 'completed' = 'completed',
): CodingSessionLiveTurnSegment {
  return {
    segment_id: id,
    kind: 'assistant_message',
    assistant_message: { message_id: id, content, status, tool_calls: [] },
  };
}

function toolSegment(id: string, tc: CodingSessionLiveToolCall): CodingSessionLiveTurnSegment {
  return { segment_id: id, kind: 'tool_call', tool_call: tc };
}

function message(overrides: Partial<CodingSessionTranscriptMessage> = {}): CodingSessionTranscriptMessage {
  return {
    event_id: 'event-1',
    role: 'assistant',
    content: '',
    timestamp: '2026-06-15T00:00:00Z',
    sequence_no: 1,
    ...overrides,
  };
}

function stream(overrides: Partial<TranscriptStreamInput> = {}): TranscriptStreamInput {
  return {
    transcript_messages: [],
    live_turn_segments: [],
    live_reasoning_message: null,
    ...overrides,
  };
}

describe('collectSegments', () => {
  it('renders a user prompt as a user segment instead of hidden run context', () => {
    const segments = collectSegments(
      stream({
        transcript_messages: [message({
          event_id: 'user-prompt-1',
          role: 'user',
          message_type: 'prompt',
          content: 'Create the Kafka architecture document',
        })],
      }),
      { includeLive: false, include: DOCK_SEGMENT_KINDS },
    );

    expect(segments).toHaveLength(1);
    expect(segments[0]).toMatchObject({
      kind: 'user',
      message: { content: 'Create the Kafka architecture document', message_type: 'prompt' },
    });
  });

  it('interleaves assistant text and tool calls from turn_segments in order', () => {
    const segments = collectSegments(
      stream({
        transcript_messages: [message({
          turn_segments: [
            assistantSegment('a1', 'First'),
            toolSegment('t1', toolCall({ tool_call_id: 'tc-1' })),
            assistantSegment('a2', 'Second'),
          ],
        })],
      }),
      { includeLive: false },
    );

    expect(segments.map((s) => s.kind)).toEqual(['assistant', 'tool', 'assistant']);
    expect(segments[0]).toMatchObject({ kind: 'assistant', content: 'First' });
    expect(segments[2]).toMatchObject({ kind: 'assistant', content: 'Second' });
  });

  it('compacts assistant progress within a turn while preserving every tool', () => {
    const segments = collectSegments(
      stream({
        transcript_messages: [
          message({ event_id: 'user', role: 'user', content: 'Investigate this issue.', sequence_no: 1 }),
          message({
            event_id: 'assistant-turn',
            sequence_no: 2,
            turn_segments: [
              assistantSegment('a1', 'I will inspect the conversation.'),
              toolSegment('t1', toolCall({ tool_call_id: 'tc-conversation', tool_name: 'list_conversation_messages' })),
              assistantSegment('a2', 'Now I will inspect the implementation.'),
              toolSegment('t2', toolCall({ tool_call_id: 'tc-repository', tool_name: 'repository_search' })),
              assistantSegment('a3', 'The issue is caused by transcript pagination.'),
            ],
          }),
        ],
      }),
      { includeLive: false, compactAssistantProgress: true },
    );

    expect(segments.map((segment) => (
      segment.kind === 'assistant'
        ? segment.content
        : segment.kind === 'tool'
          ? segment.toolCall.tool_call_id
          : segment.kind
    ))).toEqual(['user', 'tc-conversation', 'tc-repository', 'The issue is caused by transcript pagination.']);
  });

  it('keeps the latest assistant response in every user turn', () => {
    const segments = collectSegments(
      stream({
        transcript_messages: [
          message({ event_id: 'u1', role: 'user', content: 'First question', sequence_no: 1 }),
          message({ event_id: 'a1-progress', content: 'Checking first question.', sequence_no: 2 }),
          message({ event_id: 'a1-final', content: 'First answer.', sequence_no: 3 }),
          message({ event_id: 'u2', role: 'user', content: 'Second question', sequence_no: 4 }),
          message({ event_id: 'a2-progress', content: 'Checking second question.', sequence_no: 5 }),
          message({ event_id: 'a2-final', content: 'Second answer.', sequence_no: 6 }),
        ],
      }),
      { includeLive: false, compactAssistantProgress: true },
    );

    expect(segments.map((segment) => (
      segment.kind === 'assistant' ? segment.content : `user:${segment.message.content}`
    ))).toEqual(['user:First question', 'First answer.', 'user:Second question', 'Second answer.']);
  });

  it('uses approval decisions as boundaries for cumulative live progress', () => {
    const segments = collectSegments(
      stream({
        transcript_messages: [
          message({ event_id: 'user', role: 'user', content: 'Create the document.', sequence_no: 1 }),
          message({ event_id: 'approval-prompt', message_id: 'approval-prompt', content: 'Approve creating the document?', sequence_no: 2 }),
          message({
            event_id: 'approval-decision',
            role: 'user',
            message_type: 'approval_request_resolution',
            content: 'Approved.',
            sequence_no: 3,
          }),
          message({ event_id: 'resumed-progress', message_id: 'resumed-progress', content: 'Creating the document now.', sequence_no: 4 }),
        ],
        live_turn_segments: [
          assistantSegment('approval-prompt', 'Approve creating the document?'),
          assistantSegment('resumed-progress', 'Creating the document now.'),
          toolSegment('create-tool', toolCall({ tool_call_id: 'create-tool', tool_name: 'create_document' })),
          assistantSegment('resumed-final', 'The document was created.', 'streaming'),
        ],
      }),
      { includeLive: true, compactAssistantProgress: true },
    );

    expect(segments.map((segment) => {
      if (segment.kind === 'assistant') return segment.content;
      if (segment.kind === 'tool') return segment.toolCall.tool_call_id;
      if (segment.kind === 'review_decision') return `decision:${segment.message.content}`;
      return `user:${segment.message.content}`;
    })).toEqual([
      'user:Create the document.',
      'Approve creating the document?',
      'decision:Approved.',
      'create-tool',
      'The document was created.',
    ]);
  });

  it.each([
    ['clarification', 'Which repository should I inspect?'],
    ['child launch', 'Started Lens. I will report back when it finishes.'],
    ['failed run', 'I could not finish because the provider disconnected.'],
    ['cancelled run', 'The run was cancelled before completion.'],
  ])('keeps the latest %s message visible', (_, finalMessage) => {
    const segments = collectSegments(
      stream({
        transcript_messages: [
          message({ event_id: 'progress', content: 'I am checking the workspace.', sequence_no: 1 }),
          message({ event_id: 'latest', content: finalMessage, sequence_no: 2 }),
        ],
      }),
      { includeLive: false, compactAssistantProgress: true },
    );

    expect(segments).toMatchObject([{ kind: 'assistant', content: finalMessage }]);
  });

  it('drops runtime-control tool calls (including mcp-prefixed names)', () => {
    const segments = collectSegments(
      stream({
        transcript_messages: [message({
          turn_segments: [
            toolSegment('t1', toolCall({ tool_call_id: 'tc-keep', tool_name: 'read_file' })),
            toolSegment('t2', toolCall({ tool_call_id: 'tc-drop', tool_name: 'mcp__helpin__update_plan' })),
            toolSegment('t3', toolCall({ tool_call_id: 'tc-finish', tool_name: 'finish_turn' })),
          ],
        })],
      }),
      { includeLive: false },
    );

    expect(segments).toHaveLength(1);
    expect(segments[0]).toMatchObject({ kind: 'tool' });
  });

  it('emits persisted tool_calls absent from the segment timeline, deduped by content', () => {
    const sharedArgs = '{"path":"frontend/src/App.tsx"}';
    const segments = collectSegments(
      stream({
        transcript_messages: [message({
          turn_segments: [
            assistantSegment('a1', 'Applying the change.'),
            toolSegment('t1', toolCall({ tool_call_id: 'seg-read', tool_name: 'read_file', args_text: sharedArgs })),
          ],
          tool_calls: [
            // Same content as the timeline read → deduped away.
            toolCall({ tool_call_id: 'fallback-read', tool_name: 'read_file', args_text: sharedArgs }),
            // Only persisted on the message → must still render.
            toolCall({ tool_call_id: 'fallback-patch', tool_name: 'apply_patch', args_text: '*** Begin Patch' }),
          ],
        })],
      }),
      { includeLive: false },
    );

    const toolIds = segments.filter((s) => s.kind === 'tool').map((s) => (s.kind === 'tool' ? s.toolCall.tool_call_id : ''));
    expect(toolIds).toEqual(['seg-read', 'fallback-patch']);
  });

  it('uses the persisted interleaved timeline instead of an earlier aggregate tool block', () => {
    const segments = collectSegments(
      stream({
        transcript_messages: [
          message({
            event_id: 'aggregate-tools',
            sequence_no: 1,
            tool_calls: [
              toolCall({
                tool_call_id: 'tool-list',
                tool_name: 'list_directory',
                args_text: '{"path":"src"}',
              }),
              toolCall({
                tool_call_id: 'tool-read',
                tool_name: 'read_files',
                args_text: '{"paths":["src/app.ts"]}',
              }),
            ],
          }),
          message({
            event_id: 'interleaved-turn',
            sequence_no: 2,
            turn_segments: [
              assistantSegment('assistant-first', 'I will inspect the repository.'),
              toolSegment('timeline-list', toolCall({
                tool_call_id: 'tool-list',
                tool_name: 'list_directory',
                args_text: '{"path":"src"}',
              })),
              assistantSegment('assistant-second', 'Now I will read the relevant file.'),
              toolSegment('timeline-read', toolCall({
                tool_call_id: 'tool-read',
                tool_name: 'read_files',
                args_text: '{"paths":["src/app.ts"]}',
              })),
              assistantSegment('assistant-final', 'The inspection is complete.'),
            ],
          }),
        ],
      }),
      { includeLive: false },
    );

    expect(segments.map((segment) => (
      segment.kind === 'assistant' ? segment.content : segment.toolCall.tool_call_id
    ))).toEqual([
      'I will inspect the repository.',
      'tool-list',
      'Now I will read the relevant file.',
      'tool-read',
      'The inspection is complete.',
    ]);
  });

  it('reconciles legacy aggregate tools by normalized arguments without collapsing repeats', () => {
    const segments = collectSegments(
      stream({
        transcript_messages: [
          message({
            event_id: 'legacy-aggregate',
            sequence_no: 1,
            tool_calls: [
              toolCall({
                tool_call_id: 'synthetic-list-1',
                tool_name: 'list_directory',
                args_text: '{\n  "depth": 2,\n  "path": "src"\n}',
              }),
              toolCall({
                tool_call_id: 'synthetic-list-2',
                tool_name: 'list_directory',
                args_text: '{"path":"src","depth":2}',
              }),
            ],
          }),
          message({
            event_id: 'legacy-timeline',
            sequence_no: 2,
            turn_segments: [
              assistantSegment('legacy-first', 'First pass.'),
              toolSegment('legacy-tool-1', toolCall({
                tool_call_id: 'runtime-list-1',
                tool_name: 'list_directory',
                args_text: '{"path":"src","depth":2}',
              })),
              assistantSegment('legacy-second', 'Second pass.'),
              toolSegment('legacy-tool-2', toolCall({
                tool_call_id: 'runtime-list-2',
                tool_name: 'list_directory',
                args_text: '{ "depth": 2, "path": "src" }',
              })),
            ],
          }),
        ],
      }),
      { includeLive: false },
    );

    expect(segments.map((segment) => (
      segment.kind === 'assistant' ? segment.content : segment.toolCall.tool_call_id
    ))).toEqual([
      'First pass.',
      'runtime-list-1',
      'Second pass.',
      'runtime-list-2',
    ]);
  });

  it('includes live reasoning + turn segments only when includeLive is true', () => {
    const input = stream({
      live_reasoning_message: { message_id: 'r1', content: 'thinking', status: 'streaming' },
      live_turn_segments: [{
        segment_id: 'live-a',
        kind: 'assistant_message',
        assistant_message: { message_id: 'live-a', content: 'Live answer', status: 'streaming', tool_calls: [] },
      }],
    });

    expect(collectSegments(input, { includeLive: false })).toHaveLength(0);

    const live = collectSegments(input, { includeLive: true });
    expect(live.map((s) => s.kind)).toEqual(['reasoning', 'assistant']);
    expect(live[1]).toMatchObject({ kind: 'assistant', content: 'Live answer', streaming: true });
  });

  it('retains runtime chronology without stale live styling after completion', () => {
    const input = stream({
      live_reasoning_message: { message_id: 'r1', content: 'thinking', status: 'streaming' },
      live_turn_segments: [
        toolSegment('live-tool', toolCall({ status: 'running' })),
        assistantSegment('live-a', 'Completed answer', 'streaming'),
      ],
    });

    const retained = collectSegments(input, { includeLive: true, runtimeActive: false });

    expect(retained).toMatchObject([
      { kind: 'reasoning', reasoning: { status: 'completed' } },
      { kind: 'tool', toolCall: { status: 'completed' } },
      { kind: 'assistant', content: 'Completed answer', streaming: false },
    ]);
  });

  it('marks only the latest active assistant segment as streaming', () => {
    const input = stream({
      live_turn_segments: [
        assistantSegment('live-a1', 'First streamed paragraph.', 'streaming'),
        toolSegment('live-tool', toolCall({
          tool_call_id: 'tc-live',
          tool_name: 'read_file',
          status: 'completed',
        })),
        assistantSegment('live-a2', 'Current streamed paragraph.', 'streaming'),
      ],
    });

    const assistants = collectSegments(input, { includeLive: true })
      .filter((segment) => segment.kind === 'assistant');

    expect(assistants).toMatchObject([
      { content: 'First streamed paragraph.', streaming: false },
      { content: 'Current streamed paragraph.', streaming: true },
    ]);
  });

  it('does not leave a text caret active while a later tool is running', () => {
    const input = stream({
      live_turn_segments: [
        assistantSegment('live-a1', 'I will inspect that now.', 'streaming'),
        toolSegment('live-tool', toolCall({
          tool_call_id: 'tc-live',
          tool_name: 'read_file',
          status: 'running',
        })),
      ],
    });

    const assistant = collectSegments(input, { includeLive: true })
      .find((segment) => segment.kind === 'assistant');

    expect(assistant).toMatchObject({
      kind: 'assistant',
      content: 'I will inspect that now.',
      streaming: false,
    });
  });

  it('includes live tool-call snapshot segments in the transcript', () => {
    const input = stream({
      live_turn_segments: [
        toolSegment('live-tool', toolCall({
          tool_call_id: 'tc-live',
          tool_name: 'fetch_url',
          args_text: '{"url":"https://example.com"}',
          status: 'completed',
        })),
      ],
    });

    const live = collectSegments(input, { includeLive: true });
    expect(live).toHaveLength(1);
    expect(live[0]).toMatchObject({
      kind: 'tool',
      toolCall: {
        tool_call_id: 'tc-live',
        tool_name: 'fetch_url',
      },
    });
  });

  it('does not repeat live assistant segments that are already persisted', () => {
    const input = stream({
      transcript_messages: [
        message({
          event_id: 'persisted-1',
          message_id: 'live-1',
          content: 'The frontend build has started.',
          timestamp: '2026-06-15T00:00:01Z',
          sequence_no: 1,
        }),
        message({
          event_id: 'persisted-2',
          message_id: 'live-2',
          content: 'The prebuild step completed successfully.',
          timestamp: '2026-06-15T00:00:02Z',
          sequence_no: 2,
        }),
      ],
      live_turn_segments: [
        assistantSegment('live-1', 'The frontend build has started.'),
        assistantSegment('live-2', 'The prebuild step completed successfully.'),
      ],
    });

    const segments = collectSegments(input, { includeLive: true });

    expect(segments.map((s) => (s.kind === 'assistant' ? s.content : s.kind))).toEqual([
      'The frontend build has started.',
      'The prebuild step completed successfully.',
    ]);
  });

  it('keeps earlier turns interleaved when the live timeline spans multiple chat turns', () => {
    // A long-lived chat run accumulates every assistant segment in
    // live_turn_segments, but user messages only exist in the transcript.
    // Segments from turns before the last user message must keep their
    // transcript position instead of being hoisted below all user messages.
    const input = stream({
      transcript_messages: [
        message({ event_id: 'u1', role: 'user', content: 'q1', timestamp: '2026-06-15T00:00:01Z', sequence_no: 1 }),
        message({ event_id: 'p1', message_id: 'a1', content: 'answer one', timestamp: '2026-06-15T00:00:02Z', sequence_no: 2 }),
        message({ event_id: 'u2', role: 'user', content: 'q2', timestamp: '2026-06-15T00:01:00Z', sequence_no: 3 }),
        message({ event_id: 'p2', message_id: 'a2', content: 'answer two', timestamp: '2026-06-15T00:01:05Z', sequence_no: 4 }),
      ],
      live_turn_segments: [
        assistantSegment('a1', 'answer one'),
        assistantSegment('a2', 'answer two'),
        assistantSegment('a3', 'streaming tail', 'streaming'),
      ],
    });

    const segments = collectSegments(input, { includeLive: true, include: DOCK_SEGMENT_KINDS });

    expect(segments.map((s) => (s.kind === 'user' ? `user:${s.message.content}` : (s as { content: string }).content))).toEqual([
      'user:q1',
      'answer one',
      'user:q2',
      'answer two',
      'streaming tail',
    ]);
  });

  it.each([
    'I have a web search result for the competitor digest.',
    'ffer has launched a new publishing workflow for teams.',
    'web search result for the competitor digest',
  ])('reconciles a live fragment by stable message identity: %s', (liveFragment) => {
    const persistedContent = liveFragment.startsWith('ffer')
      ? 'Buffer has launched a new publishing workflow for teams.'
      : 'I have a web search result for the competitor digest.';
    const segments = collectSegments(
      stream({
        transcript_messages: [message({
          event_id: 'persisted-complete',
          message_id: 'live-partial',
          content: persistedContent,
        })],
        live_turn_segments: [assistantSegment('live-partial', liveFragment, 'streaming')],
      }),
      { includeLive: true },
    );

    expect(segments).toEqual([{
      kind: 'assistant',
      id: 'persisted-complete',
      messageId: 'live-partial',
      content: persistedContent,
    }]);
  });

  it('uses the cumulative live timeline to keep persisted prose interleaved with tools', () => {
    const first = 'I will research competitor updates and prepare the digest.';
    const second = 'Buffer has launched a new publishing workflow for teams.';
    const segments = collectSegments(
      stream({
        transcript_messages: [
          message({ event_id: 'persisted-first', message_id: 'live-first', content: first, sequence_no: 1 }),
          message({ event_id: 'persisted-second', message_id: 'live-second', content: second, sequence_no: 2 }),
        ],
        live_turn_segments: [
          assistantSegment('live-first', first),
          toolSegment('live-crawl', toolCall({
            tool_call_id: 'runtime-crawl',
            tool_name: 'crawl_url',
            args_text: '{"url":"https://buffer.com/news"}',
          })),
          assistantSegment('live-second', 'ffer has launched a new publishing workflow for teams.'),
        ],
      }),
      { includeLive: true },
    );

    expect(segments.map((segment) => (
      segment.kind === 'assistant' ? segment.content : segment.toolCall.tool_name
    ))).toEqual([
      first,
      'crawl_url',
      second,
    ]);
    expect(segments.filter((segment) => segment.kind === 'assistant')).toHaveLength(2);
  });

  it('dedupes a persisted and live tool call by stable tool identity', () => {
    const persistedTool = toolCall({
      tool_call_id: 'runtime-native-id',
      tool_name: 'crawl_url',
      args_text: '{"url":"https://example.com"}',
      status: 'completed',
      result: { content: 'Fetched page', output_summary: 'Fetched page' },
    });
    const liveTool = toolCall({
      tool_call_id: 'runtime-native-id',
      tool_name: 'crawl_url',
      args_text: '{"url":"https://example.com"}',
      status: 'running',
    });
    const segments = collectSegments(
      stream({
        transcript_messages: [message({
          event_id: 'persisted-message',
          content: 'Researching the source.',
          tool_calls: [persistedTool],
        })],
        live_turn_segments: [
          assistantSegment('live-message', 'Researching the source.'),
          toolSegment('live-tool', liveTool),
        ],
      }),
      { includeLive: true },
    );

    const tools = segments.filter((segment) => segment.kind === 'tool');
    expect(tools).toHaveLength(1);
    expect(tools[0]).toMatchObject({
      kind: 'tool',
      id: 'runtime-native-id',
      toolCall: { status: 'completed' },
    });
  });

  it('keeps paused-run tools at their live timeline positions when persisted IDs are legacy', () => {
    const segments = collectSegments(
      stream({
        transcript_messages: [message({
          event_id: 'paused-aggregate',
          tool_calls: [toolCall({
            tool_call_id: 'synthetic-read',
            tool_name: 'read_files',
            args_text: '{\n  "paths": ["src/app.ts"],\n  "limit": 10\n}',
            status: 'completed',
            duration_ms: 27,
          })],
        })],
        live_turn_segments: [
          assistantSegment('paused-first', 'I will inspect the file.'),
          toolSegment('paused-read', toolCall({
            tool_call_id: 'runtime-read',
            tool_name: 'read_files',
            args_text: '{"limit":10,"paths":["src/app.ts"]}',
            status: 'running',
          })),
          assistantSegment('paused-final', 'The run is waiting for input.'),
        ],
      }),
      { includeLive: true },
    );

    expect(segments.map((segment) => (
      segment.kind === 'assistant' ? segment.content : segment.toolCall.tool_call_id
    ))).toEqual([
      'I will inspect the file.',
      'synthetic-read',
      'The run is waiting for input.',
    ]);
    expect(segments[1]).toMatchObject({
      kind: 'tool',
      toolCall: { status: 'completed', duration_ms: 27 },
    });
  });

  it('matches repeated identical assistant turns one-to-one without dropping either occurrence', () => {
    const repeated = 'The source was checked successfully.';
    const segments = collectSegments(
      stream({
        transcript_messages: [
          message({ event_id: 'persisted-1', message_id: 'live-1', content: repeated, sequence_no: 1 }),
          message({ event_id: 'persisted-2', message_id: 'live-2', content: repeated, sequence_no: 2 }),
        ],
        live_turn_segments: [
          assistantSegment('live-1', repeated),
          assistantSegment('live-2', repeated),
        ],
      }),
      { includeLive: true },
    );

    expect(segments).toMatchObject([
      { kind: 'assistant', id: 'persisted-1', content: repeated },
      { kind: 'assistant', id: 'persisted-2', content: repeated },
    ]);
  });

  it('does not fuzzy-match unrelated short replies', () => {
    const segments = collectSegments(
      stream({
        transcript_messages: [message({ event_id: 'persisted', content: 'Done and ready.' })],
        live_turn_segments: [assistantSegment('live', 'Done', 'streaming')],
      }),
      { includeLive: true },
    );

    expect(segments).toMatchObject([
      { kind: 'assistant', id: 'persisted', content: 'Done and ready.' },
      { kind: 'assistant', id: 'live:live', content: 'Done', streaming: true },
    ]);
  });

  it('scopes kinds via the include set (dock = user + assistant + tool)', () => {
    const input = stream({
      transcript_messages: [
        message({ message_type: 'status', role: 'assistant', content: 'Preparing workspace' }),
        message({ event_id: 'user-1', message_type: 'message', role: 'user', content: 'Create the architecture doc' }),
        message({ event_id: 'event-2', turn_segments: [assistantSegment('a1', 'Done')] }),
      ],
    });

    const all = collectSegments(input, { includeLive: false, include: ALL_SEGMENT_KINDS });
    expect(all.map((s) => s.kind)).toEqual(['status', 'user', 'assistant']);

    const dock = collectSegments(input, { includeLive: false, include: DOCK_SEGMENT_KINDS });
    expect(dock.map((s) => s.kind)).toEqual(['user', 'assistant']);
  });

  it('classifies status / context / user / review-decision messages', () => {
    const segments = collectSegments(
      stream({
        transcript_messages: [
          message({ event_id: 'launch', role: 'user', message_type: 'prompt', content: 'launch context' }),
          message({ event_id: 'ctx', role: 'user', message_type: 'developer_prompt', content: 'developer context' }),
          message({ event_id: 'status', message_type: 'status', content: 'Working' }),
          message({ event_id: 'user', role: 'user', message_type: 'message', content: 'hi' }),
          message({ event_id: 'review', role: 'user', message_type: 'approval_request_resolution', content: 'Approved.' }),
        ],
      }),
      { includeLive: false },
    );

    expect(segments.map((s) => s.kind)).toEqual(['user', 'context', 'status', 'user', 'review_decision']);
  });

  it('prepends leadingContext when context is in scope', () => {
    const segments = collectSegments(
      stream({ transcript_messages: [message({ turn_segments: [assistantSegment('a1', 'Hi')] })] }),
      {
        includeLive: false,
        leadingContext: message({ event_id: 'lead', message_type: 'system_prompt', content: 'system' }),
      },
    );

    expect(segments[0]).toMatchObject({ kind: 'context', id: 'context:lead' });
    expect(segments[1]).toMatchObject({ kind: 'assistant' });
  });
});

describe('hasRenderableSegments', () => {
  it('is false for a null stream and true once a segment exists', () => {
    expect(hasRenderableSegments(null, { includeLive: true })).toBe(false);
    expect(
      hasRenderableSegments(
        stream({ transcript_messages: [message({ turn_segments: [assistantSegment('a1', 'Hi')] })] }),
        { includeLive: false, include: DOCK_SEGMENT_KINDS },
      ),
    ).toBe(true);
  });
});
