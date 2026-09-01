import { readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

import {
  contactHeaderAvatarClassName,
  contactHeaderNameClassName,
  getContactHeaderSubtitleParts,
} from '../ContactHeader';

const __dirname = dirname(fileURLToPath(import.meta.url));
const source = readFileSync(resolve(__dirname, '../ContactHeader.tsx'), 'utf8');

describe('ContactHeader', () => {
  it('uses the compact CRM detail header hierarchy', () => {
    expect(contactHeaderAvatarClassName).toContain('h-full');
    expect(contactHeaderAvatarClassName).toContain('w-full');
    expect(contactHeaderNameClassName).toContain('text-[20px]');
    expect(contactHeaderNameClassName).toContain('truncate');
    expect(contactHeaderNameClassName).not.toContain('text-[26px]');
    expect(source).toContain('<QuietStatusBadge tone="lifecycle">');
  });

  it('builds a subtitle from title and company before lifecycle metadata', () => {
    expect(getContactHeaderSubtitleParts('VP Sales', 'Acme')).toEqual(['VP Sales', 'Acme']);
    expect(getContactHeaderSubtitleParts('', 'Acme')).toEqual(['Acme']);
    expect(getContactHeaderSubtitleParts('VP Sales', '')).toEqual(['VP Sales']);
  });
});
