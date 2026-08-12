import { describe, expect, it } from 'vitest';
import { supportSidebarWidthClass } from '../supportSidebarLayout';

describe('supportSidebarWidthClass', () => {
  it('preserves collapsed details width and widens agent chat', () => {
    expect(supportSidebarWidthClass('details', true)).toBe('w-10 basis-10');
    expect(supportSidebarWidthClass('details', false)).toBe('w-[300px] basis-[300px]');
    expect(supportSidebarWidthClass('agents', true)).toBe('w-[420px] basis-[420px]');
    expect(supportSidebarWidthClass('agents', false)).toBe('w-[420px] basis-[420px]');
  });
});
