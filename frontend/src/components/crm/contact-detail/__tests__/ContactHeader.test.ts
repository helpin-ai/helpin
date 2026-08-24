import { describe, expect, it } from 'vitest';

import {
  contactHeaderAvatarClassName,
  contactHeaderLifecycleBadgeClassName,
  contactHeaderNameClassName,
  getContactHeaderSubtitleParts,
} from '../ContactHeader';

describe('ContactHeader', () => {
  it('uses the compact CRM detail header hierarchy', () => {
    expect(contactHeaderAvatarClassName).toContain('h-10');
    expect(contactHeaderAvatarClassName).toContain('w-10');
    expect(contactHeaderNameClassName).toContain('text-2xl');
    expect(contactHeaderLifecycleBadgeClassName).toContain('border-border/60');
    expect(contactHeaderLifecycleBadgeClassName).toContain('text-muted-foreground');
  });

  it('builds a subtitle from title and company before lifecycle metadata', () => {
    expect(getContactHeaderSubtitleParts('VP Sales', 'Acme')).toEqual(['VP Sales', 'Acme']);
    expect(getContactHeaderSubtitleParts('', 'Acme')).toEqual(['Acme']);
    expect(getContactHeaderSubtitleParts('VP Sales', '')).toEqual(['VP Sales']);
  });
});
