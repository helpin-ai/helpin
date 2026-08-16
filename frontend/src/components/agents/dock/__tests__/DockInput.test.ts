import { describe, expect, it } from 'vitest';
import { composerPlaceholderForContext, shouldUseExpandedComposerLayout } from '../DockInput';

describe('shouldUseExpandedComposerLayout', () => {
  it('moves wrapped text above the composer actions', () => {
    expect(shouldUseExpandedComposerLayout({
      value: 'A message that wraps',
      scrollHeight: 48,
      singleLineHeight: 28,
      currentlyExpanded: false,
    })).toBe(true);
  });

  it('keeps the expanded layout stable while the user edits', () => {
    expect(shouldUseExpandedComposerLayout({
      value: 'Shortened message',
      scrollHeight: 28,
      singleLineHeight: 28,
      currentlyExpanded: true,
    })).toBe(true);
  });

  it('returns to the inline layout after the composer is cleared', () => {
    expect(shouldUseExpandedComposerLayout({
      value: '   ',
      scrollHeight: 28,
      singleLineHeight: 28,
      currentlyExpanded: true,
    })).toBe(false);
  });
});

describe('composerPlaceholderForContext', () => {
  it('describes both asking and delegating in a general chat', () => {
    expect(composerPlaceholderForContext()).toBe('Ask a question or delegate work to agents…');
  });

  it('uses the attached conversation as the support prompt', () => {
    expect(composerPlaceholderForContext('support_conversation')).toBe('Ask about this conversation…');
  });
});
