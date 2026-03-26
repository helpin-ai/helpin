import { describe, expect, it } from 'vitest';
import { getGoogleFaviconUrl, normalizeFaviconHost } from '@/lib/favicon';

describe('normalizeFaviconHost', () => {
  it('normalizes bare domains by adding https before parsing', () => {
    expect(normalizeFaviconHost('helpin.ai')).toBe('helpin.ai');
  });

  it('strips www from fully qualified URLs', () => {
    expect(normalizeFaviconHost('https://www.example.com/docs')).toBe('example.com');
  });

  it('preserves non-www subdomains', () => {
    expect(normalizeFaviconHost('https://docs.example.com')).toBe('docs.example.com');
  });

  it('returns an empty string for invalid input', () => {
    expect(normalizeFaviconHost('')).toBe('');
    expect(normalizeFaviconHost('not a url ???')).toBe('');
  });
});

describe('getGoogleFaviconUrl', () => {
  it('builds a Google s2 favicon URL from the normalized host', () => {
    expect(getGoogleFaviconUrl('https://www.example.com/path', 32)).toBe(
      'https://www.google.com/s2/favicons?domain=example.com&sz=32',
    );
  });

  it('returns an empty string when no host can be resolved', () => {
    expect(getGoogleFaviconUrl(undefined)).toBe('');
  });
});
