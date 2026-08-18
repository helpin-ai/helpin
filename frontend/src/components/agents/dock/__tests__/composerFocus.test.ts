// @vitest-environment jsdom
import { describe, expect, it, vi } from 'vitest';
import { focusComposerAtEnd } from '../composerFocus';

describe('focusComposerAtEnd', () => {
  it('focuses the composer and places the caret after an inserted suggestion', () => {
    const textarea = document.createElement('textarea');
    const focus = vi.spyOn(textarea, 'focus');
    textarea.value = 'Investigate the issue and recommend next steps.';

    focusComposerAtEnd(textarea, textarea.value);

    expect(focus).toHaveBeenCalledOnce();
    expect(textarea.selectionStart).toBe(textarea.value.length);
    expect(textarea.selectionEnd).toBe(textarea.value.length);
  });
});
