import { describe, expect, it } from 'vitest';
import { canClearDockContext, composerPlaceholderForContext, composerTextareaHeight, contextChipMaxWidth, sendControlClassName, usesSeparateComposerActionRow } from '../DockInput';

describe('composerTextareaHeight', () => {
  it('keeps an empty composer to one line even when its placeholder measures taller', () => {
    expect(composerTextareaHeight({ value: '', scrollHeight: 64, singleLineHeight: 28 })).toBe(28);
  });

  it('uses the measured height after the user enters multiline text', () => {
    expect(composerTextareaHeight({ value: 'A message that wraps', scrollHeight: 64, singleLineHeight: 28 })).toBe(64);
  });
});

describe('composerPlaceholderForContext', () => {
  it('uses a concise prompt in a general chat', () => {
    expect(composerPlaceholderForContext()).toBe('Message agent…');
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

describe('contextChipMaxWidth', () => {
  it('reserves room for Add context beside the active context', () => {
    expect(contextChipMaxWidth(true)).toBe('calc(100% - 116px)');
  });

  it('uses the normal chip width when no additional context can be added', () => {
    expect(contextChipMaxWidth(false)).toBeUndefined();
  });
});

describe('usesSeparateComposerActionRow', () => {
  it('keeps conversation actions below even an empty message', () => {
    expect(usesSeparateComposerActionRow('conversation')).toBe(true);
    expect(usesSeparateComposerActionRow('list')).toBe(false);
  });
});

describe('sendControlClassName', () => {
  it('uses the shared black primary treatment when send is enabled', () => {
    expect(sendControlClassName(false)).toContain('bg-foreground');
    expect(sendControlClassName(false)).not.toContain('bg-orange-500');
  });
});
