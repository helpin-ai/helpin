import { describe, expect, it } from 'vitest';
import { resolveDockComposerState, transformDockStream } from '../dockChatState';
import { parseDockChildResult, parseDockPlanConfirm, stripDockPageContext } from '@/lib/dockTypes';
import type { CodingSessionStreamState } from '@/lib/pmTypes';

function emptyStream(): CodingSessionStreamState {
  return {
    transcript_messages: [],
    live_assistant_message: null,
    live_reasoning_message: null,
    live_turn_segments: [],
    activity_events: [],
    current_plan: null,
    completed_tool_calls: [],
  };
}

describe('resolveDockComposerState', () => {
  it('enables the composer for a chat with no run yet', () => {
    const state = resolveDockComposerState(null, false, false);
    expect(state.visible).toBe(true);
    expect(state.enabled).toBe(true);
  });

  it('enables replies while paused awaiting the user', () => {
    const state = resolveDockComposerState({ status: 'paused', pause_reason: 'awaiting_user_message' }, false, false);
    expect(state.enabled).toBe(true);
  });

  it('disables while the agent is working', () => {
    const state = resolveDockComposerState({ status: 'running', pause_reason: 'none' }, false, false);
    expect(state.visible).toBe(true);
    expect(state.enabled).toBe(false);
  });

  it('hides during a structured interaction', () => {
    const state = resolveDockComposerState({ status: 'paused', pause_reason: 'human_approval' }, true, false);
    expect(state.visible).toBe(false);
  });

  it('enables continuation after the run ended', () => {
    const state = resolveDockComposerState({ status: 'completed', pause_reason: 'none' }, false, false);
    expect(state.enabled).toBe(true);
  });
});

describe('transformDockStream', () => {
  it('strips page context from user messages and extracts child results', () => {
    const stream = emptyStream();
    stream.transcript_messages = [
      {
        event_id: 'e1',
        role: 'user',
        content: 'what is up?\n\n<page_context>{"entity_type":"task"}</page_context>',
        timestamp: 't',
        sequence_no: 1,
      },
      {
        event_id: 'e2',
        role: 'user',
        content: '<child_run_result>{"plan_id":"p1","status":"completed","runs":[]}</child_run_result>',
        timestamp: 't',
        sequence_no: 2,
      },
      { event_id: 'e3', role: 'assistant', content: 'done', timestamp: 't', sequence_no: 3 },
    ];
    const result = transformDockStream(stream);
    expect(result.stream.transcript_messages).toHaveLength(2);
    expect(result.stream.transcript_messages[0].content).toBe('what is up?');
    expect(result.childResults).toHaveLength(1);
    expect(result.childResults[0].result.plan_id).toBe('p1');
  });
});

describe('dock marker parsing', () => {
  it('stripDockPageContext leaves plain content unchanged', () => {
    expect(stripDockPageContext('hello')).toBe('hello');
  });

  it('parseDockChildResult rejects non-result content', () => {
    expect(parseDockChildResult('hello')).toBeNull();
    expect(parseDockChildResult('<child_run_result>not json</child_run_result>')).toBeNull();
  });

  it('parseDockPlanConfirm requires the dock kind', () => {
    expect(parseDockPlanConfirm({ kind: 'other' })).toBeNull();
    expect(parseDockPlanConfirm({ kind: 'dock_plan_confirm', summary: 's' })?.summary).toBe('s');
  });
});
