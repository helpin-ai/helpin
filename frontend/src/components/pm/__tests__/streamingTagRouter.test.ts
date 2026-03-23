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
    expect(seg.isStreamingSpecDraft).toBe(false);
  });

  it('routes complete <spec_draft> block in a single chunk', () => {
    const r = router();
    const seg = r.feed('Intro\n<spec_draft>\n# PRD\nBody\n</spec_draft>\nOutro');
    expect(seg.chatText).toBe('Intro\n\nOutro');
    expect(seg.specDraftText).toBe('\n# PRD\nBody\n');
    expect(seg.isStreamingSpecDraft).toBe(false);
  });

  it('detects opening tag and starts streaming to artifact', () => {
    const r = router();
    const s1 = r.feed('Before text\n<spec_draft>');
    expect(s1.chatText).toBe('Before text\n');
    expect(s1.isStreamingSpecDraft).toBe(true);
    expect(s1.specDraftText).toBe('');

    const s2 = r.feed('# Draft content');
    expect(s2.specDraftText).toBe('# Draft content');
    expect(s2.isStreamingSpecDraft).toBe(true);
    expect(s2.chatText).toBe('Before text\n');

    const s3 = r.feed('\nMore content</spec_draft>\nAfter');
    expect(s3.specDraftText).toBe('# Draft content\nMore content');
    expect(s3.isStreamingSpecDraft).toBe(false);
    expect(s3.chatText).toBe('Before text\n\nAfter');
  });

  it('handles partial tag at chunk boundary', () => {
    const r = router();
    const s1 = r.feed('Hello <spec_');
    // '<spec_' is held in pending buffer, 'Hello ' is in chat
    expect(s1.chatText).toBe('Hello ');

    const s2 = r.feed('draft>content here');
    expect(s2.chatText).toBe('Hello ');
    expect(s2.isStreamingSpecDraft).toBe(true);
    expect(s2.specDraftText).toBe('content here');
  });

  it('handles partial closing tag at chunk boundary', () => {
    const r = router();
    r.feed('<spec_draft>content</spec_');
    // '</spec_' held in pending
    const s2 = r.feed('draft>after');
    expect(s2.specDraftText).toBe('content');
    expect(s2.isStreamingSpecDraft).toBe(false);
    expect(s2.chatText).toBe('after');
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

  it('routes <questions> block to questionsText', () => {
    const r = router();
    const seg = r.feed('I have some questions.\n<questions><question id="q1" text="What?"><option value="a">A</option></question></questions>\nThanks.');
    expect(seg.chatText).toBe('I have some questions.\n\nThanks.');
    expect(seg.questionsText).toContain('question');
    expect(seg.isStreamingQuestions).toBe(false);
  });

  it('routes <story_plan> to storyPlanText', () => {
    const r = router();
    const s1 = r.feed('<story_plan>{"summary":"A plan"}');
    expect(s1.isStreamingStoryPlan).toBe(true);
    expect(s1.storyPlanText).toBe('{"summary":"A plan"}');

    const s2 = r.feed('</story_plan>');
    expect(s2.isStreamingStoryPlan).toBe(false);
    expect(s2.storyPlanText).toBe('{"summary":"A plan"}');
  });

  it('handles multiple sequential tags', () => {
    const r = router();
    const seg = r.feed('A<thinking>thought</thinking>B<spec_draft>draft</spec_draft>C');
    expect(seg.chatText).toBe('ABC');
    expect(seg.thinkingText).toBe('thought');
    expect(seg.specDraftText).toBe('draft');
    expect(seg.isThinking).toBe(false);
    expect(seg.isStreamingSpecDraft).toBe(false);
  });

  it('flushes false-alarm < as chat text', () => {
    const r = router();
    const seg = r.feed('a < b and <span>ok</span>');
    expect(seg.chatText).toBe('a < b and <span>ok</span>');
  });

  it('handles <approval_request> with body', () => {
    const r = router();
    const seg = r.feed('Review.\n<approval_request phase="prd"><title>PRD</title></approval_request>\nDone.');
    expect(seg.chatText).toBe('Review.\n\nDone.');
    expect(seg.approvalRequestText).toContain('<title>PRD</title>');
    expect(seg.isStreamingApproval).toBe(false);
  });

  it('handles self-closing <approval_request />', () => {
    const r = router();
    const seg = r.feed('Review.\n<approval_request phase="prd" title="PRD approval" summary="Ready" />\nDone.');
    expect(seg.chatText).toBe('Review.\n\nDone.');
    expect(seg.approvalRequestText).toContain('phase="prd"');
    expect(seg.isStreamingApproval).toBe(false);
  });

  it('finalize() flushes pending buffer as chat text', () => {
    const r = router();
    r.feed('Hello <sp');
    const seg = r.finalize();
    expect(seg.chatText).toBe('Hello <sp');
    expect(seg.isStreamingSpecDraft).toBe(false);
  });

  it('finalize() flushes pending buffer inside a tag as tag content', () => {
    const r = router();
    r.feed('<spec_draft>content</sp');
    const seg = r.finalize();
    expect(seg.specDraftText).toBe('content</sp');
    expect(seg.isStreamingSpecDraft).toBe(false);
  });

  it('reset() clears all state', () => {
    const r = router();
    r.feed('text<spec_draft>content');
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

  it('does not confuse </spec_draft> inside another tag', () => {
    const r = router();
    // If inside thinking, </spec_draft> is just content
    const seg = r.feed('<thinking>text </spec_draft> more</thinking>rest');
    expect(seg.thinkingText).toBe('text </spec_draft> more');
    expect(seg.chatText).toBe('rest');
    expect(seg.isThinking).toBe(false);
  });
});
