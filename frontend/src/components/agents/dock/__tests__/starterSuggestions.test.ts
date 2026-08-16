import { describe, expect, it } from 'vitest';
import { starterSuggestionsForContext } from '../starterSuggestions';

describe('starterSuggestionsForContext', () => {
  it('offers the most common support actions', () => {
    expect(starterSuggestionsForContext('support_conversation')).toEqual([
      {
        label: 'Draft a reply',
        prompt: 'Review this support conversation and draft a helpful, accurate reply to the customer. Flag anything that needs clarification before sending.',
      },
      {
        label: 'Investigate the issue',
        prompt: 'Investigate the customer issue using this conversation. Identify the likely cause, supporting evidence, and the best next step.',
      },
      {
        label: 'Summarize next steps',
        prompt: 'Summarize this conversation, including key facts, unresolved questions, and the recommended next steps.',
      },
    ]);
  });

  it('uses source-specific actions for documents', () => {
    expect(starterSuggestionsForContext('document')).toEqual([
      expect.objectContaining({ label: 'Summarize document' }),
      expect.objectContaining({ label: 'Improve clarity' }),
      expect.objectContaining({ label: 'Find gaps' }),
    ]);
  });

  it('falls back to workspace actions without a source context', () => {
    expect(starterSuggestionsForContext()).toEqual([
      expect.objectContaining({ label: 'Prioritize work' }),
      expect.objectContaining({ label: 'Find relevant work' }),
      expect.objectContaining({ label: 'Investigate an issue' }),
    ]);
  });
});
