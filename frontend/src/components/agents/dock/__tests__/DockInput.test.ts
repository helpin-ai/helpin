import { describe, expect, it } from 'vitest';
import { canClearDockContext, composerPlaceholderForContext, sendControlClassName, shouldUseExpandedComposerLayout, usesSeparateComposerActionRow } from '../DockInput';

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

describe('canClearDockContext', () => {
  it('allows a supplied support conversation context to be removed', () => {
    expect(canClearDockContext('support_conversation', true)).toBe(true);
  });

  it('does not show a remove control when no clear action was provided', () => {
    expect(canClearDockContext('support_conversation', false)).toBe(false);
  });
});

describe('usesSeparateComposerActionRow', () => {
  it('separates attachment and send controls only for multiline chat input', () => {
    expect(usesSeparateComposerActionRow('conversation', true)).toBe(true);
    expect(usesSeparateComposerActionRow('conversation', false)).toBe(false);
    expect(usesSeparateComposerActionRow('list', true)).toBe(false);
  });
});

describe('sendControlClassName', () => {
  it('uses the shared black primary treatment when send is enabled', () => {
    expect(sendControlClassName(false)).toContain('bg-foreground');
    expect(sendControlClassName(false)).not.toContain('bg-orange-500');
  });
});
