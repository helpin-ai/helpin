import { describe, expect, it } from 'vitest';

import {
  contactHeaderAvatarClassName,
  contactHeaderLifecycleBadgeClassName,
  contactHeaderNameClassName,
  getContactHeaderSubtitleParts,
} from '../ContactHeader';

describe('ContactHeader', () => {
  it('uses the compact CRM detail header hierarchy', () => {
    expect(contactHeaderAvatarClassName).toContain('h-full');
    expect(contactHeaderAvatarClassName).toContain('w-full');
    expect(contactHeaderNameClassName).toContain('text-[20px]');
    expect(contactHeaderNameClassName).toContain('truncate');
    expect(contactHeaderNameClassName).not.toContain('text-[26px]');
    expect(contactHeaderLifecycleBadgeClassName).not.toContain('rounded');
    expect(contactHeaderLifecycleBadgeClassName).not.toContain('border');
    expect(contactHeaderLifecycleBadgeClassName).toContain('text-quiet-text-tertiary');
  });

  it('builds a subtitle from title and company before lifecycle metadata', () => {
    expect(getContactHeaderSubtitleParts('VP Sales', 'Acme')).toEqual(['VP Sales', 'Acme']);
    expect(getContactHeaderSubtitleParts('', 'Acme')).toEqual(['Acme']);
    expect(getContactHeaderSubtitleParts('VP Sales', '')).toEqual(['VP Sales']);
  });
});
