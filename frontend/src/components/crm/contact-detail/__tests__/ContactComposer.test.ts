import { describe, expect, it } from 'vitest';

import {
  contactComposerModeBarClassName,
  contactComposerSaveBarClassName,
  contactComposerTextareaClassName,
} from '../ContactComposer';

describe('ContactComposer', () => {
  it('places note, call, and meeting tabs above the composer body', () => {
    expect(contactComposerModeBarClassName).toContain('rounded-t-xl');
    expect(contactComposerModeBarClassName).toContain('border-b');
    expect(contactComposerTextareaClassName).not.toContain('rounded-t-xl');
    expect(contactComposerSaveBarClassName).toContain('border-t');
  });
});
