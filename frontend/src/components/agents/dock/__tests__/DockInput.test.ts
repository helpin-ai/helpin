import { describe, expect, it } from 'vitest';
import { shouldUseExpandedComposerLayout } from '../DockInput';

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
