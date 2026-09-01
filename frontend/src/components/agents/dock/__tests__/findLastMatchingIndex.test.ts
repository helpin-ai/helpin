import { describe, expect, it } from 'vitest';
import { findLastMatchingIndex } from '../findLastMatchingIndex';

describe('findLastMatchingIndex', () => {
  it('returns the final matching index without requiring ES2023 array methods', () => {
    expect(findLastMatchingIndex(['user', 'assistant', 'user'], (role) => role === 'user')).toBe(2);
  });

  it('returns -1 when no item matches', () => {
    expect(findLastMatchingIndex(['assistant'], (role) => role === 'user')).toBe(-1);
  });
});
