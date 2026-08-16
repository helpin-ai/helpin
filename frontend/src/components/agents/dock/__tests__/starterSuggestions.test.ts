import { describe, expect, it } from 'vitest';
import { starterSuggestionsForContext } from '../starterSuggestions';

describe('starterSuggestionsForContext', () => {
  it('offers the most common support actions', () => {
    expect(starterSuggestionsForContext('support_conversation')).toEqual([
      'Draft a reply to the customer',
      'Investigate the issue and likely cause',
      'Summarize the conversation and recommend next steps',
    ]);
  });

  it('uses source-specific actions for documents', () => {
    expect(starterSuggestionsForContext('document')).toEqual([
      'Summarize this document',
      'Improve this document for clarity',
      'Find gaps and recommend updates',
    ]);
  });

  it('falls back to workspace actions without a source context', () => {
    expect(starterSuggestionsForContext()).toEqual([
      'Help me prioritize the next work',
      'Find relevant work or documentation',
      'Investigate an issue',
    ]);
  });
});
