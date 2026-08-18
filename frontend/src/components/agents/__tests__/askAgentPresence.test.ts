import { describe, expect, it } from 'vitest';

import { deriveAskAgentAvatarState, isAskAgentSpeaking } from '@/components/agents/askAgentPresence';
import type { CodingSessionLiveTurnSegment } from '@/lib/pmTypes';

function assistantSegment(
  content: string,
  status: 'streaming' | 'completed' = 'streaming',
): CodingSessionLiveTurnSegment {
  return {
    segment_id: 'assistant-segment',
    kind: 'assistant_message',
    assistant_message: {
      message_id: 'assistant-message',
      content,
      status,
      tool_calls: [],
    },
  };
}

describe('Ask Agent presence', () => {
  it('maps the lifecycle to idle, thinking, speaking, and error', () => {
    expect(deriveAskAgentAvatarState({})).toBe('idle');
    expect(deriveAskAgentAvatarState({ run: { status: 'paused' } })).toBe('idle');
    expect(deriveAskAgentAvatarState({ run: { status: 'queued' } })).toBe('thinking');
    expect(deriveAskAgentAvatarState({ run: { status: 'running' } })).toBe('thinking');
    expect(deriveAskAgentAvatarState({ sending: true })).toBe('thinking');
    expect(deriveAskAgentAvatarState({ run: { status: 'failed' } })).toBe('error');
    expect(deriveAskAgentAvatarState({ error: 'message failed' })).toBe('error');
  });

  it('speaks only while the latest live segment is non-empty streaming prose', () => {
    const speakingStream = { live_turn_segments: [assistantSegment('Here is the answer')] };
    expect(isAskAgentSpeaking(speakingStream)).toBe(true);
    expect(deriveAskAgentAvatarState({ run: { status: 'running' }, stream: speakingStream })).toBe('speaking');

    expect(isAskAgentSpeaking({ live_turn_segments: [assistantSegment('')] })).toBe(false);
    expect(isAskAgentSpeaking({ live_turn_segments: [assistantSegment('Done', 'completed')] })).toBe(false);
  });

  it('gives errors priority over stale streaming state', () => {
    expect(deriveAskAgentAvatarState({
      run: { status: 'failed' },
      stream: { live_turn_segments: [assistantSegment('Stale partial answer')] },
    })).toBe('error');
  });
});
