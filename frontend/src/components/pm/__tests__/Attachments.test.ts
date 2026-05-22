import { describe, expect, it } from 'vitest';
import { getAttachmentGridDensityClasses } from '@/components/pm/Attachments';

describe('getAttachmentGridDensityClasses', () => {
  it('keeps the comfortable grid for small attachment sets', () => {
    expect(getAttachmentGridDensityClasses(4)).toBe('grid grid-cols-[repeat(auto-fill,minmax(10rem,1fr))] gap-2');
  });

  it('adds columns without shrinking thumbnails for medium attachment sets', () => {
    expect(getAttachmentGridDensityClasses(8)).toBe('grid grid-cols-[repeat(auto-fill,minmax(9rem,1fr))] gap-2');
  });

  it('uses the densest unbounded grid for large attachment sets', () => {
    expect(getAttachmentGridDensityClasses(16)).toBe('grid grid-cols-[repeat(auto-fill,minmax(8rem,1fr))] gap-1.5');
  });
});
