import { describe, expect, it, vi } from 'vitest';
import { getGravatarUrl } from '../gravatar';

describe('getGravatarUrl', () => {
  it('normalizes email and uses the documented SHA-256 identifier', async () => {
    expect(await getGravatarUrl(' MyEmailAddress@example.com ')).toBe(
      'https://www.gravatar.com/avatar/84059b07d4be67b806386c0aad8070a23f18836bbaae342275dc0a83414c32ee?s=128&d=404&r=g',
    );
  });

  it.each([undefined, null, '', '   ', 'not-an-email'])('skips missing or invalid email %s', async (email) => {
    expect(await getGravatarUrl(email)).toBeUndefined();
  });

  it('keeps the initials fallback when Web Crypto is unavailable', async () => {
    vi.stubGlobal('crypto', undefined);
    try {
      expect(await getGravatarUrl('person@example.com')).toBeUndefined();
    } finally {
      vi.unstubAllGlobals();
    }
  });
});
