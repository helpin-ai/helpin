import { describe, expect, it } from 'vitest';
import { parseFollowUpSuggestions, stripFollowUpSuggestions, chatFollowUpSuggestions } from '../followUpSuggestions';

describe('parseFollowUpSuggestions', () => {
  it('parses, trims, deduplicates, and limits agent suggestions', () => {
    const content = `Answer\n\n<!-- helpin_follow_up_suggestions\n[" Compare this with the previous run ", "compare this with the previous run", "Create a fix", "One", "Fourth"]\n-->`;
    expect(parseFollowUpSuggestions(content)).toEqual([
      'Compare this with the previous run',
      'Create a fix',
      'Fourth',
    ]);
  });

  it('ignores malformed or absent markers', () => {
    expect(parseFollowUpSuggestions('Answer')).toEqual([]);
    expect(parseFollowUpSuggestions('<!-- helpin_follow_up_suggestions nope -->')).toEqual([]);
  });
});

const marker = '<!-- helpin_follow_up_suggestions ["Review the task", "Draft a reply"] -->';

describe('chat follow-up presentation', () => {
  it('removes complete and malformed control markers without removing ordinary comments', () => {
    expect(stripFollowUpSuggestions(`Answer\n\n${marker}`)).toBe('Answer');
    expect(stripFollowUpSuggestions('Answer\n<!-- helpin_follow_up_suggestions nope -->')).toBe('Answer');
    expect(stripFollowUpSuggestions('Answer <!-- ordinary comment -->')).toBe('Answer <!-- ordinary comment -->');
  });
  it('never exposes a partially streamed control marker', () => {
    for (let i = 4; i <= marker.length; i++) {
      expect(stripFollowUpSuggestions(`Answer\n\n${marker.slice(0, i)}`)).toBe('Answer');
    }
  });
  const messages = [
    { role: 'user', content: 'Help me' },
    { role: 'assistant', message_type: 'assistant_final', content: `Done\n${marker}` },
  ];
  it.each([
    ['completed', '', true], ['paused', 'awaiting_user_message', true],
    ['running', '', false], ['paused', 'human_approval', false], ['paused', 'human_input', false],
    ['paused', 'authentication', false], ['failed', '', false], ['cancelled', '', false],
  ] as const)('handles chat state %s/%s', (status, pause_reason, expected) => {
    expect(chatFollowUpSuggestions({ status, pause_reason }, messages)).toEqual(expected ? ['Review the task', 'Draft a reply'] : []);
  });
  it('does not reuse suggestions from an earlier turn or progress message', () => {
    const run = { status: 'paused', pause_reason: 'awaiting_user_message' };
    expect(chatFollowUpSuggestions(run, [...messages, { role: 'user', content: 'Next question' }])).toEqual([]);
    expect(chatFollowUpSuggestions(run, [...messages, { role: 'user', content: 'Next question' }, { role: 'assistant', message_type: 'assistant_progress', content: marker }])).toEqual([]);
    expect(chatFollowUpSuggestions(run, [...messages, { role: 'assistant', content: 'No next steps' }])).toEqual([]);
  });
});
