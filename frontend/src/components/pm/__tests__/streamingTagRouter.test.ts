import { describe, expect, it } from 'vitest';

import { INITIAL_SEGMENTS, StreamingTagRouter } from '../streamingTagRouter';

function router() {
  return new StreamingTagRouter();
}

describe('StreamingTagRouter', () => {
  it('routes plain text to chatText', () => {
    const r = router();
    const seg = r.feed('Hello, world!');
    expect(seg.chatText).toBe('Hello, world!');
    expect(seg.isThinking).toBe(false);
  });

  it('routes <thinking> block to thinkingText', () => {
    const r = router();
    const s1 = r.feed('Let me think...\n<thinking>');
    expect(s1.chatText).toBe('Let me think...\n');
    expect(s1.isThinking).toBe(true);

    const s2 = r.feed('Analyzing the problem...');
    expect(s2.thinkingText).toBe('Analyzing the problem...');
    expect(s2.isThinking).toBe(true);

    const s3 = r.feed('</thinking>\nHere is my answer.');
    expect(s3.thinkingText).toBe('Analyzing the problem...');
    expect(s3.isThinking).toBe(false);
    expect(s3.chatText).toBe('Let me think...\n\nHere is my answer.');
  });

  it('handles multiple sequential thinking tags', () => {
    const r = router();
    const seg = r.feed('A<thinking>thought</thinking>B');
    expect(seg.chatText).toBe('AB');
    expect(seg.thinkingText).toBe('thought');
    expect(seg.isThinking).toBe(false);
  });

  it('flushes false-alarm < as chat text', () => {
    const r = router();
    const seg = r.feed('a < b and <span>ok</span>');
    expect(seg.chatText).toBe('a < b and <span>ok</span>');
  });

  it('finalize() flushes pending buffer as chat text', () => {
    const r = router();
    r.feed('Hello <sp');
    const seg = r.finalize();
    expect(seg.chatText).toBe('Hello <sp');
  });

  it('reset() clears all state', () => {
    const r = router();
    r.feed('text<thinking>content');
    r.reset();
    const seg = r.getSegments();
    expect(seg).toEqual(INITIAL_SEGMENTS);
  });

  it('handles very small deltas (char by char)', () => {
    const r = router();
    const input = 'Hi<thinking>t</thinking>bye';
    let seg;
    for (const ch of input) {
      seg = r.feed(ch);
    }
    expect(seg!.chatText).toBe('Hibye');
    expect(seg!.thinkingText).toBe('t');
    expect(seg!.isThinking).toBe(false);
  });

  it('does not confuse unrelated closing tags inside thinking', () => {
    const r = router();
    const seg = r.feed('<thinking>text </story_plan> more</thinking>rest');
    expect(seg.thinkingText).toBe('text </story_plan> more');
    expect(seg.chatText).toBe('rest');
    expect(seg.isThinking).toBe(false);
  });
});
