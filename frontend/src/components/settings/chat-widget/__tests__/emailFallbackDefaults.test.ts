import { describe, expect, it } from 'vitest';
import { DEFAULT_EMAIL_FALLBACK_DELAY_SECS } from '../constants';

describe('email fallback defaults', () => {
  it('defaults the undo window to ten seconds', () => {
    expect(DEFAULT_EMAIL_FALLBACK_DELAY_SECS).toBe(10);
  });
});
