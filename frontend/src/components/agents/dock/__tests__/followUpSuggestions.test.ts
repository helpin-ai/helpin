import { describe, expect, it } from 'vitest';
import { parseFollowUpSuggestions } from '../followUpSuggestions';

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
