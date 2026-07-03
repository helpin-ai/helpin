import { describe, expect, it } from 'vitest';

import {
  contactHeaderAvatarClassName,
  contactHeaderLifecycleBadgeClassName,
  contactHeaderNameClassName,
  getContactHeaderSubtitleParts,
} from '../ContactHeader';

describe('ContactHeader', () => {
  it('uses a Clarify-like contact header hierarchy', () => {
    expect(contactHeaderAvatarClassName).toContain('h-14');
    expect(contactHeaderAvatarClassName).toContain('w-14');
    expect(contactHeaderNameClassName).toContain('text-[24px]');
    expect(contactHeaderLifecycleBadgeClassName).toContain('border-border/60');
    expect(contactHeaderLifecycleBadgeClassName).toContain('text-muted-foreground');
  });

  it('builds a subtitle from title and company before lifecycle metadata', () => {
    expect(getContactHeaderSubtitleParts('VP Sales', 'Acme')).toEqual(['VP Sales', 'Acme']);
    expect(getContactHeaderSubtitleParts('', 'Acme')).toEqual(['Acme']);
    expect(getContactHeaderSubtitleParts('VP Sales', '')).toEqual(['VP Sales']);
  });
});
